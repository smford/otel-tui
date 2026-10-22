package tui

import (
	"fmt"
	"sort"
	"strings"

	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// RenderSpanInspector renders the detailed attributes, events, and correlated logs of a span.
func RenderSpanInspector(span *store.Span, width, height, scrollOffset int) string {
	if span == nil {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("No span selected.")
	}

	var lines []string

	secTitle := func(title string) string {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			Render("▶ " + strings.ToUpper(title))
	}

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorCyan)
	valStyle := lipgloss.NewStyle().Foreground(ColorWhite)
	dimStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	formatKV := func(k, v string) string {
		return fmt.Sprintf("  %s: %s", keyStyle.Render(k), valStyle.Render(v))
	}

	// 1. Overview Section
	lines = append(lines, secTitle("Span Overview"))
	lines = append(lines, formatKV("Span ID", span.SpanID))
	lines = append(lines, formatKV("Trace ID", span.TraceID))
	if span.ParentSpanID != "" {
		lines = append(lines, formatKV("Parent ID", span.ParentSpanID))
	} else {
		lines = append(lines, formatKV("Parent ID", dimStyle.Render("<root span>")))
	}
	lines = append(lines, formatKV("Operation", span.Name))
	lines = append(lines, formatKV("Service", span.ServiceName))
	lines = append(lines, formatKV("Kind", span.Kind))
	lines = append(lines, formatKV("Duration", FormatDuration(span.Duration)))
	statusStr := span.StatusCode
	if span.StatusCode == "ERROR" {
		statusStr = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render("ERROR")
		if span.StatusMessage != "" {
			statusStr += " (" + span.StatusMessage + ")"
		}
	} else if span.StatusCode == "OK" {
		statusStr = lipgloss.NewStyle().Foreground(ColorSecondary).Render("OK")
	}
	lines = append(lines, formatKV("Status", statusStr))
	if span.IsNPlusOne {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("  ⚠ Suspected N+1 Query: Repetitive DB statement detected in trace"))
	}
	lines = append(lines, "")

	// 2. Span Attributes
	lines = append(lines, secTitle(fmt.Sprintf("Span Attributes (%d)", len(span.Attributes))))
	if len(span.Attributes) == 0 {
		lines = append(lines, dimStyle.Render("  <none>"))
	} else {
		var keys []string
		for k := range span.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, formatKV(k, span.Attributes[k]))
		}
	}
	lines = append(lines, "")

	// 3. Resource Attributes
	lines = append(lines, secTitle(fmt.Sprintf("Resource Attributes (%d)", len(span.ResourceAttributes))))
	if len(span.ResourceAttributes) == 0 {
		lines = append(lines, dimStyle.Render("  <none>"))
	} else {
		var keys []string
		for k := range span.ResourceAttributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, formatKV(k, span.ResourceAttributes[k]))
		}
	}
	lines = append(lines, "")

	// 4. Events & Exceptions
	lines = append(lines, secTitle(fmt.Sprintf("Events / Exceptions (%d)", len(span.Events))))
	if len(span.Events) == 0 {
		lines = append(lines, dimStyle.Render("  <none>"))
	} else {
		for i, ev := range span.Events {
			timeStr := ev.Timestamp.Format("15:04:05.000")
			evHeader := fmt.Sprintf("  [%d] %s @ %s", i+1, lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(ev.Name), dimStyle.Render(timeStr))
			lines = append(lines, evHeader)
			for k, v := range ev.Attributes {
				if strings.Contains(v, "\n") {
					lines = append(lines, fmt.Sprintf("      %s:", keyStyle.Render(k)))
					for _, subLine := range strings.Split(v, "\n") {
						lines = append(lines, fmt.Sprintf("        %s", dimStyle.Render(subLine)))
					}
				} else {
					lines = append(lines, fmt.Sprintf("      %s: %s", keyStyle.Render(k), valStyle.Render(v)))
				}
			}
		}
	}
	lines = append(lines, "")

	// 5. Correlated Logs
	lines = append(lines, secTitle(fmt.Sprintf("Correlated Logs (%d)", len(span.Logs))))
	if len(span.Logs) == 0 {
		lines = append(lines, dimStyle.Render("  <no logs for this span>"))
	} else {
		for _, log := range span.Logs {
			timeStr := log.Timestamp.Format("15:04:05.000")
			var sevBadge string
			if strings.EqualFold(log.Severity, "error") || strings.Contains(strings.ToLower(log.Severity), "err") {
				sevBadge = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Background(ColorError).Render(" " + log.Severity + " ")
			} else if strings.EqualFold(log.Severity, "warn") || strings.Contains(strings.ToLower(log.Severity), "warn") {
				sevBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000000")).Background(ColorWarning).Render(" " + log.Severity + " ")
			} else {
				sevBadge = lipgloss.NewStyle().Foreground(ColorCyan).Render("[" + log.Severity + "]")
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s", dimStyle.Render(timeStr), sevBadge, valStyle.Render(log.Body)))
			for k, v := range log.Attributes {
				lines = append(lines, fmt.Sprintf("      %s: %s", dimStyle.Render(k), dimStyle.Render(v)))
			}
		}
	}

	// Apply scrolling
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset >= len(lines) && len(lines) > 0 {
		scrollOffset = len(lines) - 1
	}

	endIdx := scrollOffset + height
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	visibleLines := lines[scrollOffset:endIdx]
	return strings.Join(visibleLines, "\n")
}
