package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	borderH  = "─"
	borderV  = "│"
	borderTL = "╭"
	borderTR = "╮"
	borderBL = "╰"
	borderBR = "╯"

	boxPad = 1
)

// boxInner is the content width left inside a box of the given outer width.
func boxInner(width int) int { return width - 2 - 2*boxPad }

// boxed frames content in a rounded border with the title centred in the top
// edge. width and height include the border.
func boxed(title, content string, width, height int) string {
	if boxInner(width) < 1 || height < 3 {
		return content
	}
	inner := width - 2
	pad := strings.Repeat(" ", boxPad)

	var b strings.Builder
	b.WriteString(styleBorder.Render(borderTL))
	b.WriteString(topEdge(title, inner))
	b.WriteString(styleBorder.Render(borderTR))

	lines := strings.Split(content, "\n")
	for n := 0; n < height-2; n++ {
		line := ""
		if n < len(lines) {
			line = lines[n]
		}
		b.WriteString("\n" + styleBorder.Render(borderV) + pad +
			fitCell(line, boxInner(width), alignLeft) + pad + styleBorder.Render(borderV))
	}

	b.WriteString("\n" + styleBorder.Render(borderBL+strings.Repeat(borderH, inner)+borderBR))
	return b.String()
}

func topEdge(title string, inner int) string {
	label := " " + title + " "
	w := ansi.StringWidth(label)
	if title == "" || w > inner {
		return hline(inner)
	}
	left := (inner - w) / 2
	return hline(left) + label + hline(inner-w-left)
}

func hline(n int) string {
	if n <= 0 {
		return ""
	}
	return styleBorder.Render(strings.Repeat(borderH, n))
}

// boxTitle formats a k9s-style table title: kind(scope)[count] </filter>.
func boxTitle(kind, scope, filter string, count int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(kind))
	if scope != "" {
		b.WriteString(styleDim.Render("(") + styleKey.Render(scope) + styleDim.Render(")"))
	}
	b.WriteString(styleDim.Render("[") + styleCount.Render(strconv.Itoa(count)) + styleDim.Render("]"))
	if filter != "" {
		b.WriteString(" " + styleFilterTag.Render("</"+filter+">"))
	}
	return b.String()
}
