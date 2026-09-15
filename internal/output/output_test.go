package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/uozalp/kube-waste/internal/model"
)

const gi = 1 << 30

var fetchedAt = time.Date(2026, 9, 11, 21, 0, 0, 0, time.UTC)

func summary(cpuReq, cpuUsed, memReq, memUsed int64, known bool) model.Summary {
	return model.Summary{
		Requested:  model.Resources{CPUMilli: cpuReq, MemBytes: memReq},
		Used:       model.Resources{CPUMilli: cpuUsed, MemBytes: memUsed},
		UsageKnown: known,
	}
}

// testCluster has two node groups, two namespaces and three pods, with
// "payments" spread across both groups.
func testCluster() *model.Cluster {
	return &model.Cluster{
		Context:   "test",
		FetchedAt: fetchedAt,
		NodeGroups: []model.NodeGroup{
			{Name: "worker", Nodes: 2, Summary: summary(32000, 8000, 64*gi, 16*gi, true)},
			{Name: "database", Nodes: 1, Summary: summary(16000, 8000, 32*gi, 24*gi, true)},
		},
		Namespaces: []model.Namespace{
			{Name: "payments", Pods: 2, Summary: summary(4000, 1000, 8*gi, 2*gi, true)},
			{Name: "hpe-nfs", Pods: 1, Summary: summary(8000, 10, 9*gi, gi/2, true)},
		},
		Pods: map[string][]model.Pod{
			"payments": {
				{Name: "api-1", Namespace: "payments", Node: "worker-1", Summary: summary(2000, 500, 4*gi, gi, true)},
				{Name: "api-2", Namespace: "payments", Node: "db-1", Summary: summary(2000, 500, 4*gi, gi, true)},
			},
			"hpe-nfs": {
				{Name: "nfs-0", Namespace: "hpe-nfs", Node: "worker-2", Summary: summary(8000, 10, 9*gi, gi/2, true)},
			},
		},
		NodeGroupOf: map[string]string{
			"worker-1": "worker",
			"worker-2": "worker",
			"db-1":     "database",
		},
	}
}

func decodeJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	return out
}

func renderJSON(t *testing.T, report Report) ([]byte, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	return buf.Bytes(), decodeJSON(t, buf.Bytes())
}

func renderCSV(t *testing.T, report Report) [][]string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, report); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return records
}

func nested(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	for _, key := range keys {
		next, ok := m[key].(map[string]any)
		if !ok {
			t.Fatalf("key %q is not an object in %v", key, m)
		}
		m = next
	}
	return m
}

func wantNumber(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()
	got, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%q = %#v, want a JSON number", key, m[key])
	}
	if got != want {
		t.Fatalf("%q = %v, want %v", key, got, want)
	}
}

func TestOverviewJSON(t *testing.T) {
	report, err := BuildOverview(testCluster(), "")
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	raw, doc := renderJSON(t, report)

	if doc["context"] != "test" {
		t.Fatalf("context = %v, want test", doc["context"])
	}
	if doc["timestamp"] != "2026-09-11T21:00:00Z" {
		t.Fatalf("timestamp = %v, want RFC3339 2026-09-11T21:00:00Z", doc["timestamp"])
	}

	groups := nested(t, doc, "nodegroups")
	if len(groups) != 2 {
		t.Fatalf("got %d node groups, want 2", len(groups))
	}

	cpu := nested(t, doc, "nodegroups", "worker", "cpu")
	wantNumber(t, cpu, "requested", 32)
	wantNumber(t, cpu, "used", 8)
	wantNumber(t, cpu, "waste", 24)
	wantNumber(t, cpu, "percent", 25)

	mem := nested(t, doc, "nodegroups", "worker", "memory")
	wantNumber(t, mem, "requested", 64)
	wantNumber(t, mem, "used", 16)
	wantNumber(t, mem, "waste", 48)
	wantNumber(t, mem, "percent", 25)

	for _, unit := range []string{"%", "Gi", "GiB", "cores"} {
		if strings.Contains(string(raw), unit) {
			t.Fatalf("JSON contains display formatting %q: %s", unit, raw)
		}
	}
}

func TestOverviewNodeGroupFilter(t *testing.T) {
	report, err := BuildOverview(testCluster(), "worker")
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	_, doc := renderJSON(t, report)

	groups := nested(t, doc, "nodegroups")
	if len(groups) != 1 {
		t.Fatalf("got %d node groups, want only worker: %v", len(groups), groups)
	}
	if _, ok := groups["worker"]; !ok {
		t.Fatalf("worker missing from %v", groups)
	}
}

func TestUnknownNodeGroup(t *testing.T) {
	for _, scope := range []Scope{ScopeOverview, ScopeNamespace, ScopePod} {
		if _, err := Build(scope, testCluster(), "nope"); err == nil {
			t.Fatalf("scope %s: want an error for an unknown node group", scope)
		}
	}
}

func TestMissingMetricsAreNull(t *testing.T) {
	cluster := testCluster()
	for i := range cluster.NodeGroups {
		cluster.NodeGroups[i].UsageKnown = false
	}
	report, err := BuildOverview(cluster, "worker")
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	_, doc := renderJSON(t, report)

	cpu := nested(t, doc, "nodegroups", "worker", "cpu")
	wantNumber(t, cpu, "requested", 32)
	for _, key := range []string{"used", "waste", "percent"} {
		value, ok := cpu[key]
		if !ok {
			t.Fatalf("%q missing, want an explicit null", key)
		}
		if value != nil {
			t.Fatalf("%q = %#v, want null", key, value)
		}
	}
}

func TestZeroRequestPercentIsNull(t *testing.T) {
	cluster := testCluster()
	cluster.NodeGroups[0].Summary = summary(0, 0, 0, 0, true)

	report, err := BuildOverview(cluster, "worker")
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	_, doc := renderJSON(t, report)

	cpu := nested(t, doc, "nodegroups", "worker", "cpu")
	if cpu["percent"] != nil {
		t.Fatalf("percent = %#v, want null when nothing is requested", cpu["percent"])
	}
	wantNumber(t, cpu, "waste", 0)
}

func TestNegativeWasteIsNotClamped(t *testing.T) {
	cluster := testCluster()
	// 108.49 cores requested, 118.04 cores used.
	cluster.NodeGroups[0].Summary = summary(108490, 118040, 0, 0, true)

	report, err := BuildOverview(cluster, "worker")
	if err != nil {
		t.Fatalf("BuildOverview: %v", err)
	}
	raw, doc := renderJSON(t, report)

	cpu := nested(t, doc, "nodegroups", "worker", "cpu")
	wantNumber(t, cpu, "requested", 108.49)
	wantNumber(t, cpu, "used", 118.04)
	wantNumber(t, cpu, "waste", -9.55)
	if percent, _ := cpu["percent"].(float64); percent <= 100 {
		t.Fatalf("percent = %v, want more than 100", percent)
	}
	if !strings.Contains(string(raw), `"waste":-9.55`) {
		t.Fatalf("JSON does not carry the exact negative waste: %s", raw)
	}

	rows := renderCSV(t, report)
	if rows[1][3] != "-9.55" {
		t.Fatalf("cpu_waste = %q, want -9.55", rows[1][3])
	}
}

func TestNamespaceCSV(t *testing.T) {
	report, err := BuildNamespaces(testCluster(), "")
	if err != nil {
		t.Fatalf("BuildNamespaces: %v", err)
	}
	rows := renderCSV(t, report)

	want := []string{
		"namespace",
		"cpu_requested", "cpu_used", "cpu_waste", "cpu_percent",
		"memory_requested", "memory_used", "memory_waste", "memory_percent",
	}
	if strings.Join(rows[0], ",") != strings.Join(want, ",") {
		t.Fatalf("header = %v, want %v", rows[0], want)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a header and two namespaces: %v", len(rows), rows)
	}
	if rows[1][0] != "hpe-nfs" || rows[2][0] != "payments" {
		t.Fatalf("namespaces are not sorted: %v, %v", rows[1][0], rows[2][0])
	}
	// hpe-nfs: 8 cores requested, 0.01 used.
	if got := rows[1][1:5]; got[0] != "8" || got[1] != "0.01" || got[2] != "7.99" {
		t.Fatalf("hpe-nfs cpu = %v, want 8 / 0.01 / 7.99", got)
	}
}

func TestNamespaceNodeGroupFilter(t *testing.T) {
	report, err := BuildNamespaces(testCluster(), "database")
	if err != nil {
		t.Fatalf("BuildNamespaces: %v", err)
	}
	if len(report.Namespaces) != 1 || report.Namespaces[0].Namespace != "payments" {
		t.Fatalf("namespaces = %+v, want only payments", report.Namespaces)
	}
	// Only api-2 runs on the database group, so half of the payments requests.
	if got := report.Namespaces[0].CPU.Requested; got != 2 {
		t.Fatalf("cpu requested = %v, want 2", got)
	}
}

func TestPodCSV(t *testing.T) {
	report, err := BuildPods(testCluster(), "")
	if err != nil {
		t.Fatalf("BuildPods: %v", err)
	}
	rows := renderCSV(t, report)

	want := []string{
		"namespace", "pod", "node",
		"cpu_requested", "cpu_used", "cpu_waste", "cpu_percent",
		"memory_requested", "memory_used", "memory_waste", "memory_percent",
	}
	if strings.Join(rows[0], ",") != strings.Join(want, ",") {
		t.Fatalf("header = %v, want %v", rows[0], want)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want a header and three pods: %v", len(rows), rows)
	}
	if rows[1][0] != "hpe-nfs" || rows[1][1] != "nfs-0" || rows[1][2] != "worker-2" {
		t.Fatalf("first pod row = %v", rows[1])
	}
}

func TestPodNodeGroupFilter(t *testing.T) {
	report, err := BuildPods(testCluster(), "worker")
	if err != nil {
		t.Fatalf("BuildPods: %v", err)
	}
	if len(report.Pods) != 2 {
		t.Fatalf("got %d pods, want the two on the worker group: %+v", len(report.Pods), report.Pods)
	}
	for _, pod := range report.Pods {
		if pod.Node == "db-1" {
			t.Fatalf("pod %s from another node group leaked in", pod.Pod)
		}
	}
}

func TestPodMissingMetricsCSVIsEmpty(t *testing.T) {
	cluster := testCluster()
	for ns := range cluster.Pods {
		for i := range cluster.Pods[ns] {
			cluster.Pods[ns][i].UsageKnown = false
		}
	}
	report, err := BuildPods(cluster, "")
	if err != nil {
		t.Fatalf("BuildPods: %v", err)
	}
	rows := renderCSV(t, report)
	for _, row := range rows[1:] {
		if row[3] == "" || row[7] == "" {
			t.Fatalf("row %v lost its requests", row)
		}
		for _, i := range []int{4, 5, 6, 8, 9, 10} {
			if row[i] != "" {
				t.Fatalf("row %v column %d reports usage without metrics", row, i)
			}
		}
	}
}

func TestParseScopeAndFormat(t *testing.T) {
	if _, err := ParseScope("overview"); err != nil {
		t.Fatalf("ParseScope(overview): %v", err)
	}
	if _, err := ParseScope("nodes"); err == nil {
		t.Fatal("ParseScope(nodes): want an error")
	}
	if _, err := ParseFormat("csv"); err != nil {
		t.Fatalf("ParseFormat(csv): %v", err)
	}
	if _, err := ParseFormat("yaml"); err == nil {
		t.Fatal("ParseFormat(yaml): want an error")
	}
}
