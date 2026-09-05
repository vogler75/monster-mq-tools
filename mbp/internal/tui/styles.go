package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Brand colors
	ColorPrimary   = lipgloss.Color("#00F0FF") // Cyan neon
	ColorSecondary = lipgloss.Color("#BD00FF") // Electric Purple
	ColorSuccess   = lipgloss.Color("#00FF66") // Neon Green
	ColorWarning   = lipgloss.Color("#FFB800") // Neon Amber / Yellow
	ColorDanger    = lipgloss.Color("#FF0055") // Neon Red
	ColorMuted     = lipgloss.Color("#6E7681") // Dim Gray
	ColorHighlight = lipgloss.Color("#21262D") // Card background
	ColorWhite     = lipgloss.Color("#F0F6FC")

	// Base styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorSecondary).
			Padding(0, 1)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleHeader = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorSecondary).
			Padding(0, 1).
			MarginBottom(0)

	StylePane = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#30363D")).
			Padding(0, 1)

	StylePaneActive = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	StyleSelectedRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(lipgloss.Color("#1F2430"))

	StyleNormalRow = lipgloss.NewStyle().
			Foreground(ColorWhite)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleKey = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleDesc = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Status Badges
	BadgeBuilt = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	BadgeOutdated = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	BadgeNotBuilt = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	BadgeBuilding = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	BadgePublishing = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	BadgeClean = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BadgeDirty = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6"))

	BadgeBehind = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	BadgeAhead = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Modal Dialog
	StyleDialog = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Background(lipgloss.Color("#161B22"))
)
