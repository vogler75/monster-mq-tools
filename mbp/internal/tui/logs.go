package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
)

// LogViewer wraps viewport.Model with autoscroll, per-component details, and formatting.
type LogViewer struct {
	Viewport         viewport.Model
	AutoScroll       bool
	Task             *runner.Task
	Component        *component.Component
	ComponentName    string
	EmptyMsg         string
	Width            int
	Height           int
	lastLineCount    int
	lastTaskStatus   runner.TaskStatus
	lastComponentID  string
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
	if lv.Width == width && lv.Height == height {
		return
	}
	lv.Width = width
	lv.Height = height
	lv.Viewport.Width = width
	lv.Viewport.Height = height
	lv.UpdateContent()
}

// SetComponent attaches a selected component and its associated task.
func (lv *LogViewer) SetComponent(c *component.Component, t *runner.Task, isFocused bool) {
	compChanged := lv.Component != c
	lv.Component = c
	lv.Task = t
	if c != nil {
		lv.ComponentName = c.Name
	} else {
		lv.ComponentName = ""
	}
	if compChanged {
		lv.lastLineCount = -1
		lv.lastTaskStatus = ""
		if c != nil {
			lv.lastComponentID = c.ID
		} else {
			lv.lastComponentID = ""
		}
	}
	lv.UpdateContent()
}

// SetTask attaches a new task and updates the view.
func (lv *LogViewer) SetTask(t *runner.Task, componentName string) {
	lv.Task = t
	lv.ComponentName = componentName
	lv.lastLineCount = -1
	lv.lastTaskStatus = ""
	lv.UpdateContent()
}

// SetEmptyMessage sets custom fallback text when no task or component is active.
func (lv *LogViewer) SetEmptyMessage(msg string) {
	lv.EmptyMsg = msg
	if lv.Task == nil && lv.Component == nil {
		lv.UpdateContent()
	}
}

// UpdateContent re-renders log content or component summary in the viewport.
func (lv *LogViewer) UpdateContent() {
	if lv.Task == nil {
		if lv.Component != nil {
			summary := lv.renderComponentSummary(lv.Component)
			lv.Viewport.SetContent(summary)
			return
		}
		msg := lv.EmptyMsg
		if msg == "" {
			msg = "  (No active or completed task. Select a component and press [b] to build or [u] to pull.)"
		}
		lv.Viewport.SetContent(StyleDim.Render(msg))
		return
	}

	lines := lv.Task.GetLines()
	if len(lines) == lv.lastLineCount && lv.Task.Status == lv.lastTaskStatus {
		return
	}
	lv.lastLineCount = len(lines)
	lv.lastTaskStatus = lv.Task.Status

	if len(lines) == 0 {
		lv.Viewport.SetContent(StyleDim.Render(fmt.Sprintf("  Waiting for output from: %s %s...", lv.Task.Command, strings.Join(lv.Task.Args, " "))))
		return
	}

	var sb strings.Builder
	for _, l := range lines {
		timePrefix := StyleDim.Render(l.Timestamp.Format("15:04:05") + " ")
		text := styleLogLine(l.Text, l.IsError)
		sb.WriteString(timePrefix + text + "\n")
	}

	lv.Viewport.SetContent(sb.String())
	if lv.AutoScroll {
		lv.Viewport.GotoBottom()
	}
}

// styleLogLine applies contextual colors to log output instead of naively coloring all stderr red.
func styleLogLine(text string, isStderr bool) string {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)

	// 1. Success lines (green)
	if strings.HasPrefix(trimmed, "✓") || strings.HasPrefix(trimmed, "✔") ||
		strings.Contains(lower, "build success") || strings.Contains(lower, "successfully built") ||
		strings.Contains(lower, "built successfully") {
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render(text)
	}

	// 2. Real error keywords (bold red)
	if isRealError(lower, trimmed) {
		return lipgloss.NewStyle().Foreground(ColorDanger).Bold(true).Render(text)
	}

	// 3. Warnings (amber/yellow)
	if isWarning(lower) {
		return lipgloss.NewStyle().Foreground(ColorWarning).Render(text)
	}

	// 4. Docker BuildKit steps (#1, #9, etc. - soft cyan/blue)
	if strings.HasPrefix(trimmed, "#") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Render(text)
	}

	// 5. Standard line (crisp readable white)
	return lipgloss.NewStyle().Foreground(ColorWhite).Render(text)
}

func isRealError(lower, trimmed string) bool {
	// Guard against false positives like "0 errors", "no error"
	if strings.Contains(lower, "0 error") || strings.Contains(lower, "no error") || strings.Contains(lower, "errors: 0") {
		return false
	}

	return strings.Contains(lower, "error:") ||
		strings.Contains(lower, "error [") ||
		strings.Contains(lower, "[error]") ||
		strings.HasPrefix(lower, "error ") ||
		strings.Contains(lower, "fatal:") ||
		strings.HasPrefix(lower, "fatal ") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "exception:") ||
		strings.Contains(lower, "npm err!") ||
		strings.HasPrefix(lower, "fail:") ||
		strings.Contains(lower, "exit status 1") ||
		strings.Contains(lower, "exit code 1")
}

func isWarning(lower string) bool {
	if strings.Contains(lower, "0 warning") || strings.Contains(lower, "warnings: 0") {
		return false
	}
	return strings.Contains(lower, "warning:") ||
		strings.Contains(lower, "warn:") ||
		strings.HasPrefix(lower, "warn ") ||
		strings.Contains(lower, "[warn]") ||
		strings.Contains(lower, "npm warn")
}

// renderComponentSummary renders detailed component status when idle.
func (lv *LogViewer) renderComponentSummary(c *component.Component) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorWhite).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(ColorWhite)
	dimStyle := StyleDim

	sb.WriteString(titleStyle.Render(fmt.Sprintf("  %s (%s)", c.Name, c.ID)))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  " + strings.Repeat("─", max(lv.Width-4, 50))))
	sb.WriteString("\n")

	// 1. Directory and Version
	sb.WriteString(fmt.Sprintf("  %s %s   %s %s\n",
		labelStyle.Render("Directory:"),
		dimStyle.Render(c.Directory),
		labelStyle.Render("Version:"),
		valStyle.Render(c.Version),
	))

	// 2. Git Information
	gitInfo := "Not a git repository"
	if c.Git.IsRepo {
		gitParts := []string{fmt.Sprintf("branch '%s'", c.Git.Branch)}
		if c.Git.Behind > 0 {
			gitParts = append(gitParts, BadgeBehind.Render(fmt.Sprintf("▼%d behind upstream (updates available!)", c.Git.Behind)))
		} else if c.Git.Ahead > 0 {
			gitParts = append(gitParts, BadgeAhead.Render(fmt.Sprintf("▲%d ahead", c.Git.Ahead)))
		}
		if c.Git.IsDirty {
			gitParts = append(gitParts, BadgeDirty.Render(fmt.Sprintf("*%d dirty uncommitted files", c.Git.DirtyFilesCount)))
		} else if c.Git.Behind == 0 && c.Git.Ahead == 0 {
			gitParts = append(gitParts, BadgeClean.Render("✔ clean and up to date"))
		}
		gitInfo = strings.Join(gitParts, ", ")
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Git Status:"), gitInfo))

	// 3. Artifact Details
	artInfo := dimStyle.Render("No compiled artifacts found on disk")
	if c.LatestArtifact != nil {
		artInfo = fmt.Sprintf("%s (%s, modified %s)",
			valStyle.Render(c.LatestArtifact.Name),
			formatFileSize(c.LatestArtifact.Size),
			formatRelativeTime(c.LatestArtifact.ModTime),
		)
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Artifact:  "), artInfo))

	// 4. Build Targets Summary
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("  Available Actions & Targets:"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("    %s %s (%s %s)\n",
		StyleKey.Render("[b] Build:"),
		c.DefaultBuild.Name,
		c.DefaultBuild.Command,
		strings.Join(c.DefaultBuild.Args, " "),
	))
	sb.WriteString(fmt.Sprintf("    %s %s\n",
		StyleKey.Render("[u] Git Pull:"),
		"Pull latest commits for this component",
	))
	if len(c.PublishTargets) > 0 {
		sb.WriteString(fmt.Sprintf("    %s %s\n",
			StyleKey.Render("[p] Publish:"),
			c.DefaultPublish.Name,
		))
	}
	if c.CleanTarget != nil {
		sb.WriteString(fmt.Sprintf("    %s %s\n",
			StyleKey.Render("[c] Clean:"),
			c.CleanTarget.Name,
		))
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  (Press [b] to start build or [u] to pull. Real-time streaming logs will display here.)"))

	return sb.String()
}

// RenderHeader renders the title bar for the log view.
func (lv *LogViewer) RenderHeader(width int, isFocused bool) string {
	title := "LIVE LOG OUTPUT"
	if lv.ComponentName != "" {
		title = fmt.Sprintf("LOG OUTPUT: %s", lv.ComponentName)
	}
	statusStr := BadgeClean.Render("READY")
	timerStr := ""

	if lv.Task != nil {
		switch lv.Task.Status {
		case runner.TaskRunning:
			if lv.Task.Action == "pull" {
				statusStr = BadgePulling.Render("● PULLING")
			} else {
				statusStr = BadgeBuilding.Render("● RUNNING")
			}
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

		name := lv.ComponentName
		if name == "" {
			name = lv.Task.ComponentID
		}
		title = fmt.Sprintf("%s: %s (%s)", strings.ToUpper(lv.Task.Action), name, lv.Task.TargetID)
	}

	autoScrollStatus := "[Autoscroll: ON]"
	if !lv.AutoScroll {
		autoScrollStatus = StyleDim.Render("[Autoscroll: OFF]")
	}

	focusHint := StyleDim.Render("[Tab: Log]")
	if isFocused {
		focusHint = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("[FOCUSED]")
	}

	if width < 85 {
		autoScrollStatus = "[Auto: ON]"
		if !lv.AutoScroll {
			autoScrollStatus = StyleDim.Render("[Auto: OFF]")
		}
	}
	if width < 75 && !isFocused {
		focusHint = ""
	}

	left := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(title) + timerStr + "  " + statusStr
	var rightParts []string
	if focusHint != "" {
		rightParts = append(rightParts, focusHint)
	}
	rightParts = append(rightParts, autoScrollStatus)
	right := strings.Join(rightParts, " ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	if leftW+rightW+1 > width {
		// Condense title for narrow displays
		shortName := lv.ComponentName
		if lv.Task != nil {
			shortName = lv.Task.ComponentID
		}
		title = fmt.Sprintf("LOG: %s", shortName)
		left = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(title) + timerStr + " " + statusStr
		leftW = lipgloss.Width(left)
	}

	if leftW+rightW+1 > width {
		// Truncate left if still too wide
		availLeft := max(width-rightW-1, 10)
		left = truncateVisible(left, availLeft)
		leftW = lipgloss.Width(left)
	}

	space := max(width-leftW-rightW, 1)
	line := left + strings.Repeat(" ", space) + right
	if lipgloss.Width(line) > width {
		line = truncateVisible(line, width)
	}
	return line
}

