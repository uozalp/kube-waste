package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/uozalp/kube-waste/internal/model"
)

const (
	colGap     = 2
	minFlexCol = 8
)

type alignment int

const (
	alignLeft alignment = iota
	alignRight
	alignCenter
)

// column describes one table column. Columns with a higher priority number are
// hidden first when the terminal is too narrow; priority 0 is never hidden.
type column struct {
	title    string
	width    int
	align    alignment
	flex     bool
	priority int

	sortable bool
	sort     model.SortKey
	active   bool
}

// markSort appends a direction arrow to the active sort column and flags it so
// the header can be highlighted.
func markSort(cols []column, key model.SortKey, desc bool) []column {
	out := make([]column, len(cols))
	copy(out, cols)
	for i := range out {
		if out[i].sortable && out[i].sort == key {
			out[i].title += " " + arrow(desc)
			out[i].width += 2
			out[i].active = true
		}
	}
	return out
}

type cell struct {
	text  string
	style lipgloss.Style
}

type row struct {
	cells []cell
}

func txt(s string) cell { return cell{text: s, style: styleNormal} }

func styled(s string, st lipgloss.Style) cell { return cell{text: s, style: st} }

func tableWidth(cols []column, idx []int, widths []int) int {
	if len(idx) == 0 {
		return 0
	}
	total := colGap * (len(idx) - 1)
	for p, i := range idx {
		if widths != nil {
			total += widths[p]
			continue
		}
		total += cols[i].width
	}
	return total
}

// fitColumns returns the indices of the columns that fit into width.
func fitColumns(cols []column, width int) []int {
	idx := make([]int, len(cols))
	for i := range cols {
		idx[i] = i
	}
	for len(idx) > 1 && tableWidth(cols, idx, nil) > width {
		drop, prio := -1, 0
		for pos, i := range idx {
			if cols[i].priority > prio {
				drop, prio = pos, cols[i].priority
			}
		}
		if drop < 0 {
			break
		}
		idx = append(idx[:drop:drop], idx[drop+1:]...)
	}
	return idx
}

// columnWidths distributes the available width, giving any slack to the
// flexible columns and shrinking them first when space is tight.
func columnWidths(cols []column, idx []int, width int) []int {
	widths := make([]int, len(idx))
	var flex []int
	for p, i := range idx {
		widths[p] = cols[i].width
		if cols[i].flex {
			flex = append(flex, p)
		}
	}
	slack := width - tableWidth(cols, idx, widths)
	if len(flex) == 0 {
		return widths
	}

	if slack > 0 {
		per := slack / len(flex)
		for n, p := range flex {
			if n == len(flex)-1 {
				widths[p] += slack - per*(len(flex)-1)
				continue
			}
			widths[p] += per
		}
		return widths
	}

	for slack < 0 {
		shrank := false
		for _, p := range flex {
			if slack == 0 {
				break
			}
			if widths[p] > minFlexCol {
				widths[p]--
				slack++
				shrank = true
			}
		}
		if !shrank {
			break
		}
	}
	return widths
}

func fitCell(text string, width int, align alignment) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) > width {
		text = ansi.Truncate(text, width, "…")
	}
	pad := width - ansi.StringWidth(text)
	if pad <= 0 {
		return text
	}
	switch align {
	case alignRight:
		return strings.Repeat(" ", pad) + text
	case alignCenter:
		left := pad / 2
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", pad-left)
	default:
		return text + strings.Repeat(" ", pad)
	}
}

// renderTable draws the header and the visible slice of rows. selected is an
// index into rows; pass -1 for no selection.
func renderTable(cols []column, rows []row, width, height, offset, selected int) string {
	if height <= 0 || width <= 0 {
		return ""
	}
	idx := fitColumns(cols, width)
	widths := columnWidths(cols, idx, width)

	var b strings.Builder
	header := make([]string, 0, len(idx))
	for p, i := range idx {
		text := fitCell(cols[i].title, widths[p], cols[i].align)
		if cols[i].active {
			header = append(header, styleSortHeader.Render(text))
			continue
		}
		header = append(header, styleHeader.Render(text))
	}
	b.WriteString(strings.Join(header, strings.Repeat(" ", colGap)))

	body := height - 1
	for n := 0; n < body; n++ {
		r := offset + n
		if r >= len(rows) {
			break
		}
		b.WriteString("\n")
		b.WriteString(renderRow(cols, idx, widths, rows[r], r == selected))
	}
	return b.String()
}

func renderRow(cols []column, idx, widths []int, r row, selected bool) string {
	parts := make([]string, 0, len(idx))
	for p, i := range idx {
		var c cell
		if i < len(r.cells) {
			c = r.cells[i]
		}
		text := fitCell(c.text, widths[p], cols[i].align)
		if selected {
			// A single background over the whole row reads better than mixing
			// per-cell colours with the highlight.
			parts = append(parts, text)
			continue
		}
		parts = append(parts, c.style.Render(text))
	}
	line := strings.Join(parts, strings.Repeat(" ", colGap))
	if selected {
		return styleSelected.Render(line)
	}
	return line
}

// scrollOffset keeps the cursor inside the visible window.
func scrollOffset(offset, cursor, visible, total int) int {
	if visible <= 0 || total == 0 {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	if offset > total-visible {
		offset = total - visible
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}
