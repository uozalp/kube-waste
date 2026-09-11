package tui

import (
	"strings"
	"testing"

	"github.com/uozalp/kube-waste/internal/model"
)

func TestFitCellTruncatesWithEllipsis(t *testing.T) {
	got := fitCell("t-grasshopper-pam-wallet-application", 28, alignLeft)
	if len([]rune(got)) != 28 {
		t.Fatalf("width = %d runes, want 28", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q, want an ellipsis suffix", got)
	}
}

func TestFitCellAlignment(t *testing.T) {
	if got := fitCell("7.99c", 9, alignRight); got != "    7.99c" {
		t.Errorf("right aligned = %q", got)
	}
	if got := fitCell("hpe-nfs", 9, alignLeft); got != "hpe-nfs  " {
		t.Errorf("left aligned = %q", got)
	}
}

func TestFitColumnsKeepsWasteColumns(t *testing.T) {
	// Only the name and the two waste columns fit.
	idx := fitColumns(namespaceColumns, 50)

	kept := map[string]bool{}
	for _, i := range idx {
		kept[namespaceColumns[i].title] = true
	}
	for _, must := range []string{"NAMESPACE", "WASTE CPU", "WASTE MEM"} {
		if !kept[must] {
			t.Errorf("column %q was dropped at width 50: kept %v", must, kept)
		}
	}
	if kept["PODS"] {
		t.Error("secondary PODS column should be dropped first")
	}
}

func TestFitColumnsKeepsEverythingWhenWide(t *testing.T) {
	if got := len(fitColumns(namespaceColumns, 200)); got != len(namespaceColumns) {
		t.Fatalf("kept %d columns, want %d", got, len(namespaceColumns))
	}
}

func TestColumnWidthsFillTheTerminal(t *testing.T) {
	idx := fitColumns(namespaceColumns, 160)
	widths := columnWidths(namespaceColumns, idx, 160)
	if got := tableWidth(namespaceColumns, idx, widths); got != 160 {
		t.Fatalf("table width = %d, want 160", got)
	}
}

func TestScrollOffsetKeepsCursorVisible(t *testing.T) {
	if got := scrollOffset(0, 25, 10, 100); got != 16 {
		t.Errorf("scrolling down: got %d, want 16", got)
	}
	if got := scrollOffset(20, 5, 10, 100); got != 5 {
		t.Errorf("scrolling up: got %d, want 5", got)
	}
	if got := scrollOffset(95, 99, 10, 100); got != 90 {
		t.Errorf("at the end: got %d, want 90", got)
	}
	if got := scrollOffset(5, 0, 10, 0); got != 0 {
		t.Errorf("empty table: got %d, want 0", got)
	}
}

func TestUsageCellsReportUnavailable(t *testing.T) {
	usedCPU, wasteCPU, usedMem, wasteMem := usageCells(model.Summary{
		Requested: model.Resources{CPUMilli: 8000},
	})
	for _, c := range []cell{usedCPU, wasteCPU, usedMem, wasteMem} {
		if c.text != model.Unavailable {
			t.Fatalf("got %q, want %q", c.text, model.Unavailable)
		}
	}
}

func TestUsageCellsFormatAbsoluteWaste(t *testing.T) {
	_, wasteCPU, _, wasteMem := usageCells(model.Summary{
		Requested:  model.Resources{CPUMilli: 8000, MemBytes: 9 << 30},
		Used:       model.Resources{CPUMilli: 10, MemBytes: 580813128},
		UsageKnown: true,
	})
	if wasteCPU.text != "7.99c" {
		t.Errorf("cpu waste = %q, want 7.99c", wasteCPU.text)
	}
	if wasteMem.text != "8.46G" {
		t.Errorf("memory waste = %q, want 8.46G", wasteMem.text)
	}
}

func TestMarkSortAddsDirectionArrow(t *testing.T) {
	cols := markSort(namespaceColumns, model.SortWasteCPU, true)
	for i, c := range cols {
		if c.sort == model.SortWasteCPU && c.sortable {
			if !strings.HasSuffix(c.title, "↓") {
				t.Fatalf("column %d title = %q, want a ↓ marker", i, c.title)
			}
			if !c.active {
				t.Fatalf("column %d should be marked active", i)
			}
			return
		}
	}
	t.Fatal("waste cpu column not found")
}

func TestRenderTableRendersHeaderAndRows(t *testing.T) {
	rows := []row{
		{cells: []cell{txt("hpe-nfs"), txt("8.00c"), txt("0.01c"), txt("7.99c")}},
		{cells: []cell{txt("payments"), txt("2.00c"), txt("0.25c"), txt("1.75c")}},
	}
	cols := []column{
		{title: "NAMESPACE", width: 12, flex: true},
		{title: "REQ CPU", width: 8, align: alignRight},
		{title: "USED CPU", width: 8, align: alignRight},
		{title: "WASTE CPU", width: 9, align: alignRight},
	}

	out := renderTable(cols, rows, 60, 3, 0, 0)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("rendered %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "WASTE CPU") {
		t.Errorf("header missing: %q", lines[0])
	}
	if !strings.Contains(lines[1], "hpe-nfs") || !strings.Contains(lines[1], "7.99c") {
		t.Errorf("first row missing values: %q", lines[1])
	}
}

func TestRenderTableLimitsToVisibleRows(t *testing.T) {
	rows := make([]row, 50)
	for i := range rows {
		rows[i] = row{cells: []cell{txt("ns")}}
	}
	out := renderTable([]column{{title: "NS", width: 10, flex: true}}, rows, 40, 5, 10, 12)
	if got := len(strings.Split(out, "\n")); got != 5 {
		t.Fatalf("rendered %d lines, want 5", got)
	}
}
