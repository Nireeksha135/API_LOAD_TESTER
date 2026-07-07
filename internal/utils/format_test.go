package utils

import (
	"strings"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"under kb", 512, "512 B"},
		{"exactly one kb", 1024, "1.00 KB"},
		{"megabytes", 5 * 1024 * 1024, "5.00 MB"},
		{"gigabytes", 2 * 1024 * 1024 * 1024, "2.00 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBytes(tt.input); got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"nanoseconds", 500 * time.Nanosecond, "500ns"},
		{"microseconds", 250 * time.Microsecond, "250.00µs"},
		{"milliseconds", 15 * time.Millisecond, "15.00ms"},
		{"seconds", 2500 * time.Millisecond, "2.50s"},
		{"negative", -5 * time.Millisecond, "0ns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDuration(tt.input); got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatRate(t *testing.T) {
	if got := FormatRate(42.567); got != "42.57/s" {
		t.Errorf("FormatRate(42.567) = %q, want %q", got, "42.57/s")
	}
	if got := FormatRate(0); got != "0.00/s" {
		t.Errorf("FormatRate(0) = %q, want %q", got, "0.00/s")
	}
}

func TestProgressBar(t *testing.T) {
	bar := ProgressBar(0.5, 10)
	if !strings.HasPrefix(bar, "[") || !strings.HasSuffix(bar, "]") {
		t.Errorf("ProgressBar() = %q, want brackets", bar)
	}
	filledCount := strings.Count(bar, "█")
	if filledCount != 5 {
		t.Errorf("ProgressBar(0.5, 10) filled count = %d, want 5", filledCount)
	}

	full := ProgressBar(1.0, 8)
	if strings.Count(full, "█") != 8 {
		t.Errorf("ProgressBar(1.0, 8) should be fully filled, got %q", full)
	}

	empty := ProgressBar(0.0, 8)
	if strings.Count(empty, "█") != 0 {
		t.Errorf("ProgressBar(0.0, 8) should be empty, got %q", empty)
	}

	clampedHigh := ProgressBar(1.5, 8)
	if strings.Count(clampedHigh, "█") != 8 {
		t.Errorf("ProgressBar(1.5, 8) should clamp to full, got %q", clampedHigh)
	}

	clampedLow := ProgressBar(-1.0, 8)
	if strings.Count(clampedLow, "█") != 0 {
		t.Errorf("ProgressBar(-1.0, 8) should clamp to empty, got %q", clampedLow)
	}
}
