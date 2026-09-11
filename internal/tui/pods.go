package tui

import (
	"github.com/uozalp/kube-waste/internal/model"
)

var podColumns = []column{
	{title: "POD", width: 28, flex: true, sortable: true, sort: model.SortName},
	{title: "NODE", width: 16, flex: true, priority: 5, sortable: true, sort: model.SortNode},
	{title: "REQ CPU", width: 9, align: alignRight, priority: 4, sortable: true, sort: model.SortReqCPU},
	{title: "USED CPU", width: 9, align: alignRight, priority: 3, sortable: true, sort: model.SortUsedCPU},
	{title: "WASTE CPU", width: 10, align: alignRight, sortable: true, sort: model.SortWasteCPU},
	{title: "REQ MEM", width: 10, align: alignRight, priority: 4, sortable: true, sort: model.SortReqMem},
	{title: "USED MEM", width: 10, align: alignRight, priority: 3, sortable: true, sort: model.SortUsedMem},
	{title: "WASTE MEM", width: 11, align: alignRight, sortable: true, sort: model.SortWasteMem},
}

func (m Model) renderPods() string {
	items := m.visiblePods()
	title := boxTitle("pods", m.namespace, m.podTable.filter, len(items))
	height := m.bodyHeight()

	if len(items) == 0 {
		return boxed(title, styleDim.Render("no pods match"), m.width, height)
	}

	rows := make([]row, 0, len(items))
	for _, p := range items {
		usedCPU, wasteCPU, usedMem, wasteMem := usageCells(p.Summary)
		rows = append(rows, row{cells: []cell{
			txt(p.Name),
			styled(p.Node, styleDim),
			txt(model.FormatCores(p.Requested.CPUMilli)),
			usedCPU,
			wasteCPU,
			txt(model.FormatMem(p.Requested.MemBytes)),
			usedMem,
			wasteMem,
		}})
	}

	inner := m.tableHeight()
	cursor := clamp(m.podTable.cursor, 0, len(rows)-1)
	offset := scrollOffset(m.podTable.offset, cursor, inner-1, len(rows))
	cols := markSort(podColumns, m.podTable.sortKey, m.podTable.sortDesc)
	return boxed(title, renderTable(cols, rows, boxInner(m.width), inner, offset, cursor), m.width, height)
}
