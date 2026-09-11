package tui

import (
	"fmt"
	"strings"

	"github.com/uozalp/kube-waste/internal/model"
)

var namespaceColumns = []column{
	{title: "NAMESPACE", width: 24, flex: true, sortable: true, sort: model.SortName},
	{title: "PODS", width: 5, align: alignRight, priority: 6, sortable: true, sort: model.SortPodCount},
	{title: "REQ CPU", width: 9, align: alignRight, priority: 4, sortable: true, sort: model.SortReqCPU},
	{title: "USED CPU", width: 9, align: alignRight, priority: 3, sortable: true, sort: model.SortUsedCPU},
	{title: "WASTE CPU", width: 10, align: alignRight, sortable: true, sort: model.SortWasteCPU},
	{title: "REQ MEM", width: 10, align: alignRight, priority: 4, sortable: true, sort: model.SortReqMem},
	{title: "USED MEM", width: 10, align: alignRight, priority: 3, sortable: true, sort: model.SortUsedMem},
	{title: "WASTE MEM", width: 11, align: alignRight, sortable: true, sort: model.SortWasteMem},
}

// clusterLayout splits the body between the node-group box and the namespace
// box. Both heights exclude the surrounding border.
func (m Model) clusterLayout() (groupRows, tableHeight int) {
	body := m.bodyHeight()
	if m.cluster == nil {
		return 0, body
	}

	total := len(m.cluster.NodeGroups)
	groupLines := 0

	if m.showGroups && total > 0 {
		// The namespace box keeps its border, header and two rows.
		avail := body - 5 - 4 // minus namespace block, minus group border/header/blank
		switch {
		case avail >= total:
			groupRows, groupLines = total, total+4
		case avail >= 2:
			groupRows, groupLines = avail-1, avail+4 // one line for the "… more" note
		}
	}

	tableHeight = body - groupLines - 2 // namespace border
	return groupRows, max(1, tableHeight)
}

func (m Model) renderCluster() string {
	if m.cluster == nil {
		if m.errMsg != "" {
			return styleError.Render(m.errMsg)
		}
		return m.loadingBlock("loading " + m.contextName + "…")
	}

	groupRows, tableHeight := m.clusterLayout()

	var b strings.Builder
	if groupRows > 0 {
		b.WriteString(m.renderNodeGroups(groupRows))
		b.WriteString("\n\n")
	}

	items := m.visibleNamespaces()
	title := boxTitle("namespaces", "all", m.nsTable.filter, len(items))
	if len(items) == 0 {
		b.WriteString(boxed(title, styleDim.Render("no namespaces match"), m.width, tableHeight+2))
		return b.String()
	}

	rows := make([]row, 0, len(items))
	for _, ns := range items {
		usedCPU, wasteCPU, usedMem, wasteMem := usageCells(ns.Summary)
		rows = append(rows, row{cells: []cell{
			txt(ns.Name),
			styled(fmt.Sprintf("%d", ns.Pods), styleDim),
			txt(model.FormatCores(ns.Requested.CPUMilli)),
			usedCPU,
			wasteCPU,
			txt(model.FormatMem(ns.Requested.MemBytes)),
			usedMem,
			wasteMem,
		}})
	}

	cursor := clamp(m.nsTable.cursor, 0, len(rows)-1)
	offset := scrollOffset(m.nsTable.offset, cursor, tableHeight-1, len(rows))
	cols := markSort(namespaceColumns, m.nsTable.sortKey, m.nsTable.sortDesc)
	b.WriteString(boxed(title, renderTable(cols, rows, boxInner(m.width), tableHeight, offset, cursor), m.width, tableHeight+2))
	return b.String()
}
