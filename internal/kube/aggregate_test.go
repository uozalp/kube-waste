package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/uozalp/kube-waste/internal/model"
)

func testNode(name, pool, cpu, mem string, taints ...corev1.Taint) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"karpenter.sh/nodepool": pool},
		},
		Spec: corev1.NodeSpec{Taints: taints},
		Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(mem),
		}},
	}
}

func testPod(namespace, name, nodeName, cpu, mem string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			NodeName:   nodeName,
			Containers: []corev1.Container{{Name: "app", Resources: requests(cpu, mem)}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func baseSnapshot() Snapshot {
	return Snapshot{
		Nodes: []corev1.Node{
			testNode("worker-1", "worker", "8", "32Gi"),
			testNode("worker-2", "worker", "8", "32Gi"),
			testNode("db-1", "database", "8", "32Gi", corev1.Taint{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}),
		},
		Pods: []corev1.Pod{
			testPod("hpe-nfs", "nfs-0", "worker-1", "8", "9Gi"),
			testPod("payments", "api-1", "worker-2", "2", "4Gi"),
			testPod("payments", "api-2", "db-1", "2", "4Gi"),
		},
		Namespaces: []string{"hpe-nfs", "payments", "empty"},
		NodeUsage: map[string]model.Resources{
			"worker-1": {CPUMilli: 100, MemBytes: 1 << 30},
			"worker-2": {CPUMilli: 500, MemBytes: 2 << 30},
			"db-1":     {CPUMilli: 250, MemBytes: 1 << 30},
		},
		PodUsage: map[string]model.Resources{
			PodKey("hpe-nfs", "nfs-0"):  {CPUMilli: 10, MemBytes: 580813128},
			PodKey("payments", "api-1"): {CPUMilli: 250, MemBytes: 1 << 30},
			PodKey("payments", "api-2"): {CPUMilli: 310, MemBytes: 1 << 30},
		},
		NodeMetricsOK: true,
		PodMetricsOK:  true,
	}
}

func findNamespace(t *testing.T, c *model.Cluster, name string) model.Namespace {
	t.Helper()
	for _, ns := range c.Namespaces {
		if ns.Name == name {
			return ns
		}
	}
	t.Fatalf("namespace %q not found", name)
	return model.Namespace{}
}

func findGroup(t *testing.T, c *model.Cluster, name string) model.NodeGroup {
	t.Helper()
	for _, g := range c.NodeGroups {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("node group %q not found", name)
	return model.NodeGroup{}
}

func TestAggregateClusterTotals(t *testing.T) {
	c := Aggregate("production", baseSnapshot())

	if c.Allocatable.CPUMilli != 24000 {
		t.Errorf("allocatable cpu = %d milli, want 24000", c.Allocatable.CPUMilli)
	}
	if c.Requested.CPUMilli != 12000 {
		t.Errorf("requested cpu = %d milli, want 12000", c.Requested.CPUMilli)
	}
	if c.Used.CPUMilli != 850 {
		t.Errorf("used cpu = %d milli, want 850 (node metrics)", c.Used.CPUMilli)
	}
	if !c.UsageKnown {
		t.Error("usage should be known")
	}
}

func TestAggregateNamespaceSummary(t *testing.T) {
	c := Aggregate("production", baseSnapshot())

	nfs := findNamespace(t, c, "hpe-nfs")
	if got := model.FormatCores(nfs.Requested.CPUMilli); got != "8.00c" {
		t.Errorf("requested cpu = %q", got)
	}
	if got := model.FormatCores(nfs.Used.CPUMilli); got != "0.01c" {
		t.Errorf("used cpu = %q", got)
	}
	if got := model.FormatCores(nfs.Waste().CPUMilli); got != "7.99c" {
		t.Errorf("waste cpu = %q, want 7.99c", got)
	}
	if got := model.FormatMem(nfs.Waste().MemBytes); got != "8.46G" {
		t.Errorf("waste memory = %q, want 8.46G", got)
	}

	payments := findNamespace(t, c, "payments")
	if payments.Pods != 2 {
		t.Errorf("payments pods = %d, want 2", payments.Pods)
	}
	if payments.Requested.CPUMilli != 4000 {
		t.Errorf("payments requested cpu = %d milli, want 4000", payments.Requested.CPUMilli)
	}
}

func TestAggregateKeepsEmptyNamespaces(t *testing.T) {
	c := Aggregate("production", baseSnapshot())
	empty := findNamespace(t, c, "empty")
	if !empty.Requested.IsZero() || empty.Pods != 0 {
		t.Fatalf("empty namespace = %+v", empty)
	}
}

func TestAggregateDefaultSortIsWasteCPUDescending(t *testing.T) {
	c := Aggregate("production", baseSnapshot())
	if c.Namespaces[0].Name != "hpe-nfs" {
		t.Fatalf("first namespace = %q, want hpe-nfs", c.Namespaces[0].Name)
	}
}

func TestAggregateNodeGroups(t *testing.T) {
	c := Aggregate("production", baseSnapshot())
	if len(c.NodeGroups) != 2 {
		t.Fatalf("node groups = %d, want 2", len(c.NodeGroups))
	}

	worker := findGroup(t, c, "worker")
	if worker.Nodes != 2 {
		t.Errorf("worker nodes = %d, want 2", worker.Nodes)
	}
	if worker.Tainted {
		t.Error("worker group should not be tainted")
	}
	if worker.Allocatable.CPUMilli != 16000 {
		t.Errorf("worker allocatable cpu = %d milli, want 16000", worker.Allocatable.CPUMilli)
	}
	if worker.Requested.CPUMilli != 10000 {
		t.Errorf("worker requested cpu = %d milli, want 10000", worker.Requested.CPUMilli)
	}
	if worker.Used.CPUMilli != 600 {
		t.Errorf("worker used cpu = %d milli, want 600", worker.Used.CPUMilli)
	}

	db := findGroup(t, c, "database")
	if !db.Tainted {
		t.Error("database group should be tainted")
	}
	pct, ok := model.Percent(db.Requested.CPUMilli, db.Allocatable.CPUMilli)
	if !ok || model.FormatPercent(pct, ok) != "25%" {
		t.Errorf("database request percentage = %v", pct)
	}
}

func TestAggregateWithoutMetrics(t *testing.T) {
	snap := baseSnapshot()
	snap.NodeMetricsOK = false
	snap.PodMetricsOK = false
	snap.NodeUsage, snap.PodUsage = nil, nil
	snap.MetricsError = "metrics.k8s.io API could not be queried"

	c := Aggregate("production", snap)

	if c.UsageKnown {
		t.Error("cluster usage should be unknown")
	}
	if c.Requested.CPUMilli != 12000 {
		t.Errorf("requests must still be reported, got %d milli", c.Requested.CPUMilli)
	}
	nfs := findNamespace(t, c, "hpe-nfs")
	if nfs.UsageKnown {
		t.Error("namespace usage should be unknown")
	}
	if nfs.Used.CPUMilli != 0 {
		t.Errorf("used cpu = %d, want 0 placeholder", nfs.Used.CPUMilli)
	}
	for _, g := range c.NodeGroups {
		if g.UsageKnown {
			t.Errorf("node group %q usage should be unknown", g.Name)
		}
	}
}

func TestAggregateSkipsCompletedPods(t *testing.T) {
	snap := baseSnapshot()
	done := testPod("payments", "migration", "worker-1", "16", "64Gi")
	done.Status.Phase = corev1.PodSucceeded
	snap.Pods = append(snap.Pods, done)

	c := Aggregate("production", snap)
	if c.Requested.CPUMilli != 12000 {
		t.Fatalf("requested cpu = %d milli, want 12000", c.Requested.CPUMilli)
	}
	if got := len(c.Pods["payments"]); got != 2 {
		t.Fatalf("payments pods = %d, want 2", got)
	}
}

func TestAggregatePodsIndexedByNamespace(t *testing.T) {
	c := Aggregate("production", baseSnapshot())
	pods := c.NamespacePods("payments")
	if len(pods) != 2 {
		t.Fatalf("pods = %d, want 2", len(pods))
	}
	if pods[0].Node == "" {
		t.Error("pod node name missing")
	}
	// Default sort is wasted CPU descending.
	if pods[0].Waste().CPUMilli < pods[1].Waste().CPUMilli {
		t.Error("pods are not sorted by wasted CPU descending")
	}
}
