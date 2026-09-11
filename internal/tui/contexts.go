package tui

import (
	"github.com/uozalp/kube-waste/internal/model"
)

var contextColumns = []column{
	{title: "CONTEXT", width: 24, flex: true, sortable: true, sort: model.SortName},
	{title: "CURRENT", width: 7, priority: 2, sortable: true, sort: model.SortCurrent},
	{title: "STATUS", width: 30, flex: true, priority: 1, sortable: true, sort: model.SortStatus},
	{title: "CLUSTER", width: 20, flex: true, priority: 3, sortable: true, sort: model.SortCluster},
}

func (m Model) renderContexts() string {
	items := m.visibleContexts()
	title := boxTitle("contexts", "", m.ctxTable.filter, len(items))
	height := m.bodyHeight()

	if m.contextErr != "" {
		return boxed(title, styleError.Render("kubeconfig error: "+m.contextErr), m.width, height)
	}
	if len(items) == 0 {
		return boxed(title, styleDim.Render("no kubeconfig contexts match"), m.width, height)
	}

	rows := make([]row, 0, len(items))
	for _, c := range items {
		current := ""
		if c.Current {
			current = "*"
		}
		rows = append(rows, row{cells: []cell{
			txt(c.Name),
			styled(current, styleKey),
			m.probeCell(c.Name),
			styled(c.Cluster, styleDim),
		}})
	}

	inner := m.tableHeight()
	cursor := clamp(m.ctxTable.cursor, 0, len(rows)-1)
	offset := scrollOffset(m.ctxTable.offset, cursor, inner-1, len(rows))
	cols := markSort(contextColumns, m.ctxTable.sortKey, m.ctxTable.sortDesc)
	return boxed(title, renderTable(cols, rows, boxInner(m.width), inner, offset, cursor), m.width, height)
}

func (m Model) probeCell(name string) cell {
	status, ok := m.probes[name]
	switch {
	case !ok:
		return styled("checking…", styleDim)
	case status == "Ready":
		return styled(status, styleOK)
	default:
		return styled(status, styleError)
	}
}
