package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func node(name string, labels map[string]string, taints ...corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       corev1.NodeSpec{Taints: taints},
	}
}

func TestNodeGroupNameFromWellKnownLabels(t *testing.T) {
	cases := []struct {
		label string
		want  string
	}{
		{"cloud.google.com/gke-nodepool", "worker"},
		{"eks.amazonaws.com/nodegroup", "worker"},
		{"karpenter.sh/nodepool", "worker"},
		{"kubernetes.azure.com/agentpool", "worker"},
	}
	for _, c := range cases {
		n := node("ip-10-0-1-23.eu-west-1.compute.internal", map[string]string{c.label: c.want})
		if got := NodeGroupName(n, DefaultStrategies()); got != c.want {
			t.Errorf("%s: got %q, want %q", c.label, got, c.want)
		}
	}
}

func TestNodeGroupNameFallsBackToRole(t *testing.T) {
	n := node("cp-1", map[string]string{"node-role.kubernetes.io/control-plane": ""})
	if got := NodeGroupName(n, DefaultStrategies()); got != "control-plane" {
		t.Fatalf("got %q, want control-plane", got)
	}
}

func TestNodeGroupNameFallsBackToNamePrefix(t *testing.T) {
	cases := map[string]string{
		"worker-12":      "worker",
		"controlplane-3": "controlplane",
		"jmeter-01":      "jmeter",
		"database":       "database",
		"gke-pool-a1b2":  "gke-pool",
	}
	for name, want := range cases {
		if got := NodeGroupName(node(name, nil), DefaultStrategies()); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestLabelStrategyPrefersFirstMatch(t *testing.T) {
	n := node("n1", map[string]string{
		"eks.amazonaws.com/nodegroup":   "second",
		"cloud.google.com/gke-nodepool": "first",
	})
	if got := NodeGroupName(n, DefaultStrategies()); got != "first" {
		t.Fatalf("got %q, want first", got)
	}
}

func TestStrategiesPrefersConfiguredLabel(t *testing.T) {
	const label = "node.danskespil.dk/workload"
	strategies := Strategies([]string{label})

	n := node("worker-12", map[string]string{
		label:                            "database",
		"cloud.google.com/gke-nodepool":  "pool-1",
		"node-role.kubernetes.io/worker": "",
	})
	if got := NodeGroupName(n, strategies); got != "database" {
		t.Fatalf("got %q, want database", got)
	}

	// Nodes without the configured label keep the built-in detection.
	cp := node("cp-1", map[string]string{"node-role.kubernetes.io/control-plane": ""})
	if got := NodeGroupName(cp, strategies); got != "control-plane" {
		t.Fatalf("got %q, want control-plane", got)
	}
}

func TestIsTainted(t *testing.T) {
	plain := node("n1", nil)
	if IsTainted(plain) {
		t.Error("untainted node reported as tainted")
	}

	tainted := node("n2", nil, corev1.Taint{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule})
	if !IsTainted(tainted) {
		t.Error("NoSchedule taint not detected")
	}

	soft := node("n3", nil, corev1.Taint{Key: "hint", Effect: corev1.TaintEffectPreferNoSchedule})
	if IsTainted(soft) {
		t.Error("PreferNoSchedule should not count as tainted")
	}
}
