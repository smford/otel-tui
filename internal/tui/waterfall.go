package tui

import (
	"fmt"
	"strings"

	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// WaterfallRow contains the formatted visual components for a single span row in the waterfall.
type WaterfallRow struct {
	Span       *store.Span
	TreePrefix string
	Service    string
	Name       string
	Duration   string
	Percentage string
	TimingBar  string
	IsSelected bool
}

// RenderWaterfall renders the tree of spans with tree indentations and timing bars.
func RenderWaterfall(trace *store.Trace, selectedSpanIdx int, width, height int) string {
	if trace == nil || len(trace.SpanTree) == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("No spans available in this trace.")
	}

	// Bar width allocation: 25% to 35% of total width, capped between 12 and 40 columns
	barWidth := width / 4
	if barWidth < 12 {
		barWidth = 12
	}
	if barWidth > 40 {
		barWidth = 40
	}

	// Calculate scrolling window if there are more spans than height
	visibleRows := height
	if visibleRows <= 0 {
		visibleRows = 10
	}

	startIdx := 0
	if selectedSpanIdx >= visibleRows {
		startIdx = selectedSpanIdx - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(trace.SpanTree) {
		endIdx = len(trace.SpanTree)
	}

	var lines []string

	// Column header
	treeColHeader := "SPAN HIERARCHY"
	timeColHeader := fmt.Sprintf("TIMELINE (%s total)", FormatDuration(trace.Duration))
	headerLeftWidth := width - barWidth - 22
	if headerLeftWidth < 20 {
		headerLeftWidth = 20
	}
	headerLine := fmt.Sprintf("  %-*s  %s", headerLeftWidth, treeColHeader, timeColHeader)
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorMuted).Render(headerLine))

	for i := startIdx; i < endIdx; i++ {
		s := trace.SpanTree[i]
		isSelected := i == selectedSpanIdx

		// Cursor indicator
		cursor := "  "
		if isSelected {
			cursor = "❯ "
		}

		// Tree indentation
		indent := strings.Repeat("  ", s.Depth)
		var branchIcon string
		if s.Depth == 0 {
			branchIcon = "◆ "
		} else {
			branchIcon = "├─"
		}

		// Status / Error marker
		statusIcon := " "
		if s.HasError || s.StatusCode == "ERROR" {
			statusIcon = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render("✖ ")
		} else {
			statusIcon = lipgloss.NewStyle().Foreground(ColorSecondary).Render("● ")
		}

		// Service tag
		svcColor := ServiceColor(s.ServiceName)
		svcText := lipgloss.NewStyle().Foreground(svcColor).Bold(true).Render(s.ServiceName)

		// Critical path indicator
		cpText := ""
		if s.IsCriticalPath {
			cpText = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(" [CP]")
		}

		// N+1 query indicator
		n1Text := ""
		if s.IsNPlusOne {
			n1Text = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(" [N+1]")
		}

		// Span operation name
		opName := s.Name
		if len(opName) > 28 {
			opName = opName[:25] + "..."
		}

		// Left tree column content
		leftContent := fmt.Sprintf("%s%s%s%s: %s%s%s", indent, branchIcon, statusIcon, svcText, opName, cpText, n1Text)

		// Timing bar calculation
		barStr := RenderTimingBar(barWidth, s.RelativeOffset, s.RelativeWidth, '█', '░')
		var styledBar string
		if s.HasError || s.StatusCode == "ERROR" {
			styledBar = lipgloss.NewStyle().Foreground(ColorError).Render(barStr)
		} else if s.IsCriticalPath {
			styledBar = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(barStr)
		} else {
			styledBar = lipgloss.NewStyle().Foreground(ColorCyan).Render(barStr)
		}

		durText := FormatDuration(s.Duration)
		pct := int(s.RelativeWidth * 100)
		if pct > 100 {
			pct = 100
		}
		timingInfo := fmt.Sprintf("[%s] %7s (%2d%%)", styledBar, durText, pct)

		// Format line with spacing
		availableTreeWidth := width - barWidth - 24
		if availableTreeWidth < 20 {
			availableTreeWidth = 20
		}

		// Strip ANSI width calculation for clean alignment
		treeTextPlain := stripAnsi(leftContent)
		pad := availableTreeWidth - len(treeTextPlain)
		if pad < 1 {
			pad = 1
		}
		padding := strings.Repeat(" ", pad)

		rowText := fmt.Sprintf("%s%s%s%s", cursor, leftContent, padding, timingInfo)

		if isSelected {
			lines = append(lines, RowSelectedStyle.Width(width).Render(rowText))
		} else {
			lines = append(lines, RowNormalStyle.Render(rowText))
		}
	}

	return strings.Join(lines, "\n")
}

// Simple ANSI stripper for padding calculation
func stripAnsi(str string) string {
	var sb strings.Builder
	inAnsi := false
	for _, r := range str {
		if r == '\x1b' {
			inAnsi = true
			continue
		}
		if inAnsi {
			if r == 'm' {
				inAnsi = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
