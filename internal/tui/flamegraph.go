package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
	header := fmt.Sprintf("FLAMEGRAPH VIEW [f: switch to Waterfall] — Total Duration: %s", FormatDuration(trace.Duration))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Render(header))
	lines = append(lines, "")

	var selectedSpan *store.Span
	if selectedSpanIdx >= 0 && selectedSpanIdx < len(trace.SpanTree) {
		selectedSpan = trace.SpanTree[selectedSpanIdx]
	}

	// Fixed prefix width: " L00 │ " is 7 chars
	prefixWidth := 7
	barWidth := width - prefixWidth - 4
	if barWidth < 20 {
		barWidth = 20
	}

	for d := 0; d <= maxDepth; d++ {
		spansAtDepth := depthMap[d]
		if len(spansAtDepth) == 0 {
			continue
		}

		var rowSegments []string
		lastEnd := 0

		for _, s := range spansAtDepth {
			coords := ComputeBarCoordinates(barWidth, s.RelativeOffset, s.RelativeWidth)
			if coords.SpanWidth < 1 {
				coords.SpanWidth = 1
			}

			// Prevent overlapping spans on the same depth line
			if coords.StartCol < lastEnd {
				coords.StartCol = lastEnd
			}
			if coords.StartCol >= barWidth {
				break
			}
			if coords.StartCol+coords.SpanWidth > barWidth {
				coords.SpanWidth = barWidth - coords.StartCol
			}
			if coords.SpanWidth < 1 {
				coords.SpanWidth = 1
			}

			// Add gap between spans if any
			gap := coords.StartCol - lastEnd
			if gap > 0 {
				rowSegments = append(rowSegments, strings.Repeat(" ", gap))
			}

			// Format label to fit inside the block
			label := formatFlamegraphLabel(s, coords.SpanWidth)

			// Pad label with spaces to fill exact spanWidth columns
			labelWidth := utf8.RuneCountInString(label)
			if labelWidth > coords.SpanWidth {
				runes := []rune(label)
				label = string(runes[:coords.SpanWidth])
				labelWidth = coords.SpanWidth
			}
			remainingPad := coords.SpanWidth - labelWidth
			padLeft := remainingPad / 2
			padRight := remainingPad - padLeft
			cellText := strings.Repeat(" ", padLeft) + label + strings.Repeat(" ", padRight)

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

			rowSegments = append(rowSegments, blockStyle.Render(cellText))
			lastEnd = coords.StartCol + coords.SpanWidth
		}

		// Trailing empty space to maintain clean row width
		if lastEnd < barWidth {
			rowSegments = append(rowSegments, strings.Repeat(" ", barWidth-lastEnd))
		}

		depthPrefix := fmt.Sprintf(" L%-2d │ ", d)
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render(depthPrefix)+strings.Join(rowSegments, ""))
	}

	return strings.Join(lines, "\n")
}

func formatFlamegraphLabel(s *store.Span, spanWidth int) string {
	durStr := FormatDuration(s.Duration)

	// Try: " service: op (12ms) "
	full := fmt.Sprintf(" %s: %s (%s) ", s.ServiceName, s.Name, durStr)
	if utf8.RuneCountInString(full) <= spanWidth {
		return full
	}

	// Try: " op (12ms) "
	medium := fmt.Sprintf(" %s (%s) ", s.Name, durStr)
	if utf8.RuneCountInString(medium) <= spanWidth {
		return medium
	}

	// Try: " op "
	short := fmt.Sprintf(" %s ", s.Name)
	if utf8.RuneCountInString(short) <= spanWidth {
		return short
	}

	// Try truncated: " op... "
	if spanWidth >= 6 {
		runes := []rune(s.Name)
		maxRunes := spanWidth - 4
		if maxRunes > len(runes) {
			maxRunes = len(runes)
		}
		if maxRunes > 0 {
			return fmt.Sprintf(" %s… ", string(runes[:maxRunes]))
		}
	}

	// Too small for text: empty block
	return ""
}
