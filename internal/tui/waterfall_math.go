package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// BarCoordinates calculates the character columns for the waterfall timing bar.
type BarCoordinates struct {
	TotalWidth int
	StartCol   int // 0-indexed column where the span begins
	SpanWidth  int // Number of columns occupied by the span
	EndCol     int // StartCol + SpanWidth
}

// ComputeBarCoordinates calculates exact start and width columns for a span timing bar.
func ComputeBarCoordinates(totalWidth int, relativeOffset, relativeWidth float64) BarCoordinates {
	if totalWidth <= 0 {
		return BarCoordinates{}
	}

	// Clamp input values between 0.0 and 1.0
	if relativeOffset < 0 {
		relativeOffset = 0
	} else if relativeOffset > 1.0 {
		relativeOffset = 1.0
	}

	if relativeWidth < 0 {
		relativeWidth = 0
	} else if relativeWidth > 1.0 {
		relativeWidth = 1.0
	}

	startCol := int(math.Floor(relativeOffset * float64(totalWidth)))
	if startCol >= totalWidth {
		startCol = totalWidth - 1
	}

	spanWidth := int(math.Round(relativeWidth * float64(totalWidth)))
	// Guarantee visibility for non-zero spans
	if relativeWidth > 0 && spanWidth < 1 {
		spanWidth = 1
	}
	if spanWidth > totalWidth {
		spanWidth = totalWidth
	}
	if startCol+spanWidth > totalWidth {
		spanWidth = totalWidth - startCol
	}
	if spanWidth < 1 {
		spanWidth = 1
	}

	return BarCoordinates{
		TotalWidth: totalWidth,
		StartCol:   startCol,
		SpanWidth:  spanWidth,
		EndCol:     startCol + spanWidth,
	}
}

// RenderTimingBar produces a fixed-width string visualization of a span's timing within a trace.
// Example: "     █████████░░░░░░"
func RenderTimingBar(totalWidth int, relativeOffset, relativeWidth float64, fillChar, bgChar rune) string {
	if totalWidth <= 0 {
		return ""
	}
	coords := ComputeBarCoordinates(totalWidth, relativeOffset, relativeWidth)

	var sb strings.Builder
	sb.Grow(totalWidth * 4) // rune safety

	// Leading background
	for i := 0; i < coords.StartCol; i++ {
		sb.WriteRune(bgChar)
	}
	// Active span
	for i := 0; i < coords.SpanWidth; i++ {
		sb.WriteRune(fillChar)
	}
	// Trailing background
	trailing := coords.TotalWidth - coords.EndCol
	for i := 0; i < trailing; i++ {
		sb.WriteRune(bgChar)
	}

	return sb.String()
}

// FormatDuration formats a duration concisely for terminal columns (e.g. 45µs, 12.4ms, 1.25s).
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Microseconds())+(float64(d.Nanoseconds()%1000)/1000.0))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Milliseconds())+(float64(d%time.Millisecond)/float64(time.Millisecond)))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// PadRight pads string s with trailing spaces until its terminal display width equals targetWidth.
// Correctly accounts for multi-byte Unicode characters and ANSI styling.
func PadRight(s string, targetWidth int) string {
	w := lipgloss.Width(s)
	if w >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-w)
}

// PadLeft pads string s with leading spaces until its terminal display width equals targetWidth.
// Correctly accounts for multi-byte Unicode characters and ANSI styling.
func PadLeft(s string, targetWidth int) string {
	w := lipgloss.Width(s)
	if w >= targetWidth {
		return s
	}
	return strings.Repeat(" ", targetWidth-w) + s
}
