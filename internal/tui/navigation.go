package tui

import "github.com/uozalp/kube-waste/internal/model"

var (
	contextSortKeys = []model.SortKey{
		model.SortName, model.SortCurrent, model.SortStatus, model.SortCluster,
	}

	namespaceSortKeys = []model.SortKey{
		model.SortName, model.SortPodCount,
		model.SortReqCPU, model.SortUsedCPU, model.SortWasteCPU,
		model.SortReqMem, model.SortUsedMem, model.SortWasteMem,
	}

	podSortKeys = []model.SortKey{
		model.SortName, model.SortNode,
		model.SortReqCPU, model.SortUsedCPU, model.SortWasteCPU,
		model.SortReqMem, model.SortUsedMem, model.SortWasteMem,
	}
)

func (m *Model) sortKeys() []model.SortKey {
	switch m.screen {
	case screenContexts:
		return contextSortKeys
	case screenPods:
		return podSortKeys
	default:
		return namespaceSortKeys
	}
}

// cycleSort moves to the next sortable column of the current view.
func (m *Model) cycleSort(dir int) {
	keys := m.sortKeys()
	st := m.state()
	pos := 0
	for i, k := range keys {
		if k == st.sortKey {
			pos = i
			break
		}
	}
	next := keys[((pos+dir)%len(keys)+len(keys))%len(keys)]
	m.setSort(next)
}

// setSort selects a column, toggling the direction when it is already active.
func (m *Model) setSort(key model.SortKey) {
	st := m.state()
	if st.sortKey == key {
		st.sortDesc = !st.sortDesc
	} else {
		st.sortKey = key
		// Names read best ascending, amounts read best largest-first.
		st.sortDesc = !key.Textual()
	}
	m.resort()
}

func (m *Model) toggleSortDirection() {
	m.state().sortDesc = !m.state().sortDesc
	m.resort()
}

// resort re-sorts the current view and jumps back to the top row, so the new
// order is visible right away instead of scrolling along with the old row.
func (m *Model) resort() {
	switch m.screen {
	case screenContexts:
		m.applyContextSort()
	case screenPods:
		m.applyPodSort()
	default:
		m.applyNamespaceSort()
	}
	st := m.state()
	st.cursor, st.offset = 0, 0
}

func (m *Model) applyContextSort() {
	for i := range m.contexts {
		m.contexts[i].Status = m.probes[m.contexts[i].Name]
	}
	model.SortContexts(m.contexts, m.ctxTable.sortKey, m.ctxTable.sortDesc)
}

// setProbe records a reachability result, keeping the order right when the
// context table is sorted by status.
func (m *Model) setProbe(name, status string) {
	if m.probes == nil {
		return
	}
	m.probes[name] = status
	if m.ctxTable.sortKey == model.SortStatus {
		m.applyContextSort()
	}
}

func (m *Model) applyNamespaceSort() {
	if m.cluster == nil {
		return
	}
	model.SortNamespaces(m.cluster.Namespaces, m.nsTable.sortKey, m.nsTable.sortDesc)
}

func (m *Model) applyPodSort() {
	if m.cluster == nil {
		return
	}
	for _, pods := range m.cluster.Pods {
		model.SortPods(pods, m.podTable.sortKey, m.podTable.sortDesc)
	}
}

func (m Model) visibleContexts() []model.ContextInfo {
	return model.FilterContexts(m.contexts, m.ctxTable.filter)
}

func (m Model) visibleNamespaces() []model.Namespace {
	if m.cluster == nil {
		return nil
	}
	return model.FilterNamespaces(m.cluster.Namespaces, m.nsTable.filter)
}

func (m Model) visiblePods() []model.Pod {
	if m.cluster == nil {
		return nil
	}
	return model.FilterPods(m.cluster.NamespacePods(m.namespace), m.podTable.filter)
}
