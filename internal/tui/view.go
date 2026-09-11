package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/uozalp/kube-waste/internal/model"
)

// gutter insets the chrome one column from the box border.
const gutter = 1

// chromeLines counts the refresh line above the body and the status line below.
const chromeLines = 2

// View renders the whole screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "starting…"
	}

	var b strings.Builder
	b.WriteString(strings.Join(m.headerBlock(), "\n"))
	b.WriteString("\n")
	b.WriteString(m.renderUpdated())
	b.WriteString("\n")

	body := m.renderBody()
	lines := strings.Split(body, "\n")
	if n := m.bodyHeight(); len(lines) > n {
		lines = lines[:n]
	} else {
		for len(lines) < n {
			lines = append(lines, "")
		}
	}
	b.WriteString(strings.Join(lines, "\n"))

	b.WriteString("\n")
	b.WriteString(m.renderStatus())
	return b.String()
}

func (m Model) bodyHeight() int {
	return max(1, m.height-len(m.headerBlock())-chromeLines)
}

// renderUpdated fills the separator line under the logo with the refresh time.
func (m Model) renderUpdated() string {
	text := m.spinner()
	if text == "" && m.cluster != nil && m.screen != screenContexts {
		text = "updated " + m.cluster.FetchedAt.Format("15:04:05")
	}
	if text == "" {
		return ""
	}
	return styleDim.Render(fitCell(text, m.width-gutter, alignRight))
}

// renderStatus draws the breadcrumb on the left and a discreet hint on the
// right, both inset one column from the box border above.
func (m Model) renderStatus() string {
	if m.filtering {
		return strings.Repeat(" ", gutter) + styleKey.Render("/") + m.state().filter + "▊" +
			styleDim.Render("   enter: apply   esc: clear")
	}

	if m.errMsg != "" {
		return styleError.Render(fitCell(strings.Repeat(" ", gutter)+"error: "+m.errMsg, m.width, alignLeft))
	}

	left := strings.Repeat(" ", gutter) + stylePath.Render(m.breadcrumb())

	hint := ""
	if n := m.rowCount(); n > 0 {
		hint = styleDim.Render(fmt.Sprintf("%d/%d   ", min(m.state().cursor+1, n), n))
	}
	right := hint + styleKey.Render("?") + styleDim.Render(" help")

	pad := m.width - gutter - ansi.StringWidth(left) - ansi.StringWidth(right)
	if pad < 1 {
		return fitCell(left, m.width, alignLeft)
	}
	return left + strings.Repeat(" ", pad) + right
}

func (m Model) breadcrumb() string {
	if m.screen == screenContexts {
		return ""
	}
	parts := []string{"Contexts", m.contextName, "Namespaces"}
	if m.screen == screenPods {
		parts = append(parts, m.namespace, "Pods")
	}
	return strings.Join(parts, " > ")
}

// loadingBlock centres the spinner and its message in the body area.
func (m Model) loadingBlock(msg string) string {
	line := styleKey.Render(spinnerFrames[m.spinnerIndex]) + " " + styleDim.Render(msg)
	top := max(0, (m.bodyHeight()-1)/2)
	return strings.Repeat("\n", top) + fitCell(line, m.width, alignCenter)
}

func (m Model) renderBody() string {
	if m.showHelp {
		return helpOverlay(m.width)
	}
	switch m.screen {
	case screenContexts:
		return m.renderContexts()
	case screenPods:
		return m.renderPods()
	default:
		return m.renderCluster()
	}
}

func arrow(desc bool) string {
	if desc {
		return "↓"
	}
	return "↑"
}

// tableHeight is the number of lines available to the main table, including
// its header row but excluding the surrounding border.
func (m Model) tableHeight() int {
	switch m.screen {
	case screenContexts, screenPods:
		return max(2, m.bodyHeight()-2) // border
	default:
		_, h := m.clusterLayout()
		return h
	}
}

// usageCells renders the used/waste values of a summary, falling back to N/A
// when metrics are unavailable.
func usageCells(s model.Summary) (usedCPU, wasteCPU, usedMem, wasteMem cell) {
	if !s.UsageKnown {
		na := styled(model.Unavailable, styleDim)
		return na, na, na, na
	}
	waste := s.Waste()
	return txt(model.FormatCores(s.Used.CPUMilli)),
		styled(model.FormatCores(waste.CPUMilli), wasteStyle(waste.CPUMilli, s.Requested.CPUMilli, true)),
		txt(model.FormatMem(s.Used.MemBytes)),
		styled(model.FormatMem(waste.MemBytes), wasteStyle(waste.MemBytes, s.Requested.MemBytes, true))
}
