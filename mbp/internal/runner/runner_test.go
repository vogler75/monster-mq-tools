package runner

import (
	"context"
	"testing"
	"time"
)

func TestTaskExecution(t *testing.T) {
	task := NewTask("test", "build", "echo-test", "echo", []string{"hello", "monster"}, ".")
	ctx := context.Background()

	err := task.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	<-task.DoneChan()

	if task.Status != TaskSuccess {
		t.Errorf("expected status %s, got %s (error: %s)", TaskSuccess, task.Status, task.Error)
	}

	lines := task.GetLines()
	if len(lines) == 0 {
		t.Errorf("expected at least 1 log line, got 0")
	} else if lines[0].Text != "hello monster" {
		t.Errorf("unexpected output: %s", lines[0].Text)
	}
}

func TestTaskCancellation(t *testing.T) {
	task := NewTask("test", "build", "sleep-test", "sleep", []string{"5"}, ".")
	ctx := context.Background()

	err := task.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start task: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	task.Cancel()

	<-task.DoneChan()

	if task.Status != TaskCancelled && task.Status != TaskFailed {
		t.Errorf("expected task to be cancelled or failed, got %s", task.Status)
	}
}
