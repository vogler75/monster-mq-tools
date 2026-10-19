package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTaskExecution(t *testing.T) {
	cmd := "echo"
	args := []string{"hello", "monster"}
	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/c", "echo hello monster"}
	}
	task := NewTask("test", "build", "echo-test", cmd, args, ".")
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
	cmd := "sleep"
	args := []string{"5"}
	if runtime.GOOS == "windows" {
		cmd = "ping"
		args = []string{"127.0.0.1", "-n", "10"}
	}
	task := NewTask("test", "build", "sleep-test", cmd, args, ".")
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

func TestPowerShellScriptExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific PowerShell test")
	}

	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "test-script.ps1")
	scriptContent := "param([string]$name)\nWrite-Output \"Hello $name from PowerShell\"\nexit 0\n"
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	task := NewTask("test", "build", "ps-test", "./test-script.ps1", []string{"Monster"}, tmpDir)
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
	found := false
	for _, l := range lines {
		if strings.Contains(l.Text, "Hello Monster from PowerShell") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected line containing 'Hello Monster from PowerShell', got lines: %+v", lines)
	}
}
