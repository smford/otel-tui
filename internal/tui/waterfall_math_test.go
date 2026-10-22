package tui

import (
	"testing"
	"time"
	"unicode/utf8"
)

func TestComputeBarCoordinates(t *testing.T) {
	tests := []struct {
		name           string
		totalWidth     int
		offset         float64
		width          float64
		expectedStart  int
		expectedWidth  int
	}{
		{"full span", 20, 0.0, 1.0, 0, 20},
		{"half offset, half width", 20, 0.5, 0.5, 10, 10},
		{"quarter offset, quarter width", 40, 0.25, 0.25, 10, 10},
		{"tiny duration clamped to 1 char", 30, 0.5, 0.001, 15, 1},
		{"end clamped to width", 20, 0.9, 0.5, 18, 2},
		{"zero width handled", 0, 0.0, 1.0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			coords := ComputeBarCoordinates(tc.totalWidth, tc.offset, tc.width)
			if tc.totalWidth == 0 {
				if coords.TotalWidth != 0 {
					t.Errorf("expected 0 coords for 0 width, got %+v", coords)
				}
				return
			}
			if coords.StartCol != tc.expectedStart {
				t.Errorf("expected start %d, got %d", tc.expectedStart, coords.StartCol)
			}
			if coords.SpanWidth != tc.expectedWidth {
				t.Errorf("expected width %d, got %d", tc.expectedWidth, coords.SpanWidth)
			}
			if coords.EndCol > tc.totalWidth {
				t.Errorf("end col %d exceeds total width %d", coords.EndCol, tc.totalWidth)
			}
		})
	}
}

func TestRenderTimingBar(t *testing.T) {
	widths := []int{10, 20, 35, 50}
	offsets := []float64{0.0, 0.1, 0.33, 0.7, 0.99}
	durations := []float64{0.005, 0.1, 0.25, 0.5, 1.0}

	for _, w := range widths {
		for _, off := range offsets {
			for _, dur := range durations {
				rendered := RenderTimingBar(w, off, dur, '█', '░')
				charCount := utf8.RuneCountInString(rendered)
				if charCount != w {
					t.Fatalf("expected rendered rune count %d, got %d (rendered: %q)", w, charCount, rendered)
				}
			}
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{500 * time.Nanosecond, "500ns"},
		{1500 * time.Nanosecond, "1.5µs"},
		{12500 * time.Microsecond, "12.5ms"},
		{1250 * time.Millisecond, "1.25s"},
	}

	for _, tc := range tests {
		got := FormatDuration(tc.d)
		if got != tc.expected {
			t.Errorf("duration %v: expected %s, got %s", tc.d, tc.expected, got)
		}
	}
}
