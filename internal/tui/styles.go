package tui

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Base Colors
	ColorPrimary   = lipgloss.Color("#7D56F4") // Purple/Violet
	ColorSecondary = lipgloss.Color("#04B575") // Emerald Green
	ColorAccent    = lipgloss.Color("#FF79C6") // Pink
	ColorWarning   = lipgloss.Color("#FFB86C") // Amber
	ColorError     = lipgloss.Color("#FF5555") // Red
	ColorMuted     = lipgloss.Color("#6272A4") // Muted Blue/Gray
	ColorSubtle    = lipgloss.Color("#44475A") // Darker Slate
	ColorWhite     = lipgloss.Color("#F8F8F2")
	ColorHighlight = lipgloss.Color("#F1FA8C") // Yellow
	ColorCyan      = lipgloss.Color("#8BE9FD") // Cyan

	// Header & Title Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorPrimary).
			Padding(0, 1)

	SubTitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HeaderBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorSubtle).
			Padding(0, 1)

	// Status Badges
	BadgeOk = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#000000")).
		Background(ColorSecondary).
		Padding(0, 1).
		SetString(" OK ")

	BadgeError = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite).
		Background(ColorError).
		Padding(0, 1).
		SetString(" ERR ")

	BadgePaused = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(ColorWarning).
			Padding(0, 1).
			SetString(" PAUSED ")

	BadgeLive = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Background(ColorSecondary).
			Padding(0, 1).
			SetString(" LIVE ")

	BadgeCriticalPath = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF5555")).
				SetString("[CRITICAL PATH]")

	// List & Table Styles
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWhite).
				Background(ColorSubtle).
				Padding(0, 1)

	RowSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWhite).
				Background(lipgloss.Color("#383A59"))

	RowNormalStyle = lipgloss.NewStyle().
			Foreground(ColorWhite)

	// Panel Styles
	PanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorSubtle).
			Padding(0, 1)

	PanelActiveStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	// Key Help Styles
	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCyan)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Service palette for deterministic distinct colors per service
	servicePalette = []lipgloss.Color{
		lipgloss.Color("#8BE9FD"), // Cyan
		lipgloss.Color("#50FA7B"), // Green
		lipgloss.Color("#FFB86C"), // Orange
		lipgloss.Color("#FF79C6"), // Pink
		lipgloss.Color("#BD93F9"), // Purple
		lipgloss.Color("#F1FA8C"), // Yellow
		lipgloss.Color("#69FF94"), // Light Green
		lipgloss.Color("#D6ACFF"), // Lavender
	}
)

// ServiceColor returns a consistent color for a given service name.
func ServiceColor(svcName string) lipgloss.Color {
	if svcName == "" || svcName == "unknown-service" {
		return ColorMuted
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(svcName))
	idx := int(h.Sum32()) % len(servicePalette)
	if idx < 0 {
		idx = -idx
	}
	return servicePalette[idx]
}
