package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var helpEntries = [][2]string{
	{"↑ / k", "Move up"},
	{"↓ / j", "Move down"},
	{"← / →", "Previous / next sort column"},
	{"pgup / pgdn", "Page up / down"},
	{"g / G", "First / last row"},
	{"enter", "Open selected item"},
	{"esc", "Go back"},
	{"r", "Refresh"},
	{"/", "Filter"},
	{"s / S", "Next sort column / reverse order"},
	{"c", "Sort by wasted CPU"},
	{"m", "Sort by wasted memory"},
	{"n", "Toggle node groups"},
	{"?", "Help"},
	{"q", "Quit"},
}

func helpOverlay(width int) string {
	var b strings.Builder
	b.WriteString(styleSection.Render("Navigation"))
	b.WriteString("\n\n")
	for _, e := range helpEntries {
		b.WriteString("  " + styleKey.Render(fitCell(e[0], 12, alignLeft)) + e[1] + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("kube-waste is read-only; it never modifies Kubernetes resources."))

	box := styleOverlay.Render(b.String())
	if lipgloss.Width(box) > width {
		return b.String()
	}
	return box
}
