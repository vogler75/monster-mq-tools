package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
)

// DialogType indicates the active modal type.
type DialogType int

const (
	DialogNone DialogType = iota
	DialogBuildTarget
	DialogPublishTarget
	DialogCleanConfirm
	DialogHelp
)

// DialogModel holds modal state.
type DialogModel struct {
	Type        DialogType
	Component   *component.Component
	Targets     []component.Target
	SelectedIdx int
}

// RenderModal renders the active dialog modal centered in width and height.
func RenderModal(d *DialogModel, width, height int) string {
	if d.Type == DialogNone || d.Component == nil {
		return ""
	}

	var content strings.Builder

	switch d.Type {
	case DialogBuildTarget:
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("🛠  SELECT BUILD TARGET"))
		content.WriteString("\n")
		content.WriteString(StyleDim.Render(fmt.Sprintf("Component: %s (%s)", d.Component.Name, d.Component.ID)))
		content.WriteString("\n\n")

		for i, t := range d.Targets {
			cursor := "  "
			if i == d.SelectedIdx {
				cursor = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("❯ ")
			}
			name := t.Name
			if i == d.SelectedIdx {
				name = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(name)
			}
			content.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
			content.WriteString(fmt.Sprintf("    %s\n", StyleDim.Render(t.Description)))
		}

		content.WriteString("\n")
		content.WriteString(StyleDim.Render("[↑/↓] Select   [Enter] Start Build   [Esc] Cancel"))

	case DialogPublishTarget:
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render("🚀  CONFIRM PUBLISHING"))
		content.WriteString("\n")
		content.WriteString(StyleDim.Render(fmt.Sprintf("Component: %s (v%s)", d.Component.Name, d.Component.Version)))
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(ColorWarning).Render("⚠  WARNING: This will upload release assets to external repositories!"))
		content.WriteString("\n\n")

		for i, t := range d.Targets {
			cursor := "  "
			if i == d.SelectedIdx {
				cursor = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render("❯ ")
			}
			name := t.Name
			if i == d.SelectedIdx {
				name = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(name)
			}
			content.WriteString(fmt.Sprintf("%s%s\n", cursor, name))
			content.WriteString(fmt.Sprintf("    %s\n", StyleDim.Render(t.Description)))
		}

		content.WriteString("\n")
		content.WriteString(StyleDim.Render("[↑/↓] Select   [Enter] Confirm & Publish   [Esc] Cancel"))

	case DialogCleanConfirm:
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorDanger).Render("🗑  CONFIRM CLEAN ARTIFACTS"))
		content.WriteString("\n\n")
		content.WriteString(fmt.Sprintf("Are you sure you want to clean build outputs for %s?\n\n", d.Component.Name))
		content.WriteString(StyleDim.Render("[Enter] Clean   [Esc] Cancel"))

	case DialogHelp:
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("📖  MBP KEYBOARD SHORTCUTS"))
		content.WriteString("\n\n")
		shortcuts := [][]string{
			{"[↑/↓] or [j/k]", "Navigate component list (when menu active)"},
			{"[Tab]", "Toggle focus between Component Menu & Log Viewer"},
			{"[↑/↓] or [j/k]", "Scroll log lines (when log focused or fullscreen)"},
			{"[PgUp/PgDn]", "Scroll log output pages up/down"},
			{"[b]", "Build selected component (opens target picker)"},
			{"[B]", "Build ALL components sequentially"},
			{"[u]", "Git pull latest updates for selected component"},
			{"[U]", "Git pull latest updates for ALL components"},
			{"[p]", "Publish selected component to GitHub / Docker"},
			{"[c]", "Clean build outputs for selected component"},
			{"[r]", "Refresh status without fetching remote"},
			{"[Enter] / [f]", "Open / toggle full-screen live log viewer"},
			{"[Esc]", "Back to Menu from log view, fullscreen, or dialog"},
			{"[a]", "Toggle log autoscroll on/off"},
			{"[x] / [Ctrl+C]", "Cancel / kill running build process"},
			{"[?]", "Toggle this help dialog"},
			{"[q]", "Quit MBP"},
		}
		for _, sc := range shortcuts {
			content.WriteString(fmt.Sprintf("  %-16s %s\n", StyleKey.Render(sc[0]), StyleDesc.Render(sc[1])))
		}
		content.WriteString("\n")
		content.WriteString(StyleDim.Render("[Esc] or [?] Close Help"))
	}

	dialogBox := StyleDialog.Width(min(width-10, 68)).Render(content.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialogBox)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
