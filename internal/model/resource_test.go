package model

import "testing"

const gi = int64(1) << 30

// gib builds a byte count from a fractional GiB value.
func gib(v float64) int64 { return int64(v * float64(gi)) }

func TestSummaryWasteCPU(t *testing.T) {
	s := Summary{
		Requested:  Resources{CPUMilli: 8000},
		Used:       Resources{CPUMilli: 10},
		UsageKnown: true,
	}
	if got := s.Waste().CPUMilli; got != 7990 {
		t.Fatalf("waste = %d milli, want 7990", got)
	}
	if got := FormatCores(s.Waste().CPUMilli); got != "7.99c" {
		t.Fatalf("formatted waste = %q, want %q", got, "7.99c")
	}
}

func TestSummaryWasteMemory(t *testing.T) {
	s := Summary{
		Requested:  Resources{MemBytes: 9 * gi},
		Used:       Resources{MemBytes: 580813128}, // ~0.54 GiB
		UsageKnown: true,
	}
	if got := FormatMem(s.Requested.MemBytes); got != "9.00G" {
		t.Fatalf("requested = %q, want %q", got, "9.00G")
	}
	if got := FormatMem(s.Used.MemBytes); got != "0.54G" {
		t.Fatalf("used = %q, want %q", got, "0.54G")
	}
	if got := FormatMem(s.Waste().MemBytes); got != "8.46G" {
		t.Fatalf("waste = %q, want %q", got, "8.46G")
	}
}

func TestNegativeWasteIsNotClamped(t *testing.T) {
	requested := gib(108.49)
	used := gib(118.04)
	s := Summary{
		Requested:  Resources{MemBytes: requested, CPUMilli: 1000},
		Used:       Resources{MemBytes: used, CPUMilli: 1500},
		UsageKnown: true,
	}
	waste := s.Waste()
	if waste.MemBytes >= 0 {
		t.Fatalf("memory waste = %d, want negative", waste.MemBytes)
	}
	if got := FormatMem(waste.MemBytes); got != "-9.55G" {
		t.Fatalf("memory waste = %q, want %q", got, "-9.55G")
	}
	if got := FormatCores(waste.CPUMilli); got != "-0.50c" {
		t.Fatalf("cpu waste = %q, want %q", got, "-0.50c")
	}
}

func TestPercent(t *testing.T) {
	if got, ok := Percent(148600, 440000); !ok || FormatPercent(got, ok) != "34%" {
		t.Fatalf("Percent = %v ok=%v, want 34%%", got, ok)
	}
	if _, ok := Percent(100, 0); ok {
		t.Fatal("Percent with zero total should not be ok")
	}
	if got := FormatPercent(0, false); got != Unavailable {
		t.Fatalf("FormatPercent(unknown) = %q, want %q", got, Unavailable)
	}
}

func TestWasteRatioUnknownUsage(t *testing.T) {
	s := Summary{Requested: Resources{CPUMilli: 1000}}
	if _, ok := s.WasteRatio(CPU); ok {
		t.Fatal("waste ratio should be unknown when usage is unknown")
	}
}

func TestSummaryAddPropagatesUnknownUsage(t *testing.T) {
	known := Summary{Requested: Resources{CPUMilli: 100}, Used: Resources{CPUMilli: 10}, UsageKnown: true}
	unknown := Summary{Requested: Resources{CPUMilli: 200}}

	sum := known.Add(unknown)
	if sum.Requested.CPUMilli != 300 {
		t.Fatalf("requested = %d, want 300", sum.Requested.CPUMilli)
	}
	if sum.UsageKnown {
		t.Fatal("usage should be unknown when one operand is unknown")
	}
}

func TestFormatting(t *testing.T) {
	cases := []struct {
		milli int64
		want  string
	}{
		{10, "0.01c"},
		{1230, "1.23c"},
		{8000, "8.00c"},
	}
	for _, c := range cases {
		if got := FormatCores(c.milli); got != c.want {
			t.Errorf("FormatCores(%d) = %q, want %q", c.milli, got, c.want)
		}
	}

	if got := FormatCoresLong(440000); got != "440.0 cores" {
		t.Errorf("FormatCoresLong = %q", got)
	}
	if got := FormatMemLong(gib(1928.4)); got != "1928.4 GiB" {
		t.Errorf("FormatMemLong = %q", got)
	}
}

func TestMaxResources(t *testing.T) {
	a := Resources{CPUMilli: 500, MemBytes: 4 * gi}
	b := Resources{CPUMilli: 1500, MemBytes: 1 * gi}
	got := MaxResources(a, b)
	if got.CPUMilli != 1500 || got.MemBytes != 4*gi {
		t.Fatalf("MaxResources = %+v", got)
	}
}
