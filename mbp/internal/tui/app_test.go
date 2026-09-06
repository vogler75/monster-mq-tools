package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
)

func TestAppModelSwitchingAndLayout(t *testing.T) {
	m, err := NewAppModel("/home/vogler/Workspace/monster")
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}

	heights := []int{24, 30, 40}
	widths := []int{80, 100, 120}

	for _, h := range heights {
		for _, w := range widths {
			m.Update(tea.WindowSizeMsg{Width: w, Height: h})

			for idx := 0; idx < len(m.Components); idx++ {
				m.SelectedIdx = idx
				m.updateLogViewerForSelection()
				view := m.View()
				lines := strings.Split(view, "\n")
				if len(lines) > h {
					t.Errorf("At %dx%d, idx %d exceeded height %d! Got %d lines", w, h, idx, h, len(lines))
				}
			}
		}
	}
}

func TestAppModelPerComponentLogPreservation(t *testing.T) {
	m, err := NewAppModel("/home/vogler/Workspace/monster")
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// Simulate task for component 0 (main)
	taskMain := runner.NewTask("main", "build", "build-all", "./build.sh", []string{"--all", "-c"}, "/tmp")
	taskMain.Status = runner.TaskSuccess
	for i := 0; i < 50; i++ {
		taskMain.Lines = append(taskMain.Lines, runner.LogLine{
			Timestamp: time.Now(),
			Text:      "Main build output line",
		})
	}
	m.ComponentTasks["main"] = taskMain

	// Simulate task for component 1 (edge)
	taskEdge := runner.NewTask("edge", "pull", "git-pull", "git", []string{"pull"}, "/tmp")
	taskEdge.Status = runner.TaskSuccess
	for i := 0; i < 20; i++ {
		taskEdge.Lines = append(taskEdge.Lines, runner.LogLine{
			Timestamp: time.Now(),
			Text:      "Edge git pull output line",
		})
	}
	m.ComponentTasks["edge"] = taskEdge

	// Select main
	m.SelectedIdx = 0
	m.updateLogViewerForSelection()
	if m.LogViewer.Task != taskMain {
		t.Errorf("Expected main task attached, got %v", m.LogViewer.Task)
	}

	// Switch focus to logs and scroll up
	m.Focus = FocusLogs
	m.LogViewer.Viewport.SetYOffset(5)
	m.LogViewer.AutoScroll = false
	m.saveCurrentLogState()

	// Switch to edge
	m.SelectedIdx = 1
	m.Focus = FocusMenu
	m.updateLogViewerForSelection()
	if m.LogViewer.Task != taskEdge {
		t.Errorf("Expected edge task attached, got %v", m.LogViewer.Task)
	}

	// Switch back to main - scroll position must be preserved!
	m.SelectedIdx = 0
	m.updateLogViewerForSelection()
	if m.LogViewer.Viewport.YOffset != 5 {
		t.Errorf("Expected preserved YOffset 5 for main, got %d", m.LogViewer.Viewport.YOffset)
	}
	if m.LogViewer.AutoScroll != false {
		t.Errorf("Expected preserved AutoScroll false for main, got true")
	}
}

func TestAppModelFocusToggle(t *testing.T) {
	m, err := NewAppModel("/home/vogler/Workspace/monster")
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	if m.Focus != FocusMenu {
		t.Errorf("Initial focus should be FocusMenu, got %d", m.Focus)
	}

	// Press Tab -> switch to FocusLogs
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus != FocusLogs {
		t.Errorf("Expected FocusLogs after Tab, got %d", m.Focus)
	}

	// Press Tab -> switch back to FocusMenu
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus != FocusMenu {
		t.Errorf("Expected FocusMenu after Tab, got %d", m.Focus)
	}

	// Press Tab -> FocusLogs, then Esc -> FocusMenu
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus != FocusLogs {
		t.Errorf("Expected FocusLogs after Tab, got %d", m.Focus)
	}
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Focus != FocusMenu {
		t.Errorf("Expected FocusMenu after Esc, got %d", m.Focus)
	}
}

func TestAppModelFullscreenToggle(t *testing.T) {
	m, err := NewAppModel("/home/vogler/Workspace/monster")
	if err != nil {
		t.Fatalf("Failed to create model: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// Toggle fullscreen with 'f'
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if !m.FullscreenLog {
		t.Errorf("Expected FullscreenLog to be true")
	}
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > 24 {
		t.Errorf("Fullscreen view exceeded height 24! Got %d lines", len(lines))
	}

	// Exit fullscreen with 'esc'
	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if m.FullscreenLog {
		t.Errorf("Expected FullscreenLog to be false after Esc")
	}
	view = m.View()
	lines = strings.Split(view, "\n")
	if len(lines) > 24 {
		t.Errorf("Normal view after exiting fullscreen exceeded height 24! Got %d lines", len(lines))
	}
}

func TestStyleLogLine(t *testing.T) {
	tests := []struct {
		input       string
		isStderr    bool
		expectError bool
		expectWarn  bool
		expectSucc  bool
		expectStep  bool
	}{
		{
			input:       "#9 exporting layers done",
			isStderr:    true,
			expectStep:  true,
			expectError: false,
		},
		{
			input:       "#9 exporting manifest sha256:84bc55a9ca2a1e37f261f6ad8c7249b84659d47f307f90a8c18d078a2669d2e5 done",
			isStderr:    true,
			expectStep:  true,
			expectError: false,
		},
		{
			input:       "#9 sending tarball",
			isStderr:    true,
			expectStep:  true,
			expectError: false,
		},
		{
			input:       "✓ Compat Docker image built successfully",
			isStderr:    false,
			expectSucc:  true,
			expectError: false,
		},
		{
			input:       "fatal: not a git repository (or any of the parent directories): .git",
			isStderr:    true,
			expectError: true,
		},
		{
			input:       "error: failed to push some refs to 'origin'",
			isStderr:    true,
			expectError: true,
		},
		{
			input:       "npm ERR! code ELIFECYCLE",
			isStderr:    true,
			expectError: true,
		},
		{
			input:       "warning: LF will be replaced by CRLF",
			isStderr:    true,
			expectWarn:  true,
			expectError: false,
		},
		{
			input:       "Build completed with 0 errors and 0 warnings",
			isStderr:    false,
			expectError: false,
			expectWarn:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			lower := strings.ToLower(tc.input)
			trimmed := strings.TrimSpace(tc.input)

			isErr := isRealError(lower, trimmed)
			if isErr != tc.expectError {
				t.Errorf("isRealError(%q) = %v, expected %v", tc.input, isErr, tc.expectError)
			}

			isWrn := isWarning(lower)
			if isWrn != tc.expectWarn {
				t.Errorf("isWarning(%q) = %v, expected %v", tc.input, isWrn, tc.expectWarn)
			}

			styled := styleLogLine(tc.input, tc.isStderr)
			// Ensure it renders text
			if !strings.Contains(styled, trimmed) && !strings.Contains(styled, tc.input) {
				t.Errorf("styleLogLine lost input text: %q", styled)
			}
		})
	}
}
