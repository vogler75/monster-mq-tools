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

	// Allocate column widths based on available width
	availWidth := max(width, 60)

	// Fixed columns:
	// prefix: 2 ("❯ " or "  ")
	// compCol: 12
	// versionCol: 9
	// gitCol: 18
	// buildCol: 14
	// Spaces between columns: 4 (1 each between 5 items)
	// Total fixed = 2 + 12 + 1 + 9 + 1 + 18 + 1 + 14 + 1 = 59
	fixedWidth := 59
	artWidth := availWidth - fixedWidth
	if artWidth < 10 {
		artWidth = 10
	}

	// Table Header
	headerCols := fmt.Sprintf(
		"  %-12s %-9s %-18s %-14s %s",
		"COMPONENT", "VERSION", "GIT STATUS", "BUILD STATUS", "LATEST ARTIFACT",
	)
	if lipgloss.Width(headerCols) > availWidth {
		headerCols = truncateVisible(headerCols, availWidth)
	} else if lenRemain := availWidth - lipgloss.Width(headerCols); lenRemain > 0 {
		headerCols += strings.Repeat(" ", lenRemain)
	}
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(headerCols))
	sb.WriteString("\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", availWidth)))
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
		if len(nameStr) > 12 {
			nameStr = nameStr[:12]
		}
		compCol := fmt.Sprintf("%-12s", nameStr)

		// 3. Version
		vStr := c.Version
		if vStr == "" || vStr == "unknown" {
			vStr = "-"
		} else if !strings.HasPrefix(vStr, "v") {
			vStr = "v" + vStr
		}
		if len(vStr) > 9 {
			vStr = vStr[:9]
		}
		versionCol := fmt.Sprintf("%-9s", vStr)

		// 4. Git Status string
		gitCol := renderGitStatus(c, 18)

		// 5. Build Status string
		buildCol := renderBuildStatus(c, 14)

		// 6. Artifact info
		artCol := renderArtifactInfo(c, artWidth)

		rowText := fmt.Sprintf("%s%s %s %s %s %s", prefix, compCol, versionCol, gitCol, buildCol, artCol)

		// Ensure row visible width is padded to exactly availWidth
		rowVis := lipgloss.Width(rowText)
		if rowVis < availWidth {
			rowText += strings.Repeat(" ", availWidth-rowVis)
		}

		if isSel {
			sb.WriteString(StyleSelectedRow.Width(availWidth).Render(rowText))
		} else {
			sb.WriteString(StyleNormalRow.Render(rowText))
		}
		if i < len(comps)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func renderGitStatus(c *component.Component, targetWidth int) string {
	if !c.Git.IsRepo {
		str := "no git repo"
		if len(str) < targetWidth {
			str += strings.Repeat(" ", targetWidth-len(str))
		}
		return StyleDim.Render(str)
	}

	branch := c.Git.Branch
	if branch == "" {
		branch = "HEAD"
	}
	if len(branch) > 8 {
		branch = branch[:8]
	}

	var parts []string
	parts = append(parts, branch)

	if c.Git.Behind > 0 {
		parts = append(parts, BadgeBehind.Render(fmt.Sprintf("▼%d", c.Git.Behind)))
	} else if c.Git.Ahead > 0 {
		parts = append(parts, BadgeAhead.Render(fmt.Sprintf("▲%d", c.Git.Ahead)))
	}

	if c.Git.IsDirty {
		parts = append(parts, BadgeDirty.Render(fmt.Sprintf("*%d", c.Git.DirtyFilesCount)))
	} else if c.Git.Behind == 0 && c.Git.Ahead == 0 {
		parts = append(parts, BadgeClean.Render("✔clean"))
	}

	out := strings.Join(parts, " ")
	visLen := lipgloss.Width(out)
	if visLen < targetWidth {
		out += strings.Repeat(" ", targetWidth-visLen)
	}
	return out
}

func renderBuildStatus(c *component.Component, targetWidth int) string {
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
		badge = BadgePublishing.Render("🚀 Publishing")
	case component.StatusPulling:
		badge = BadgePulling.Render("📥 Pulling...")
	case component.StatusSuccess:
		badge = BadgeBuilt.Render("✔ Success")
	case component.StatusFailed:
		badge = BadgeNotBuilt.Render("✘ Failed")
	default:
		badge = StyleDim.Render(string(c.Status))
	}

	visLen := lipgloss.Width(badge)
	if visLen < targetWidth {
		badge += strings.Repeat(" ", targetWidth-visLen)
	}
	return badge
}

func renderArtifactInfo(c *component.Component, targetWidth int) string {
	if c.LatestArtifact == nil {
		str := "none"
		if len(str) < targetWidth {
			str += strings.Repeat(" ", targetWidth-len(str))
		}
		return StyleDim.Render(str)
	}

	sizeStr := formatFileSize(c.LatestArtifact.Size)
	timeStr := formatRelativeTime(c.LatestArtifact.ModTime)

	name := c.LatestArtifact.Name
	fullInfo := fmt.Sprintf("%s (%s, %s)", name, sizeStr, timeStr)
	if lipgloss.Width(fullInfo) > targetWidth {
		shortInfo := fmt.Sprintf("%s (%s)", name, sizeStr)
		if lipgloss.Width(shortInfo) > targetWidth {
			fullInfo = truncateVisible(shortInfo, targetWidth)
		} else {
			fullInfo = shortInfo
		}
	}

	visLen := lipgloss.Width(fullInfo)
	if visLen < targetWidth {
		fullInfo += strings.Repeat(" ", targetWidth-visLen)
	}
	return fullInfo
}

func truncateVisible(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return ""
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

