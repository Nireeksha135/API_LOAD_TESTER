// Package utils provides small, dependency-free formatting helpers
// shared by the reporter and CLI layers: human-readable byte sizes,
// human-readable durations, and a simple ASCII/Unicode progress bar.
package utils

import (
	"fmt"
	"strings"
	"time"
)

// FormatBytes renders n bytes as a human-readable string using
// binary (1024-based) units, e.g. 1536 -> "1.50 KB".
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := int64(unit), 0
	for n/div >= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	return fmt.Sprintf("%.2f %s", float64(n)/float64(div), units[exp])
}

// FormatDuration renders a time.Duration using the most readable
// unit for its magnitude, always with two decimal places of
// precision (except for sub-microsecond durations, rendered as whole
// nanoseconds).
func FormatDuration(d time.Duration) string {
	switch {
	case d < 0:
		return "0ns"
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// FormatRate renders a per-second rate, e.g. 42.567 -> "42.57/s".
func FormatRate(perSecond float64) string {
	return fmt.Sprintf("%.2f/s", perSecond)
}

// ProgressBar renders a fixed-width Unicode progress bar for the
// given completion fraction (clamped to [0, 1]).
//
//	ProgressBar(0.5, 10) -> "[█████░░░░░]"
func ProgressBar(fraction float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}

	filled := int(fraction*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}
