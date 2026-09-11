// Package model contains the application specific data types used by the TUI.
// It deliberately has no dependency on the Kubernetes API types.
package model

import (
	"fmt"
	"math"
)

// Resources holds a CPU amount in milli-cores and a memory amount in bytes.
// Values are kept numeric everywhere and only formatted at the UI layer.
type Resources struct {
	CPUMilli int64
	MemBytes int64
}

// Add returns the element-wise sum of r and o.
func (r Resources) Add(o Resources) Resources {
	return Resources{CPUMilli: r.CPUMilli + o.CPUMilli, MemBytes: r.MemBytes + o.MemBytes}
}

// Sub returns the element-wise difference of r and o.
func (r Resources) Sub(o Resources) Resources {
	return Resources{CPUMilli: r.CPUMilli - o.CPUMilli, MemBytes: r.MemBytes - o.MemBytes}
}

// IsZero reports whether both CPU and memory are zero.
func (r Resources) IsZero() bool { return r.CPUMilli == 0 && r.MemBytes == 0 }

// MaxResources returns the element-wise maximum of a and b.
func MaxResources(a, b Resources) Resources {
	return Resources{
		CPUMilli: max(a.CPUMilli, b.CPUMilli),
		MemBytes: max(a.MemBytes, b.MemBytes),
	}
}

// Summary is the core resource model: what was requested, what is actually
// used, and the difference between the two. Waste may be negative when actual
// usage exceeds the requests.
type Summary struct {
	Requested Resources
	Used      Resources
	// UsageKnown is false when metrics.k8s.io could not be queried. In that
	// case Used and Waste must be rendered as unavailable rather than zero.
	UsageKnown bool
}

// Waste returns requested minus used. Negative values are not clamped.
func (s Summary) Waste() Resources { return s.Requested.Sub(s.Used) }

// Add merges o into s. Usage is only known when it is known for both operands.
func (s Summary) Add(o Summary) Summary {
	return Summary{
		Requested:  s.Requested.Add(o.Requested),
		Used:       s.Used.Add(o.Used),
		UsageKnown: s.UsageKnown && o.UsageKnown,
	}
}

// Percent returns 100*part/whole. ok is false when whole is zero.
func Percent(part, whole int64) (value float64, ok bool) {
	if whole == 0 {
		return 0, false
	}
	return float64(part) / float64(whole) * 100, true
}

// WasteRatio returns the fraction of the request that is wasted, in percent.
// ok is false when nothing was requested.
func (s Summary) WasteRatio(kind ResourceKind) (value float64, ok bool) {
	if !s.UsageKnown {
		return 0, false
	}
	switch kind {
	case CPU:
		return Percent(s.Waste().CPUMilli, s.Requested.CPUMilli)
	default:
		return Percent(s.Waste().MemBytes, s.Requested.MemBytes)
	}
}

// ResourceKind distinguishes CPU from memory in generic helpers.
type ResourceKind int

const (
	CPU ResourceKind = iota
	Memory
)

const (
	gibibyte = 1 << 30
	// Unavailable is rendered wherever usage data is missing.
	Unavailable = "N/A"
)

// FormatCores renders milli-cores as e.g. "8.00c".
func FormatCores(milli int64) string {
	return fmt.Sprintf("%.2fc", float64(milli)/1000)
}

// FormatMem renders bytes as gibibytes, e.g. "108.88G".
func FormatMem(bytes int64) string {
	return fmt.Sprintf("%.2fG", float64(bytes)/gibibyte)
}

// FormatCoresLong renders milli-cores for the overview, e.g. "440.0 cores".
func FormatCoresLong(milli int64) string {
	return fmt.Sprintf("%.1f cores", float64(milli)/1000)
}

// FormatMemLong renders bytes for the overview, e.g. "1928.4 GiB".
func FormatMemLong(bytes int64) string {
	return fmt.Sprintf("%.1f GiB", float64(bytes)/gibibyte)
}

// FormatPercent renders a percentage as e.g. "34%". Values are rounded.
func FormatPercent(value float64, ok bool) string {
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return Unavailable
	}
	return fmt.Sprintf("%.0f%%", value)
}
