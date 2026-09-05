package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vogler75/monster-mq-tools/mbp/internal/component"
)

// RenderComponentTable renders the list of components with selection and statuses.
func RenderComponentTable(comps []*component.Component, selectedIdx int, width int) string {
	var sb strings.Builder

	// Table Header
	headerCols := fmt.Sprintf(
		"  %-14s %-10s %-24s %-16s %s",
		"COMPONENT", "VERSION", "GIT STATUS", "BUILD STATUS", "LATEST ARTIFACT",
	)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(headerCols))
	sb.WriteString("\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", max(width-4, 70))))
	sb.WriteString("\n")

	for i, c := range comps {
		isSel := i == selectedIdx

		// 1. Selector icon
		prefix := "  "
		if isSel {
			prefix = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("❯ ")
		}

		// 2. Component ID / Name
		nameStr := c.ID
		if len(nameStr) > 14 {
			nameStr = nameStr[:14]
		}
		compCol := fmt.Sprintf("%-14s", nameStr)

		// 3. Version
		vStr := c.Version
		if vStr == "" || vStr == "unknown" {
			vStr = "-"
		} else if !strings.HasPrefix(vStr, "v") {
			vStr = "v" + vStr
		}
		if len(vStr) > 10 {
			vStr = vStr[:10]
		}
		versionCol := fmt.Sprintf("%-10s", vStr)

		// 4. Git Status string
		gitCol := renderGitStatus(c)

		// 5. Build Status string
		buildCol := renderBuildStatus(c)

		// 6. Artifact info
		artCol := renderArtifactInfo(c)

		rowText := fmt.Sprintf("%s%s %s %s %s %s", prefix, compCol, versionCol, gitCol, buildCol, artCol)

		if isSel {
			sb.WriteString(StyleSelectedRow.Render(rowText))
		} else {
			sb.WriteString(StyleNormalRow.Render(rowText))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderGitStatus(c *component.Component) string {
	if !c.Git.IsRepo {
		return StyleDim.Render(fmt.Sprintf("%-24s", "no git repo"))
	}

	branch := c.Git.Branch
	if branch == "" {
		branch = "HEAD"
	}
	if len(branch) > 10 {
		branch = branch[:10]
	}

	var parts []string
	parts = append(parts, branch)

	if c.Git.Behind > 0 {
		parts = append(parts, BadgeBehind.Render(fmt.Sprintf("▼%d behind!", c.Git.Behind)))
	} else if c.Git.Ahead > 0 {
		parts = append(parts, BadgeAhead.Render(fmt.Sprintf("▲%d", c.Git.Ahead)))
	}

	if c.Git.IsDirty {
		parts = append(parts, BadgeDirty.Render(fmt.Sprintf("*%d", c.Git.DirtyFilesCount)))
	} else if c.Git.Behind == 0 && c.Git.Ahead == 0 {
		parts = append(parts, BadgeClean.Render("✔clean"))
	}

	out := strings.Join(parts, " ")
	// Pad or truncate visually
	visLen := lipgloss.Width(out)
	if visLen < 24 {
		out += strings.Repeat(" ", 24-visLen)
	}
	return out
}

func renderBuildStatus(c *component.Component) string {
	var badge string
	switch c.Status {
	case component.StatusBuilt:
		badge = BadgeBuilt.Render("● Built")
	case component.StatusOutdated:
		badge = BadgeOutdated.Render("○ Outdated")
	case component.StatusNotBuilt:
		badge = BadgeNotBuilt.Render("✕ Missing")
	case component.StatusBuilding:
		badge = BadgeBuilding.Render("⚙ Building...")
	case component.StatusPublishing:
		badge = BadgePublishing.Render("🚀 Publishing...")
	case component.StatusSuccess:
		badge = BadgeBuilt.Render("✔ Success")
	case component.StatusFailed:
		badge = BadgeNotBuilt.Render("✘ Failed")
	default:
		badge = StyleDim.Render(string(c.Status))
	}

	visLen := lipgloss.Width(badge)
	if visLen < 16 {
		badge += strings.Repeat(" ", 16-visLen)
	}
	return badge
}

func renderArtifactInfo(c *component.Component) string {
	if c.LatestArtifact == nil {
		return StyleDim.Render("none")
	}

	sizeStr := formatFileSize(c.LatestArtifact.Size)
	timeStr := formatRelativeTime(c.LatestArtifact.ModTime)

	name := c.LatestArtifact.Name
	if len(name) > 26 {
		name = name[:23] + "..."
	}

	return fmt.Sprintf("%s (%s, %s)", name, sizeStr, StyleDim.Render(timeStr))
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
