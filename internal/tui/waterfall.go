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

	// Calculate available left column width for the tree hierarchy
	availableTreeWidth := width - barWidth - 24
	if availableTreeWidth < 20 {
		availableTreeWidth = 20
	}

	// Column header
	treeColHeader := "SPAN HIERARCHY"
	timeColHeader := fmt.Sprintf("TIMELINE (%s total)", FormatDuration(trace.Duration))
	headerLeft := PadRight("  "+treeColHeader, availableTreeWidth+2)
	headerLine := headerLeft + timeColHeader
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorMuted).Render(headerLine))

	for i := startIdx; i < endIdx; i++ {
		s := trace.SpanTree[i]
		isSelected := i == selectedSpanIdx

		// Cursor indicator (2 columns)
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

		// Operation name: truncate safely to fit available tree width
		maxOpWidth := availableTreeWidth - (s.Depth * 2) - lipgloss.Width(s.ServiceName) - 16
		if maxOpWidth < 10 {
			maxOpWidth = 10
		}
		opName := s.Name
		if len(opName) > maxOpWidth {
			if maxOpWidth > 3 {
				opName = opName[:maxOpWidth-3] + "..."
			} else {
				opName = opName[:maxOpWidth]
			}
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

		// Exactly padded left column so timingInfo always starts at the same visual column
		leftCol := cursor + leftContent
		leftPadded := PadRight(leftCol, availableTreeWidth+2)
		rowText := leftPadded + timingInfo

		if isSelected {
			lines = append(lines, RowSelectedStyle.Width(width).Render(rowText))
		} else {
			lines = append(lines, RowNormalStyle.Render(rowText))
		}
	}

	return strings.Join(lines, "\n")
}
