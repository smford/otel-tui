package tui

import (
	"fmt"
	"strings"

	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// RenderTraceDiffView renders a side-by-side comparative diff of two traces.
func RenderTraceDiffView(diff *store.TraceDiff, width, height int) string {
	if diff == nil {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("  Select two traces to compare.")
	}

	var lines []string

	// Header summary
	deltaDurStr := FormatDuration(diff.DeltaDuration)
	if diff.DeltaDuration > 0 {
		deltaDurStr = "+" + deltaDurStr
	}
	pctStr := fmt.Sprintf("%+.1f%%", diff.DeltaPercent)

	var durDeltaBadge string
	if diff.DeltaDuration > 0 {
		durDeltaBadge = lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render(fmt.Sprintf("%s (%s SLOWER)", deltaDurStr, pctStr))
	} else if diff.DeltaDuration < 0 {
		durDeltaBadge = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(fmt.Sprintf("%s (%s FASTER)", deltaDurStr, pctStr))
	} else {
		durDeltaBadge = lipgloss.NewStyle().Foreground(ColorWhite).Render("0ms (IDENTICAL)")
	}

	statusDeltaStr := ""
	if diff.StatusChanged {
		statusDeltaStr = lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render(" [STATUS CHANGED]")
	}

	title := fmt.Sprintf("TRACE DIFF: %s vs %s", diff.BaseTraceID[:8], diff.TargetTraceID[:8])
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("◆ "+title))
	lines = append(lines, fmt.Sprintf("  Duration: %s ➔ %s  │  Delta: %s%s",
		FormatDuration(diff.BaseDuration), FormatDuration(diff.TargetDuration), durDeltaBadge, statusDeltaStr))
	lines = append(lines, fmt.Sprintf("  Span Count: %d ➔ %d (%+d spans)",
		diff.BaseSpanCount, diff.TargetSpanCount, diff.TargetSpanCount-diff.BaseSpanCount))
	lines = append(lines, "")

	// Table Header
	headerCols := fmt.Sprintf("  %s %s %s %s %s %s %s",
		PadRight("DIFF", 4),
		PadRight("SERVICE", 18),
		PadRight("OPERATION", 28),
		PadRight("BASE DUR", 12),
		PadRight("TARGET DUR", 12),
		PadRight("DELTA (ms / %)", 18),
		PadRight("STATUS", 12))
	lines = append(lines, TableHeaderStyle.Width(width-4).Render(headerCols))

	for _, item := range diff.SpanDiffs {
		var diffIcon string
		var rowStyle lipgloss.Style

		deltaStr := FormatDuration(item.DeltaDuration)
		if item.DeltaDuration > 0 {
			deltaStr = "+" + deltaStr
		}
		pct := fmt.Sprintf("%+.1f%%", item.DeltaPercent)
		deltaCombined := fmt.Sprintf("%-10s %7s", deltaStr, pct)

		switch item.ChangeType {
		case "ADDED":
			diffIcon = "+ "
			rowStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
			deltaCombined = fmt.Sprintf("%-10s %7s", "+"+FormatDuration(item.TargetDuration), "[NEW]")
		case "REMOVED":
			diffIcon = "- "
			rowStyle = lipgloss.NewStyle().Foreground(ColorMuted)
			deltaCombined = fmt.Sprintf("%-10s %7s", "-"+FormatDuration(item.BaseDuration), "[GONE]")
		default:
			if item.DeltaDuration > 10*1000*1000 || item.StatusChanged { // >10ms slower
				diffIcon = "▲ "
				rowStyle = lipgloss.NewStyle().Foreground(ColorError)
			} else if item.DeltaDuration < -10*1000*1000 { // >10ms faster
				diffIcon = "▼ "
				rowStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
			} else {
				diffIcon = "  "
				rowStyle = lipgloss.NewStyle().Foreground(ColorWhite)
			}
		}

		svcText := item.ServiceName
		if len(svcText) > 16 {
			svcText = svcText[:14] + ".."
		}
		opText := item.OperationName
		if len(opText) > 26 {
			opText = opText[:24] + ".."
		}

		baseDurText := "-"
		if item.BaseDuration > 0 {
			baseDurText = FormatDuration(item.BaseDuration)
		}
		targetDurText := "-"
		if item.TargetDuration > 0 {
			targetDurText = FormatDuration(item.TargetDuration)
		}

		statusText := item.TargetStatus
		if item.StatusChanged {
			statusText = fmt.Sprintf("%s ➔ %s", item.BaseStatus, item.TargetStatus)
		}

		diffCell := PadRight(diffIcon, 4)
		svcCell := PadRight(svcText, 18)
		opCell := PadRight(opText, 28)
		baseDurCell := PadRight(baseDurText, 12)
		targetDurCell := PadRight(targetDurText, 12)
		deltaCell := PadRight(deltaCombined, 18)
		statusCell := PadRight(statusText, 12)

		rowStr := fmt.Sprintf("  %s %s %s %s %s %s %s",
			diffCell, svcCell, opCell, baseDurCell, targetDurCell, deltaCell, statusCell)

		lines = append(lines, rowStyle.Render(rowStr))
	}

	return strings.Join(lines, "\n")
}
