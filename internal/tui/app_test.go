package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uozalp/kube-waste/internal/model"
)

// gib builds a byte count from a GiB value.
func gib(v float64) int64 { return int64(v * float64(int64(1)<<30)) }

func testCluster() *model.Cluster {
	ns := func(name string, reqCPU, usedCPU int64, reqMem, usedMem float64) model.Namespace {
		return model.Namespace{
			Name: name,
			Summary: model.Summary{
				Requested:  model.Resources{CPUMilli: reqCPU, MemBytes: gib(reqMem)},
				Used:       model.Resources{CPUMilli: usedCPU, MemBytes: gib(usedMem)},
				UsageKnown: true,
			},
			Pods: 1,
		}
	}

	c := &model.Cluster{
		Context:     "production",
		Allocatable: model.Resources{CPUMilli: 440000, MemBytes: gib(1928.4)},
		Summary: model.Summary{
			Requested:  model.Resources{CPUMilli: 148600, MemBytes: gib(855)},
			Used:       model.Resources{CPUMilli: 87000, MemBytes: gib(692)},
			UsageKnown: true,
		},
		NodeGroups: []model.NodeGroup{
			{Name: "worker", Nodes: 25, Allocatable: model.Resources{CPUMilli: 200000, MemBytes: gib(783.3)},
				Summary: model.Summary{
					Requested:  model.Resources{CPUMilli: 136000, MemBytes: gib(618.8)},
					Used:       model.Resources{CPUMilli: 64000, MemBytes: gib(438.6)},
					UsageKnown: true,
				}},
			{Name: "jmeter", Nodes: 10, Tainted: true, Allocatable: model.Resources{CPUMilli: 160000, MemBytes: gib(627.8)},
				Summary: model.Summary{UsageKnown: true}},
		},
		Namespaces: []model.Namespace{
			ns("hpe-nfs", 8000, 10, 9, 0.54),
			ns("addon-solr-operator", 3980, 1220, 108.49, 118.04),
			ns("payments", 4000, 560, 8, 2.1),
		},
		Pods: map[string][]model.Pod{
			"payments": {
				{Name: "payments-api-7d8f6c9b7d-xk92m", Namespace: "payments", Node: "worker-12", Summary: model.Summary{
					Requested: model.Resources{CPUMilli: 2000, MemBytes: gib(4)},
					Used:      model.Resources{CPUMilli: 250, MemBytes: gib(1.1)}, UsageKnown: true}},
				{Name: "payments-api-7d8f6c9b7d-j82kd", Namespace: "payments", Node: "worker-13", Summary: model.Summary{
					Requested: model.Resources{CPUMilli: 2000, MemBytes: gib(4)},
					Used:      model.Resources{CPUMilli: 310, MemBytes: gib(1.2)}, UsageKnown: true}},
			},
		},
	}
	model.SortNamespaces(c.Namespaces, model.SortWasteCPU, true)
	return c
}

func newTestModel() Model {
	m := New(Options{})
	m.width, m.height = 140, 40
	m.screen = screenCluster
	m.contextName = "production"
	m.cluster = testCluster()
	return m
}

func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		if len(k) == 1 {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		} else {
			msg = tea.KeyMsg{Type: keyTypes[k]}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

var keyTypes = map[string]tea.KeyType{
	"enter":     tea.KeyEnter,
	"esc":       tea.KeyEscape,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"backspace": tea.KeyBackspace,
}

func TestClusterViewShowsOverviewAndWaste(t *testing.T) {
	out := newTestModel().View()

	for _, want := range []string{
		"Contexts > production > Namespaces",
		"ALLOCATABLE",
		"440.0 cores",
		"1928.4 GiB",
		"148.6 cores (34%)",
		"87.0 cores (20%)",
		"692.0 GiB (36%)",
		"nodegroups[2]",
		"namespaces(all)[3]",
		"WASTE CPU",
		"7.99c",
		"-9.55G",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q\n%s", want, out)
		}
	}
}

func TestMetricsUnavailableIsExplained(t *testing.T) {
	m := newTestModel()
	m.cluster.UsageKnown = false
	m.cluster.MetricsError = "metrics.k8s.io API could not be queried"
	for i := range m.cluster.Namespaces {
		m.cluster.Namespaces[i].UsageKnown = false
	}

	out := m.View()
	if !strings.Contains(out, "USED") {
		t.Error("overview should keep the used row")
	}
	if !strings.Contains(out, "Metrics unavailable") {
		t.Error("view should explain why metrics are missing")
	}
	if !strings.Contains(out, model.Unavailable) {
		t.Error("namespace table should show N/A instead of zero usage")
	}
	if !strings.Contains(out, "8.00c") {
		t.Error("requests must still be shown without metrics")
	}
}

func TestNavigationIntoPodsAndBack(t *testing.T) {
	m := newTestModel()
	m.nsTable.cursor = 0
	m = send(t, m, "/")
	m = send(t, m, "p", "a", "y")
	m = send(t, m, "enter")
	m = send(t, m, "enter")

	if m.screen != screenPods || m.namespace != "payments" {
		t.Fatalf("screen = %v namespace = %q", m.screen, m.namespace)
	}
	out := m.View()
	if !strings.Contains(out, "payments-api-7d8f6c9b7d-xk92m") || !strings.Contains(out, "worker-12") {
		t.Errorf("pod view missing rows\n%s", out)
	}

	m = send(t, m, "esc")
	if m.screen != screenCluster || m.namespace != "" {
		t.Fatalf("esc did not return to namespaces: %v", m.screen)
	}
}

func TestFilteringNarrowsTheTable(t *testing.T) {
	m := newTestModel()
	m = send(t, m, "/")
	m = send(t, m, "s", "o", "l", "r")

	if got := len(m.visibleNamespaces()); got != 1 {
		t.Fatalf("filtered rows = %d, want 1", got)
	}

	m = send(t, m, "backspace")
	if got := len(m.visibleNamespaces()); got != 1 {
		t.Fatalf("rows after backspace = %d, want 1", got)
	}

	m = send(t, m, "esc")
	if m.filtering || m.nsTable.filter != "" {
		t.Fatal("esc should clear the filter")
	}
	if got := len(m.visibleNamespaces()); got != 3 {
		t.Fatalf("rows after clearing = %d, want 3", got)
	}
}

func TestEscClearsFilterBeforeGoingBack(t *testing.T) {
	m := newTestModel()
	m = send(t, m, "/")
	m = send(t, m, "s", "o", "l")
	m = send(t, m, "enter")

	if m.filtering || m.nsTable.filter != "sol" {
		t.Fatalf("enter should apply the filter: filtering=%v filter=%q", m.filtering, m.nsTable.filter)
	}
	if !strings.Contains(m.View(), "</sol>") {
		t.Errorf("title should show the active filter\n%s", m.View())
	}

	m = send(t, m, "esc")
	if m.nsTable.filter != "" {
		t.Fatalf("first esc should clear the filter, got %q", m.nsTable.filter)
	}
	if m.screen != screenCluster {
		t.Fatalf("first esc should stay on the namespace screen, got %v", m.screen)
	}

	m = send(t, m, "esc")
	if m.screen != screenContexts {
		t.Fatalf("second esc should go back, got %v", m.screen)
	}
}

func TestSortKeysJumpToTop(t *testing.T) {
	m := newTestModel()
	m.nsTable.cursor = 1

	m = send(t, m, "m") // sort by wasted memory
	if m.nsTable.sortKey != model.SortWasteMem || !m.nsTable.sortDesc {
		t.Fatalf("sort = %v desc=%v", m.nsTable.sortKey, m.nsTable.sortDesc)
	}
	if m.nsTable.cursor != 0 || m.nsTable.offset != 0 {
		t.Errorf("sorting should jump to the top, cursor=%d offset=%d", m.nsTable.cursor, m.nsTable.offset)
	}

	m = send(t, m, "m") // pressing again reverses the direction
	if m.nsTable.sortDesc {
		t.Error("repeating the sort key should reverse the order")
	}

	m = send(t, m, "c")
	if m.nsTable.sortKey != model.SortWasteCPU {
		t.Errorf("sort = %v, want waste cpu", m.nsTable.sortKey)
	}
	if m.cluster.Namespaces[0].Name != "hpe-nfs" {
		t.Errorf("top row = %q, want hpe-nfs", m.cluster.Namespaces[0].Name)
	}
}

func TestCursorStaysInRange(t *testing.T) {
	m := newTestModel()
	m = send(t, m, "k", "k", "k")
	if m.nsTable.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.nsTable.cursor)
	}
	m = send(t, m, "j", "j", "j", "j", "j")
	if want := len(m.cluster.Namespaces) - 1; m.nsTable.cursor != want {
		t.Errorf("cursor = %d, want %d", m.nsTable.cursor, want)
	}
}

func TestHelpOverlayToggles(t *testing.T) {
	m := send(t, newTestModel(), "?")
	if !strings.Contains(m.View(), "Move up") {
		t.Error("help overlay not shown")
	}
	m = send(t, m, "?")
	if strings.Contains(m.View(), "Move up") {
		t.Error("help overlay not hidden")
	}
}

func TestNarrowTerminalKeepsWasteColumns(t *testing.T) {
	m := newTestModel()
	m.width = 52
	out := m.View()
	if !strings.Contains(out, "WASTE CPU") || !strings.Contains(out, "WASTE MEM") {
		t.Errorf("waste columns dropped on a narrow terminal\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if got := len([]rune(line)); got > m.width {
			t.Fatalf("line of %d runes exceeds width %d: %q", got, m.width, line)
		}
	}
}

func TestClusterErrorIsDisplayed(t *testing.T) {
	m := New(Options{})
	m.width, m.height = 120, 30
	m.screen = screenCluster
	m.contextName = "staging"

	next, _ := m.Update(clusterMsg{context: "staging", err: errForbidden{}})
	m = next.(Model)

	if m.probes["staging"] != "Error: forbidden" {
		t.Errorf("probe status = %q", m.probes["staging"])
	}
	if m.screen != screenContexts {
		t.Error("a failed context should return to the context list")
	}
}

type errForbidden struct{}

func (errForbidden) Error() string { return "forbidden" }
