package tui

import (
	"fmt"

	"github.com/uozalp/kube-waste/internal/model"
)

var nodeGroupColumns = []column{
	{title: "GROUP", width: 16, flex: true},
	{title: "NODES", width: 5, align: alignRight, priority: 4},
	{title: "TAINTED", width: 7, align: alignRight, priority: 6},
	{title: "ALLOC CPU", width: 9, align: alignRight, priority: 2},
	{title: "REQ%", width: 5, align: alignRight},
	{title: "USED%", width: 5, align: alignRight},
	{title: "ALLOC MEM", width: 9, align: alignRight, priority: 3},
	{title: "REQ%", width: 5, align: alignRight, priority: 1},
	{title: "USED%", width: 5, align: alignRight, priority: 1},
}

func (m Model) renderNodeGroups(rows int) string {
	groups := m.cluster.NodeGroups
	if rows > len(groups) {
		rows = len(groups)
	}

	table := make([]row, 0, rows)
	for _, g := range groups[:rows] {
		reqCPU, okReqCPU := model.Percent(g.Requested.CPUMilli, g.Allocatable.CPUMilli)
		reqMem, okReqMem := model.Percent(g.Requested.MemBytes, g.Allocatable.MemBytes)

		usedCPUCell := styled(model.Unavailable, styleDim)
		usedMemCell := usedCPUCell
		if g.UsageKnown {
			usedCPU, ok := model.Percent(g.Used.CPUMilli, g.Allocatable.CPUMilli)
			usedCPUCell = styled(model.FormatPercent(usedCPU, ok), utilStyle(usedCPU, ok))
			usedMem, ok := model.Percent(g.Used.MemBytes, g.Allocatable.MemBytes)
			usedMemCell = styled(model.FormatPercent(usedMem, ok), utilStyle(usedMem, ok))
		}

		table = append(table, row{cells: []cell{
			txt(g.Name),
			txt(fmt.Sprintf("%d", g.Nodes)),
			styled(fmt.Sprintf("%t", g.Tainted), styleDim),
			txt(model.FormatCores(g.Allocatable.CPUMilli)),
			txt(model.FormatPercent(reqCPU, okReqCPU)),
			usedCPUCell,
			txt(model.FormatMem(g.Allocatable.MemBytes)),
			txt(model.FormatPercent(reqMem, okReqMem)),
			usedMemCell,
		}})
	}

	content := renderTable(nodeGroupColumns, table, boxInner(m.width), rows+1, 0, -1)
	height := rows + 3
	if rows < len(groups) {
		content += "\n" + styleDim.Render(fmt.Sprintf("… %d more node groups (n hides this section)", len(groups)-rows))
		height++
	}
	return boxed(boxTitle("nodegroups", "", "", len(groups)), content, m.width, height)
}
