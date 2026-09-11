package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/uozalp/kube-waste/internal/model"
)

const (
	logoGap  = 4
	logoRows = 5

	ovLabelW = 11
	ovCPUW   = 17
	ovMemW   = 19
	ovGap    = "  "
)

// logoGlyphs spells "kube-waste", one five-row block per letter.
var logoGlyphs = [][logoRows]string{
	{" _    ", "| | __", "| |/ /", "|   < ", "|_|\\_\\"},
	{"       ", " _   _ ", "| | | |", "| |_| |", " \\__,_|"},
	{" _     ", "| |__  ", "| '_ \\ ", "| |_) |", "|_.__/ "},
	{"       ", "  ___  ", " / _ \\ ", "|  __/ ", " \\___| "},
	{"       ", "       ", " _____ ", "|_____|", "       "},
	{"           ", " __      __", " \\ \\    / /", "  \\ \\/\\/ / ", "   \\_/\\_/  "},
	{"       ", "  __ _ ", " / _` |", "| (_| |", " \\__,_|"},
	{"      ", " ___  ", "/ __| ", "\\__ \\ ", "|___/ "},
	{" _   ", "| |_ ", "| __|", "| |_ ", " \\__|"},
	{"       ", "  ___  ", " / _ \\ ", "|  __/ ", " \\___| "},
}

var logoArt, logoWidth = buildLogo()

func buildLogo() ([]string, int) {
	art := make([]string, logoRows)
	for r := 0; r < logoRows; r++ {
		var b strings.Builder
		for _, g := range logoGlyphs {
			b.WriteString(g[r])
		}
		art[r] = b.String()
	}
	return art, ansi.StringWidth(art[0])
}

// headerBlock puts the cluster summary on the left and the logo on the right.
// The summary starts one row down so it sits centred against the taller logo.
func (m Model) headerBlock() []string {
	left := m.overviewLines()
	if len(left) > 0 {
		indented := make([]string, 0, len(left)+1)
		indented = append(indented, "")
		for _, l := range left {
			indented = append(indented, strings.Repeat(" ", gutter)+l)
		}
		left = indented
	}
	leftW := min(blockWidth(left), m.width)

	right, rightW := m.logoBlock()
	if leftW+logoGap+rightW > m.width {
		right, rightW = nil, 0
	}

	n := max(len(left), len(right))
	if n == 0 {
		return []string{styleTitle.Render("kube-waste")}
	}

	out := make([]string, n)
	for i := range out {
		line := ""
		if i < len(left) {
			line = left[i]
		}
		if rightW == 0 {
			out[i] = fitCell(line, m.width, alignLeft)
			continue
		}
		line = fitCell(line, leftW, alignLeft)
		if i < len(right) {
			out[i] = line + strings.Repeat(" ", m.width-leftW-rightW) + right[i]
			continue
		}
		out[i] = line
	}
	return out
}

func (m Model) logoBlock() ([]string, int) {
	if m.height < 18 {
		return []string{styleTitle.Render("kube-waste")}, 10
	}
	out := make([]string, len(logoArt))
	for i, l := range logoArt {
		out[i] = styleTitle.Render(fitCell(l, logoWidth, alignLeft))
	}
	return out, logoWidth
}

func blockWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

// overviewLines renders the cluster capacity summary as a small table.
func (m Model) overviewLines() []string {
	c := m.cluster
	if c == nil || m.screen == screenContexts {
		return nil
	}

	row := func(label, cpu, mem string) string {
		return styleSection.Render(fitCell(label, ovLabelW, alignLeft)) + ovGap +
			fitCell(cpu, ovCPUW, alignRight) + ovGap +
			fitCell(mem, ovMemW, alignRight)
	}

	reqCPU, okReqCPU := model.Percent(c.Requested.CPUMilli, c.Allocatable.CPUMilli)
	reqMem, okReqMem := model.Percent(c.Requested.MemBytes, c.Allocatable.MemBytes)

	lines := []string{
		styleHeader.Render(fitCell("", ovLabelW, alignLeft) + ovGap +
			fitCell("CPU", ovCPUW, alignRight) + ovGap +
			fitCell("MEMORY", ovMemW, alignRight)),
		row("ALLOCATABLE", model.FormatCoresLong(c.Allocatable.CPUMilli), model.FormatMemLong(c.Allocatable.MemBytes)),
		row("REQUESTED",
			model.FormatCoresLong(c.Requested.CPUMilli)+" ("+model.FormatPercent(reqCPU, okReqCPU)+")",
			model.FormatMemLong(c.Requested.MemBytes)+" ("+model.FormatPercent(reqMem, okReqMem)+")"),
	}

	if c.UsageKnown {
		usedCPU, okUsedCPU := model.Percent(c.Used.CPUMilli, c.Allocatable.CPUMilli)
		usedMem, okUsedMem := model.Percent(c.Used.MemBytes, c.Allocatable.MemBytes)
		lines = append(lines, row("USED",
			model.FormatCoresLong(c.Used.CPUMilli)+" ("+model.FormatPercent(usedCPU, okUsedCPU)+")",
			model.FormatMemLong(c.Used.MemBytes)+" ("+model.FormatPercent(usedMem, okUsedMem)+")"))
	} else {
		lines = append(lines, row("USED", model.Unavailable, model.Unavailable))
	}

	if c.MetricsError != "" {
		lines = append(lines, styleWarn.Render("Metrics unavailable: "+c.MetricsError))
	}
	for _, w := range c.Warnings {
		lines = append(lines, styleWarn.Render(w))
	}
	return lines
}
