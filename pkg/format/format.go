package format

import (
	"fmt"
	"time"
)

const (
	zeroPercent  = "0%"
	zeroLatency  = "0ms"
	neverChecked = "never"
)

func Bytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

// Duration formats duration in a readable way
func Duration(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func EndpointsUp(healthy, total int) string {
	// Single-rune fast path is only valid for single digits (0-9): '0'+10 is
	// rune(58), which is ':', not "10". The guard must be < 10, not <= 10 -
	// a fleet with exactly 10 endpoints previously rendered "9/:".
	if total < 10 && healthy < 10 {
		return string(rune('0'+healthy)) + "/" + string(rune('0'+total))
	}
	return fmt.Sprintf("%d/%d", healthy, total)
}

func Percentage(value float64) string {
	if value == 0 {
		return zeroPercent
	}
	if value == 100.0 {
		return "100%"
	}
	return fmt.Sprintf("%.1f%%", value)
}

func Latency(ms int64) string {
	if ms == 0 {
		return zeroLatency
	}
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	if ms < 10 {
		return string(rune('0'+ms)) + "ms"
	}
	return fmt.Sprintf("%dms", ms)
}

func Duration2(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
