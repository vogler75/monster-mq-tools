package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
	"github.com/vogler75/monster-mq-tools/mbp/internal/runner"
)

type queuedTask struct {
	ComponentID string
	Target      component.Target
	Action      string
}

type FocusArea int

const (
	FocusMenu FocusArea = iota
	FocusLogs
)

type ComponentLogState struct {
	YOffset    int
	AutoScroll bool
	Task       *runner.Task
}

// AppModel is the primary Bubble Tea model for MBP.
type AppModel struct {
	RootDir        string
	Components     []*component.Component
	SelectedIdx    int
	LogViewer      LogViewer
	Dialog         DialogModel
	ActiveTask     *runner.Task
	ComponentTasks map[string]*runner.Task
	LogStates      map[string]*ComponentLogState
	Focus          FocusArea
	Queue          []queuedTask
	Spinner        spinner.Model

	Width         int
	Height        int
	FullscreenLog bool
	StatusMsg     string
}

// Custom Bubble Tea Messages
type logLineMsg struct {
	taskID string
	line   runner.LogLine
}

type taskDoneMsg struct {
	task *runner.Task
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

	m := &AppModel{
		RootDir:        rootDir,
		Components:     comps,
		SelectedIdx:    0,
		LogViewer:      lv,
		Spinner:        s,
		Queue:          make([]queuedTask, 0),
		ComponentTasks: make(map[string]*runner.Task),
		LogStates:      make(map[string]*ComponentLogState),
		Focus:          FocusMenu,
	}
	m.updateLogViewerForSelection()
	return m, nil
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
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			sel := m.selectedComponent()
			if sel != nil && m.ActiveTask.ComponentID == sel.ID {
				m.LogViewer.UpdateContent()
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		cmds = append(cmds, cmd)

	case logLineMsg:
		sel := m.selectedComponent()
		if sel != nil && sel.ID == msg.taskID {
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

	case tea.KeyMsg:
		cmd := m.handleKeyPress(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		// Forward mouse wheel scrolling to viewport
		var vpCmd tea.Cmd
		m.LogViewer.Viewport, vpCmd = m.LogViewer.Viewport.Update(msg)
		if vpCmd != nil {
			cmds = append(cmds, vpCmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// If modal is active
	if m.Dialog.Type != DialogNone {
		switch key {
		case "esc", "q":
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

	// Tab switches focus between menu table and log view
	if key == "tab" && !m.FullscreenLog {
		if m.Focus == FocusMenu {
			m.Focus = FocusLogs
		} else {
			m.Focus = FocusMenu
		}
		m.updateLogViewerForSelection()
		return nil
	}

	// Normal View Keybindings
	switch key {
	case "q":
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.StatusMsg = "Task running! Press 'x' to cancel task first, or Ctrl+C to force exit."
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
		if m.Focus == FocusLogs || m.FullscreenLog {
			m.LogViewer.Viewport.LineUp(1)
			m.LogViewer.AutoScroll = false
			return nil
		}
		if m.SelectedIdx > 0 {
			m.saveCurrentLogState()
			m.SelectedIdx--
			m.updateLogViewerForSelection()
		}

	case "down", "j":
		if m.Focus == FocusLogs || m.FullscreenLog {
			m.LogViewer.Viewport.LineDown(1)
			return nil
		}
		if m.SelectedIdx < len(m.Components)-1 {
			m.saveCurrentLogState()
			m.SelectedIdx++
			m.updateLogViewerForSelection()
		}

	case "pgup":
		m.LogViewer.Viewport.ViewUp()
		m.LogViewer.AutoScroll = false
		return nil

	case "pgdown":
		m.LogViewer.Viewport.ViewDown()
		return nil

	case "home":
		if m.Focus == FocusLogs || m.FullscreenLog {
			m.LogViewer.Viewport.GotoTop()
			m.LogViewer.AutoScroll = false
			return nil
		}

	case "end":
		if m.Focus == FocusLogs || m.FullscreenLog {
			m.LogViewer.Viewport.GotoBottom()
			m.LogViewer.AutoScroll = true
			return nil
		}

	case "u":
		// Git pull selected component
		sel := m.selectedComponent()
		if sel == nil {
			return nil
		}
		if !sel.Git.IsRepo {
			m.StatusMsg = fmt.Sprintf("%s is not a git repository.", sel.Name)
			return nil
		}
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.StatusMsg = "A task is already running! Please wait or cancel it."
			return nil
		}
		target := component.Target{
			ID:      "git-pull",
			Name:    "Git Pull",
			Command: "git",
			Args:    []string{"pull"},
		}
		m.StatusMsg = fmt.Sprintf("Pulling latest changes for %s...", sel.Name)
		return m.startTaskCmd(sel.ID, "pull", target)

	case "U":
		// Git pull ALL components sequentially
		return m.triggerPullAll()

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

	case "r":
		// Refresh status
		for _, c := range m.Components {
			component.RefreshStatus(c)
		}
		m.StatusMsg = "Component statuses refreshed."
		m.updateLogViewerForSelection()

	case "f", "l":
		// Toggle fullscreen log view
		m.FullscreenLog = !m.FullscreenLog
		if m.FullscreenLog {
			m.Focus = FocusLogs
		} else {
			m.Focus = FocusMenu
		}
		m.resizeLayout()
		m.updateLogViewerForSelection()

	case "a":
		// Toggle log autoscroll
		m.LogViewer.AutoScroll = !m.LogViewer.AutoScroll
		if m.LogViewer.AutoScroll {
			m.LogViewer.Viewport.GotoBottom()
		}

	case "x", "X":
		// Cancel active task
		if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
			m.ActiveTask.Cancel()
			m.StatusMsg = "Cancelling task..."
		}

	case "esc":
		// Go back from fullscreen logs or focus log view, or dismiss status message
		if m.FullscreenLog {
			m.FullscreenLog = false
			m.Focus = FocusMenu
			m.resizeLayout()
			m.updateLogViewerForSelection()
			return nil
		}
		if m.Focus == FocusLogs {
			m.Focus = FocusMenu
			m.updateLogViewerForSelection()
			return nil
		}
		if m.StatusMsg != "" {
			m.StatusMsg = ""
			return nil
		}

	case "enter":
		// Enter toggles fullscreen log view for selected component
		m.FullscreenLog = !m.FullscreenLog
		if m.FullscreenLog {
			m.Focus = FocusLogs
		} else {
			m.Focus = FocusMenu
		}
		m.resizeLayout()
		m.updateLogViewerForSelection()
		return nil

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

func (m *AppModel) isLogFocused() bool {
	return m.Focus == FocusLogs || m.FullscreenLog
}

func (m *AppModel) getOrCreateLogState(id string) *ComponentLogState {
	s, ok := m.LogStates[id]
	if !ok {
		s = &ComponentLogState{
			AutoScroll: true,
			YOffset:    0,
		}
		m.LogStates[id] = s
	}
	return s
}

func (m *AppModel) saveCurrentLogState() {
	if sel := m.selectedComponent(); sel != nil {
		state := m.getOrCreateLogState(sel.ID)
		state.YOffset = m.LogViewer.Viewport.YOffset
		state.AutoScroll = m.LogViewer.AutoScroll
	}
}

func (m *AppModel) updateLogViewerForSelection() {
	sel := m.selectedComponent()
	if sel == nil {
		m.LogViewer.SetComponent(nil, nil, m.isLogFocused())
		return
	}

	state := m.getOrCreateLogState(sel.ID)
	task := m.ComponentTasks[sel.ID]
	state.Task = task

	m.LogViewer.SetComponent(sel, task, m.isLogFocused())
	m.LogViewer.AutoScroll = state.AutoScroll
	m.LogViewer.Viewport.YOffset = state.YOffset
	if state.AutoScroll && task != nil {
		m.LogViewer.Viewport.GotoBottom()
	}
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

func (m *AppModel) triggerPullAll() tea.Cmd {
	if m.ActiveTask != nil && m.ActiveTask.Status == runner.TaskRunning {
		m.StatusMsg = "Cannot start Pull All: another task is already running!"
		return nil
	}

	m.Queue = nil
	for _, c := range m.Components {
		if !c.Git.IsRepo {
			continue
		}
		m.Queue = append(m.Queue, queuedTask{
			ComponentID: c.ID,
			Target: component.Target{
				ID:      "git-pull",
				Name:    "Git Pull",
				Command: "git",
				Args:    []string{"pull"},
			},
			Action: "pull",
		})
	}

	if len(m.Queue) == 0 {
		m.StatusMsg = "No git repositories available to pull."
		return nil
	}

	first := m.Queue[0]
	m.Queue = m.Queue[1:]
	m.StatusMsg = fmt.Sprintf("Started Pull All sequence (%d repositories queued).", len(m.Queue)+1)
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
	m.ComponentTasks[componentID] = task

	sel := m.selectedComponent()
	if sel != nil && sel.ID == componentID {
		m.LogViewer.SetComponent(comp, task, m.isLogFocused())
	}

	if action == "build" {
		comp.Status = component.StatusBuilding
	} else if action == "publish" {
		comp.Status = component.StatusPublishing
	} else if action == "pull" {
		comp.Status = component.StatusPulling
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
		if t.Action == "build" {
			comp.LastBuildTime = t.EndTime
			comp.LastBuildDuration = t.Duration
		}

		if t.Status == runner.TaskSuccess {
			if t.Action == "build" {
				comp.Status = component.StatusSuccess
			}
			m.StatusMsg = fmt.Sprintf("✔ %s %s finished successfully in %s", strings.ToUpper(t.Action), comp.Name, t.Duration.Round(time.Second))
		} else if t.Status == runner.TaskCancelled {
			if t.Action == "build" {
				comp.Status = component.StatusOutdated
			}
			m.StatusMsg = fmt.Sprintf("⊘ %s cancelled.", comp.Name)
		} else {
			if t.Action == "build" {
				comp.Status = component.StatusFailed
				comp.LastError = t.Error
			}
			m.StatusMsg = fmt.Sprintf("✘ %s failed (exit %d): %s", comp.Name, t.ExitCode, t.Error)
		}
	}
	sel := m.selectedComponent()
	if sel != nil && sel.ID == t.ComponentID {
		m.LogViewer.UpdateContent()
	}
}

func (m *AppModel) resizeLayout() {
	w := max(m.Width, 60)
	h := max(m.Height, 16)

	headerHeight := 3

	footerHeight := 1
	if m.StatusMsg != "" {
		footerHeight = 2
	}

	tableHeight := 0
	if !m.FullscreenLog {
		// Border top (1) + Border bottom (1) + Header (1) + Separator (1) + len(m.Components)
		tableHeight = 4 + len(m.Components)
	}

	logPaneOverhead := 3 // Border top (1) + Border bottom (1) + Log Title (1)

	fixedLines := headerHeight + tableHeight + logPaneOverhead + footerHeight

	logH := h - fixedLines
	if logH < 3 {
		logH = 3
	}

	innerW := max(w-6, 50)
	m.LogViewer.SetSize(innerW, logH)
}

func (m *AppModel) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing MBP..."
	}

	m.resizeLayout()

	var sb strings.Builder

	// 1. Header Banner
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	// 2. Component Status Table
	if !m.FullscreenLog {
		tableStyle := StylePane.Copy().UnsetHeight()
		if m.Focus == FocusMenu {
			tableStyle = StylePaneActive.Copy().UnsetHeight()
		}
		tableBox := tableStyle.Width(m.Width - 4).Render(RenderComponentTable(m.Components, m.SelectedIdx, m.Width-6))
		sb.WriteString(tableBox)
		sb.WriteString("\n")
	}

	// 3. Log Output Pane
	logStyle := StylePane.Copy().UnsetHeight()
	if m.Focus == FocusLogs || m.FullscreenLog {
		logStyle = StylePaneActive.Copy().UnsetHeight()
	}
	logHeader := m.LogViewer.RenderHeader(m.Width-6, m.isLogFocused())
	logBody := m.LogViewer.Viewport.View()
	logPane := logStyle.Width(m.Width - 4).Render(logHeader + "\n" + logBody)
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
	innerW := max(m.Width-6, 40)
	title := StyleTitle.Render(" MBP ") + " " + StyleSubtitle.Render("MonsterMQ Build Pipeline")

	rootText := fmt.Sprintf("Root: %s", m.RootDir)
	if lipgloss.Width(rootText) > 28 && m.Width < 100 {
		rootText = fmt.Sprintf("Root: .../%s", filepath.Base(m.RootDir))
	}
	pathStr := StyleDim.Render(rootText)

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
	if behindCount > 0 && m.Width >= 95 {
		stats += " | " + BadgeBehind.Render(fmt.Sprintf("Git Updates: %d", behindCount))
	}

	left := title
	if m.Width >= 80 {
		left += "  " + pathStr
	}
	right := stats

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	if leftW+rightW+2 > innerW {
		right = fmt.Sprintf("%s | %s", BadgeBuilt.Render(fmt.Sprintf("B:%d", builtCount)), BadgeOutdated.Render(fmt.Sprintf("O:%d", outdatedCount)))
		rightW = lipgloss.Width(right)
	}

	space := max(innerW-leftW-rightW, 1)
	line := left + strings.Repeat(" ", space) + right
	if lipgloss.Width(line) > innerW {
		line = truncateVisible(line, innerW)
	}

	return StyleHeader.Width(m.Width - 4).Render(line)
}

func (m *AppModel) renderFooter() string {
	var statusLine string
	if m.StatusMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(ColorWarning).Render("ℹ " + m.StatusMsg)
	}

	var keys []string
	if m.FullscreenLog {
		keys = []string{
			"[Esc/f] Back to Menu",
			"[↑/↓] Scroll",
			"[PgUp/PgDn] Page",
			"[a] Autoscroll",
			"[x] Cancel Task",
			"[?] Help",
			"[q] Quit",
		}
	} else if m.Focus == FocusLogs {
		keys = []string{
			"[Tab/Esc] Back to Menu",
			"[↑/↓] Scroll Log",
			"[PgUp/PgDn] Page",
			"[a] Autoscroll",
			"[f] Fullscreen",
			"[x] Cancel Task",
			"[?] Help",
			"[q] Quit",
		}
	} else {
		if m.Width < 105 {
			keys = []string{
				"[↑/↓] Select",
				"[Tab] Focus Log",
				"[b] Build",
				"[u] Pull",
				"[p] Publish",
				"[f] Full Log",
				"[?] Help",
				"[q] Quit",
			}
		} else {
			keys = []string{
				"[↑/↓] Select",
				"[Tab] Focus Log",
				"[b] Build",
				"[B] Build All",
				"[u] Pull",
				"[U] Pull All",
				"[p] Publish",
				"[c] Clean",
				"[f] Full Log",
				"[x] Kill",
				"[?] Help",
				"[q] Quit",
			}
		}
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
