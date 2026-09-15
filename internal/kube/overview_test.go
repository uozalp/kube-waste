package kube

import "testing"

func TestAggregateOverviewSkipsDetail(t *testing.T) {
	snap := baseSnapshot()
	// The overview collector never fetches these.
	snap.Namespaces = nil
	snap.PodUsage = nil
	snap.PodMetricsOK = false

	overview := AggregateOverview("test", snap)

	if len(overview.Namespaces) != 0 {
		t.Fatalf("namespaces = %v, want none", overview.Namespaces)
	}
	if len(overview.Pods) != 0 {
		t.Fatalf("pods = %v, want none", overview.Pods)
	}

	full := Aggregate("test", baseSnapshot())
	if len(overview.NodeGroups) != len(full.NodeGroups) {
		t.Fatalf("got %d node groups, want %d", len(overview.NodeGroups), len(full.NodeGroups))
	}
	for i, group := range overview.NodeGroups {
		want := full.NodeGroups[i]
		if group.Name != want.Name {
			t.Fatalf("node group %d = %q, want %q", i, group.Name, want.Name)
		}
		if group.Requested != want.Requested || group.Used != want.Used {
			t.Fatalf("node group %q = %+v, want %+v", group.Name, group.Summary, want.Summary)
		}
	}
	if overview.Requested != full.Requested {
		t.Fatalf("cluster requests = %+v, want %+v", overview.Requested, full.Requested)
	}
}

func TestAggregateMapsNodesToGroups(t *testing.T) {
	cluster := Aggregate("test", baseSnapshot())

	want := map[string]string{"worker-1": "worker", "worker-2": "worker", "db-1": "database"}
	for node, group := range want {
		if got := cluster.NodeGroupOf[node]; got != group {
			t.Fatalf("NodeGroupOf[%q] = %q, want %q", node, got, group)
		}
	}
}
