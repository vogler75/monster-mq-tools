package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
	"github.com/vogler75/monster-mq-tools/mbp/internal/git"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
)

type queuedTask struct {
	ComponentID string
	Target      component.Target
	Action      string
}

// AppModel is the primary Bubble Tea model for MBP.
type AppModel struct {
	RootDir       string
	Components    []*component.Component
	SelectedIdx   int
	LogViewer     LogViewer
	Dialog        DialogModel
	ActiveTask    *runner.Task
	Queue         []queuedTask
	Spinner       spinner.Model

	Width         int
	Height        int
	FullscreenLog bool
	StatusMsg     string
	FetchingGit   bool
}

// Custom Bubble Tea Messages
type logLineMsg struct {
	taskID string
	line   runner.LogLine
}

type taskDoneMsg struct {
	task *runner.Task
}

type gitFetchDoneMsg struct {
	err error
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// NewAppModel initializes the Bubble Tea application state.
func NewAppModel(rootDir string) (*AppModel, error) {
	comps, err := component.LoadAndRefreshAll(rootDir)
	if err != nil {
		return nil, err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	lv := NewLogViewer(80, 15)

	return &AppModel{
		RootDir:     rootDir,
		Components:  comps,
		SelectedIdx: 0,
		LogViewer:   lv,
		Spinner:     s,
		Queue:       make([]queuedTask, 0),
	}, nil
}

func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		tickCmd(),
	)
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.resizeLayout()

	case tickMsg:
		cmds = append(cmds, tickCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)

	case logLineMsg:
		if m.ActiveTask != nil && m.ActiveTask.ID == msg.taskID {
			m.LogViewer.UpdateContent()
		}

	case taskDoneMsg:
		m.handleTaskDone(msg.task)
		// Process next in queue if any
		if len(m.Queue) > 0 {
			next := m.Queue[0]
			m.Queue = m.Queue[1:]
			cmds = append(cmds, m.startTaskCmd(next.ComponentID, next.Action, next.Target))
		}

	case gitFetchDoneMsg:
		m.FetchingGit = false
		if msg.err != nil {
			m.StatusMsg = fmt.Sprintf("Git fetch failed: %v", msg.err)
		} else {
			m.StatusMsg = "Git fetch complete. Statuses updated."
			for _, c := range m.Components {
				component.RefreshStatus(c)
			}
		}

	case tea.KeyMsg:
		cmd := m.handleKeyPress(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Propagate viewport updates if log viewer active
	var vpCmd tea.Cmd
	m.LogViewer.Viewport, vpCmd = m.LogViewer.Viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// If modal is active
	if m.Dialog.Type != DialogNone {
		switch key {
		case "esc":
			m.Dialog.Type = DialogNone
			return nil
		case "up", "k":
			if m.Dialog.SelectedIdx > 0 {
				m.Dialog.SelectedIdx--
			}
			return nil
		case "down", "j":
			if m.Dialog.SelectedIdx < len(m.Dialog.Targets)-1 {
				m.Dialog.SelectedIdx++
			}
			return nil
		case "enter":
			return m.executeDialogAction()
		}
		return nil
	}

	// Normal View Keybindings
	switch key {
	case "q":
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.StatusMsg = "Task running! Press 'k' to cancel task first, or Ctrl+C to force exit."
			return nil
		}
		return tea.Quit

	case "ctrl+c":
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.ActiveTask.Cancel()
			m.StatusMsg = "Running task cancelled."
			return nil
		}
		return tea.Quit

	case "up", "k":
		if m.SelectedIdx > 0 {
			m.SelectedIdx--
		}

	case "down", "j":
		if m.SelectedIdx < len(m.Components)-1 {
			m.SelectedIdx++
		}

	case "b":
		// Build selected
		sel := m.selectedComponent()
		if sel == nil {
			return nil
		}
		m.Dialog = DialogModel{
			Type:        DialogBuildTarget,
			Component:   sel,
			Targets:     sel.BuildTargets,
			SelectedIdx: 0,
		}

	case "B":
		// Build ALL components sequentially
		return m.triggerBuildAll()

	case "p":
		// Publish selected
		sel := m.selectedComponent()
		if sel == nil {
			return nil
		}
		m.Dialog = DialogModel{
			Type:        DialogPublishTarget,
			Component:   sel,
			Targets:     sel.PublishTargets,
			SelectedIdx: 0,
		}

	case "c":
		// Clean selected
		sel := m.selectedComponent()
		if sel == nil || sel.CleanTarget == nil {
			m.StatusMsg = "Selected component has no clean target configured."
			return nil
		}
		m.Dialog = DialogModel{
			Type:        DialogCleanConfirm,
			Component:   sel,
			Targets:     []component.Target{*sel.CleanTarget},
			SelectedIdx: 0,
		}

	case "g":
		// Fetch git origin
		if m.FetchingGit {
			return nil
		}
		m.FetchingGit = true
		m.StatusMsg = "Fetching git remotes in background..."
		return m.gitFetchCmd()

	case "r":
		// Refresh status
		for _, c := range m.Components {
			component.RefreshStatus(c)
		}
		m.StatusMsg = "Component statuses refreshed."

	case "f", "l":
		// Toggle fullscreen log view
		m.FullscreenLog = !m.FullscreenLog
		m.resizeLayout()

	case "a":
		// Toggle log autoscroll
		m.LogViewer.AutoScroll = !m.LogViewer.AutoScroll

	case "x", "X":
		// Cancel active task
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.ActiveTask.Cancel()
			m.StatusMsg = "Cancelling task..."
		}

	case "?":
		m.Dialog = DialogModel{
			Type: DialogHelp,
		}
	}

	return nil
}

func (m *AppModel) selectedComponent() *component.Component {
	if m.SelectedIdx >= 0 && m.SelectedIdx < len(m.Components) {
		return m.Components[m.SelectedIdx]
	}
	return nil
}

func (m *AppModel) executeDialogAction() tea.Cmd {
	d := m.Dialog
	m.Dialog.Type = DialogNone

	if d.Component == nil || len(d.Targets) == 0 {
		return nil
	}

	target := d.Targets[d.SelectedIdx]

	switch d.Type {
	case DialogBuildTarget:
		return m.startTaskCmd(d.Component.ID, "build", target)
	case DialogPublishTarget:
		return m.startTaskCmd(d.Component.ID, "publish", target)
	case DialogCleanConfirm:
		return m.startTaskCmd(d.Component.ID, "clean", target)
	}
	return nil
}

func (m *AppModel) triggerBuildAll() tea.Cmd {
	if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
		m.StatusMsg = "Cannot start Build All: another task is already running!"
		return nil
	}

	m.Queue = nil
	for _, c := range m.Components {
		if c.ID == "tools" {
			continue // Skip tools in main full build unless selected
		}
		m.Queue = append(m.Queue, queuedTask{
			ComponentID: c.ID,
			Target:      c.DefaultBuild,
			Action:      "build",
		})
	}

	if len(m.Queue) == 0 {
		return nil
	}

	first := m.Queue[0]
	m.Queue = m.Queue[1:]
	m.StatusMsg = fmt.Sprintf("Started Build All sequence (%d components queued).", len(m.Queue)+1)
	return m.startTaskCmd(first.ComponentID, first.Action, first.Target)
}

func (m *AppModel) startTaskCmd(componentID, action string, target component.Target) tea.Cmd {
	comp, err := component.FindComponent(m.Components, componentID)
	if err != nil {
		m.StatusMsg = err.Error()
		return nil
	}

	task := runner.NewTask(
		componentID,
		action,
		target.ID,
		target.Command,
		target.Args,
		comp.Directory,
	)

	m.ActiveTask = task
	m.LogViewer.SetTask(task)

	if action == "build" {
		comp.Status = component.StatusBuilding
	} else if action == "publish" {
		comp.Status = component.StatusPublishing
	}

	return func() tea.Msg {
		ctx := context.Background()
		if err := task.Start(ctx); err != nil {
			return taskDoneMsg{task: task}
		}

		// Background forwarder for lines
		go func() {
			for line := range task.LineChan() {
				_ = line
				// Line buffered inside task; UI polls on tick or logLineMsg
			}
		}()

		<-task.DoneChan()
		return taskDoneMsg{task: task}
	}
}

func (m *AppModel) handleTaskDone(t *runner.Task) {
	comp, err := component.FindComponent(m.Components, t.ComponentID)
	if err == nil {
		component.RefreshStatus(comp)
		comp.LastBuildTime = t.EndTime
		comp.LastBuildDuration = t.Duration

		if t.Status == runner.TaskSuccess {
			comp.Status = component.StatusSuccess
			m.StatusMsg = fmt.Sprintf("✔ %s %s finished successfully in %s", strings.ToUpper(t.Action), comp.Name, t.Duration.Round(time.Second))
		} else if t.Status == runner.TaskCancelled {
			comp.Status = component.StatusOutdated
			m.StatusMsg = fmt.Sprintf("⊘ %s cancelled.", comp.Name)
		} else {
			comp.Status = component.StatusFailed
			comp.LastError = t.Error
			m.StatusMsg = fmt.Sprintf("✘ %s failed (exit %d): %s", comp.Name, t.ExitCode, t.Error)
		}
	}
	m.LogViewer.UpdateContent()
}

func (m *AppModel) gitFetchCmd() tea.Cmd {
	comps := m.Components
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var lastErr error
		for _, c := range comps {
			if c.Git.IsRepo {
				if err := git.Fetch(ctx, c.Directory); err != nil {
					lastErr = err
				}
			}
		}
		return gitFetchDoneMsg{err: lastErr}
	}
}

func (m *AppModel) resizeLayout() {
	w := max(m.Width, 80)
	h := max(m.Height, 24)

	if m.FullscreenLog {
		m.LogViewer.SetSize(w-4, h-6)
	} else {
		// Table gets ~8-10 rows, log gets remainder
		logH := max(h-14, 8)
		m.LogViewer.SetSize(w-4, logH)
	}
}

func (m *AppModel) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing MBP..."
	}

	var sb strings.Builder

	// 1. Header Banner
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	if !m.FullscreenLog {
		// 2. Component Status Table
		tableBox := StylePane.Width(m.Width - 4).Render(RenderComponentTable(m.Components, m.SelectedIdx, m.Width-6))
		sb.WriteString(tableBox)
		sb.WriteString("\n")
	}

	// 3. Log Output Pane
	logHeader := m.LogViewer.RenderHeader(m.Width - 6)
	logBody := m.LogViewer.Viewport.View()
	logPane := StylePane.Width(m.Width - 4).Render(logHeader + "\n" + logBody)
	sb.WriteString(logPane)
	sb.WriteString("\n")

	// 4. Status Bar & Action Keybindings
	sb.WriteString(m.renderFooter())

	// 5. Overlay modal dialog if open
	if m.Dialog.Type != DialogNone {
		modal := RenderModal(&m.Dialog, m.Width, m.Height)
		return modal
	}

	return sb.String()
}

func (m *AppModel) renderHeader() string {
	title := StyleTitle.Render(" MBP ") + " " + StyleSubtitle.Render("MonsterMQ Build Pipeline")
	pathStr := StyleDim.Render(fmt.Sprintf("Root: %s", m.RootDir))

	// Summary badges
	builtCount := 0
	outdatedCount := 0
	behindCount := 0
	for _, c := range m.Components {
		if c.Status == component.StatusBuilt || c.Status == component.StatusSuccess {
			builtCount++
		} else if c.Status == component.StatusOutdated {
			outdatedCount++
		}
		if c.Git.Behind > 0 {
			behindCount++
		}
	}

	stats := fmt.Sprintf(
		"Total: %d | %s | %s",
		len(m.Components),
		BadgeBuilt.Render(fmt.Sprintf("Built: %d", builtCount)),
		BadgeOutdated.Render(fmt.Sprintf("Outdated: %d", outdatedCount)),
	)
	if behindCount > 0 {
		stats += " | " + BadgeBehind.Render(fmt.Sprintf("Git Updates: %d", behindCount))
	}

	if m.FetchingGit {
		stats += " | " + m.Spinner.View() + " " + StyleDim.Render("Fetching git...")
	}

	left := title + "  " + pathStr
	right := stats
	space := max(m.Width-lipgloss.Width(left)-lipgloss.Width(right)-4, 1)

	return StyleHeader.Width(m.Width - 4).Render(left + strings.Repeat(" ", space) + right)
}

func (m *AppModel) renderFooter() string {
	var statusLine string
	if m.StatusMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(ColorWarning).Render("ℹ " + m.StatusMsg)
	}

	keys := []string{
		"[↑/↓] Select",
		"[b] Build",
		"[B] Build All",
		"[p] Publish",
		"[c] Clean",
		"[g] Git Fetch",
		"[f] Full Log",
		"[x] Kill",
		"[?] Help",
		"[q] Quit",
	}

	var formattedKeys []string
	for _, k := range keys {
		parts := strings.SplitN(k, " ", 2)
		if len(parts) == 2 {
			formattedKeys = append(formattedKeys, StyleKey.Render(parts[0])+" "+StyleDesc.Render(parts[1]))
		}
	}
	keybar := strings.Join(formattedKeys, "  ")

	if statusLine != "" {
		return statusLine + "\n" + keybar
	}
	return keybar
}
