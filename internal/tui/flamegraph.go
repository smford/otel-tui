package tui

import (
	"fmt"
	"strings"

	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// RenderFlamegraph renders spans as stacked flame layers grouped by depth.
func RenderFlamegraph(trace *store.Trace, selectedSpanIdx int, width, height int) string {
	if trace == nil || len(trace.SpanTree) == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("No spans available.")
	}

	if width < 30 {
		width = 30
	}

	// Group spans by depth level
	maxDepth := 0
	depthMap := make(map[int][]*store.Span)
	for _, s := range trace.SpanTree {
		depthMap[s.Depth] = append(depthMap[s.Depth], s)
		if s.Depth > maxDepth {
			maxDepth = s.Depth
		}
	}

	var lines []string
	header := fmt.Sprintf("FLAMEGRAPH VIEW [f: switch to waterfall] — Total Trace Duration: %s", FormatDuration(trace.Duration))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Render(header))
	lines = append(lines, "")

	var selectedSpan *store.Span
	if selectedSpanIdx >= 0 && selectedSpanIdx < len(trace.SpanTree) {
		selectedSpan = trace.SpanTree[selectedSpanIdx]
	}

	for d := 0; d <= maxDepth; d++ {
		spansAtDepth := depthMap[d]
		if len(spansAtDepth) == 0 {
			continue
		}

		// Allocate a character buffer for this depth row
		rowRunes := make([]rune, width)
		for i := range rowRunes {
			rowRunes[i] = ' '
		}

		var rowSegments []string
		lastEnd := 0

		for _, s := range spansAtDepth {
			coords := ComputeBarCoordinates(width, s.RelativeOffset, s.RelativeWidth)
			if coords.SpanWidth < 1 {
				coords.SpanWidth = 1
			}

			// Add gap before span if any
			gap := coords.StartCol - lastEnd
			if gap > 0 {
				rowSegments = append(rowSegments, strings.Repeat(" ", gap))
			} else if gap < 0 {
				coords.StartCol = lastEnd
			}

			// Block label: e.g. "api: /checkout (180ms)"
			label := fmt.Sprintf("%s: %s (%s)", s.ServiceName, s.Name, FormatDuration(s.Duration))
			if len(label) > coords.SpanWidth-2 {
				label = fmt.Sprintf("%s (%s)", s.Name, FormatDuration(s.Duration))
			}
			if len(label) > coords.SpanWidth-2 {
				label = s.Name
			}
			if len(label) > coords.SpanWidth-2 {
				if coords.SpanWidth > 4 {
					label = label[:coords.SpanWidth-4] + ".."
				} else {
					label = ""
				}
			}

			// Center or pad label inside block
			padTotal := coords.SpanWidth - len(label)
			if padTotal < 0 {
				padTotal = 0
			}
			padL := padTotal / 2
			padR := padTotal - padL

			blockText := strings.Repeat("█", padL) + label + strings.Repeat("█", padR)
			if len(blockText) > coords.SpanWidth {
				blockText = blockText[:coords.SpanWidth]
			}

			// Style block
			isSelected := selectedSpan != nil && s.SpanID == selectedSpan.SpanID
			var blockStyle lipgloss.Style

			if isSelected {
				blockStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#000000")).
					Background(ColorHighlight)
			} else if s.HasError || s.StatusCode == "ERROR" {
				blockStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorWhite).
					Background(ColorError)
			} else if s.IsCriticalPath {
				blockStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#000000")).
					Background(ColorWarning)
			} else {
				bg := ServiceColor(s.ServiceName)
				blockStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#000000")).
					Background(bg)
			}

			rowSegments = append(rowSegments, blockStyle.Render(blockText))
			lastEnd = coords.StartCol + coords.SpanWidth
		}

		depthPrefix := fmt.Sprintf("L%d │ ", d)
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render(depthPrefix)+strings.Join(rowSegments, ""))
	}

	return strings.Join(lines, "\n")
}
