package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
)

// LogViewer wraps viewport.Model with autoscroll and formatting.
type LogViewer struct {
	Viewport   viewport.Model
	AutoScroll bool
	Task       *runner.Task
	Width      int
	Height     int
}

// NewLogViewer initializes a LogViewer.
func NewLogViewer(width, height int) LogViewer {
	vp := viewport.New(width, height)
	return LogViewer{
		Viewport:   vp,
		AutoScroll: true,
		Width:      width,
		Height:     height,
	}
}

// SetSize updates the dimensions of the viewport.
func (lv *LogViewer) SetSize(width, height int) {
	lv.Width = width
	lv.Height = height
	lv.Viewport.Width = width
	lv.Viewport.Height = height
	lv.UpdateContent()
}

// SetTask attaches a new task and clears or resets the view.
func (lv *LogViewer) SetTask(t *runner.Task) {
	lv.Task = t
	lv.AutoScroll = true
	lv.UpdateContent()
}

// AppendLine adds a line and scrolls if autoscroll is on.
func (lv *LogViewer) UpdateContent() {
	if lv.Task == nil {
		lv.Viewport.SetContent(StyleDim.Render("  (No active or completed task. Select a component and press [b] to build or [p] to publish.)"))
		return
	}

	lines := lv.Task.GetLines()
	if len(lines) == 0 {
		lv.Viewport.SetContent(StyleDim.Render(fmt.Sprintf("  Waiting for output from: %s %s...", lv.Task.Command, strings.Join(lv.Task.Args, " "))))
		return
	}

	var sb strings.Builder
	for _, l := range lines {
		timePrefix := StyleDim.Render(l.Timestamp.Format("15:04:05") + " ")
		text := l.Text
		if l.IsError {
			text = lipgloss.NewStyle().Foreground(ColorDanger).Render(text)
		}
		sb.WriteString(timePrefix + text + "\n")
	}

	lv.Viewport.SetContent(sb.String())
	if lv.AutoScroll {
		lv.Viewport.GotoBottom()
	}
}

// RenderHeader renders the title bar for the log view.
func (lv *LogViewer) RenderHeader(width int) string {
	title := "LIVE LOG OUTPUT"
	statusStr := "IDLE"
	timerStr := ""

	if lv.Task != nil {
		switch lv.Task.Status {
		case runner.TaskRunning:
			statusStr = BadgeBuilding.Render("● RUNNING")
		case runner.TaskSuccess:
			statusStr = BadgeBuilt.Render("✔ SUCCESS")
		case runner.TaskFailed:
			statusStr = BadgeNotBuilt.Render(fmt.Sprintf("✘ FAILED (code %d)", lv.Task.ExitCode))
		case runner.TaskCancelled:
			statusStr = BadgeOutdated.Render("⊘ CANCELLED")
		}

		if !lv.Task.StartTime.IsZero() {
			dur := lv.Task.Duration
			if lv.Task.Status == runner.TaskRunning {
				dur = time.Since(lv.Task.StartTime)
			}
			timerStr = fmt.Sprintf(" [%02d:%02d]", int(dur.Minutes()), int(dur.Seconds())%60)
		}

		title = fmt.Sprintf("%s: %s %s", strings.ToUpper(lv.Task.Action), lv.Task.ComponentID, lv.Task.TargetID)
	}

	autoScrollStatus := "[Autoscroll: ON]"
	if !lv.AutoScroll {
		autoScrollStatus = StyleDim.Render("[Autoscroll: OFF - press 'a']")
	}

	left := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(title) + timerStr + "  " + statusStr
	right := autoScrollStatus

	space := max(width-lipgloss.Width(left)-lipgloss.Width(right)-4, 1)
	return left + strings.Repeat(" ", space) + right
}
