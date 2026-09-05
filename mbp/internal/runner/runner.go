package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// LogLine represents an output line from a running task.
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Text      string    `json:"text"`
	IsError   bool      `json:"is_error"`
}

// TaskStatus represents the lifecycle state of an execution.
type TaskStatus string

const (
	TaskIdle      TaskStatus = "idle"
	TaskRunning   TaskStatus = "running"
	TaskSuccess   TaskStatus = "success"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Task represents a running or finished build/publish process.
type Task struct {
	ID          string        `json:"id"`
	ComponentID string        `json:"component_id"`
	Action      string        `json:"action"` // "build" or "publish"
	TargetID    string        `json:"target_id"`
	Command     string        `json:"command"`
	Args        []string      `json:"args"`
	Dir         string        `json:"dir"`

	Status      TaskStatus    `json:"status"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	ExitCode    int           `json:"exit_code"`
	Error       string        `json:"error,omitempty"`

	Lines       []LogLine     `json:"lines"`
	maxLines    int

	cancel      context.CancelFunc
	cmd         *exec.Cmd
	mu          sync.RWMutex
	lineChan    chan LogLine
	doneChan    chan struct{}
}

// NewTask creates a new executable task.
func NewTask(componentID, action, targetID, command string, args []string, dir string) *Task {
	return &Task{
		ID:          fmt.Sprintf("%s-%s-%d", componentID, action, time.Now().UnixNano()),
		ComponentID: componentID,
		Action:      action,
		TargetID:    targetID,
		Command:     command,
		Args:        args,
		Dir:         dir,
		Status:      TaskIdle,
		Lines:       make([]LogLine, 0, 1024),
		maxLines:    5000,
		lineChan:    make(chan LogLine, 256),
		doneChan:    make(chan struct{}),
	}
}

// LineChan returns the channel yielding output lines as they arrive.
func (t *Task) LineChan() <-chan LogLine {
	return t.lineChan
}

// DoneChan returns a channel closed when the task completes.
func (t *Task) DoneChan() <-chan struct{} {
	return t.doneChan
}

// GetLines returns a snapshot of all accumulated log lines.
func (t *Task) GetLines() []LogLine {
	t.mu.RLock()
	defer t.mu.RUnlock()
	copied := make([]LogLine, len(t.Lines))
	copy(copied, t.Lines)
	return copied
}

// Start launches the command asynchronously.
func (t *Task) Start(parentCtx context.Context) error {
	t.mu.Lock()
	if t.Status == TaskRunning {
		t.mu.Unlock()
		return fmt.Errorf("task %s is already running", t.ID)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	t.cancel = cancel
	t.Status = TaskRunning
	t.StartTime = time.Now()

	cmd := exec.CommandContext(ctx, t.Command, t.Args...)
	cmd.Dir = t.Dir
	// Set process group so killing terminates child processes (e.g. npm, docker, maven)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.cmd = cmd
	t.mu.Unlock()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.finish(TaskFailed, 1, fmt.Errorf("stdout pipe: %w", err))
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.finish(TaskFailed, 1, fmt.Errorf("stderr pipe: %w", err))
		return err
	}

	if err := cmd.Start(); err != nil {
		t.finish(TaskFailed, 1, fmt.Errorf("cmd start: %w", err))
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go t.streamReader(stdout, false, &wg)
	go t.streamReader(stderr, true, &wg)

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()

		t.mu.Lock()
		defer t.mu.Unlock()

		t.EndTime = time.Now()
		t.Duration = t.EndTime.Sub(t.StartTime)

		if ctx.Err() == context.Canceled {
			t.Status = TaskCancelled
			t.Error = "process cancelled by user"
			close(t.lineChan)
			close(t.doneChan)
			return
		}

		if waitErr != nil {
			t.Status = TaskFailed
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				t.ExitCode = exitErr.ExitCode()
			} else {
				t.ExitCode = 1
			}
			t.Error = waitErr.Error()
		} else {
			t.Status = TaskSuccess
			t.ExitCode = 0
		}

		close(t.lineChan)
		close(t.doneChan)
	}()

	return nil
}

func (t *Task) streamReader(r io.Reader, isErr bool, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		logLine := LogLine{
			Timestamp: time.Now(),
			Text:      line,
			IsError:   isErr,
		}

		t.mu.Lock()
		if len(t.Lines) >= t.maxLines {
			t.Lines = t.Lines[1:]
		}
		t.Lines = append(t.Lines, logLine)
		t.mu.Unlock()

		select {
		case t.lineChan <- logLine:
		default:
			// Non-blocking drop if channel full
		}
	}
}

// Cancel terminates the running command and all processes in its process group.
func (t *Task) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd != nil && t.cmd.Process != nil && t.Status == TaskRunning {
		// Kill entire process group
		_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	}
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *Task) finish(status TaskStatus, exitCode int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
	t.ExitCode = exitCode
	t.EndTime = time.Now()
	t.Duration = t.EndTime.Sub(t.StartTime)
	if err != nil {
		t.Error = err.Error()
	}
	close(t.lineChan)
	close(t.doneChan)
}
