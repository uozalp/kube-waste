package model

import "testing"

func namespaces() []Namespace {
	return []Namespace{
		{Name: "hpe-nfs", Summary: Summary{
			Requested:  Resources{CPUMilli: 8000, MemBytes: 9 * gi},
			Used:       Resources{CPUMilli: 10, MemBytes: gi / 2},
			UsageKnown: true,
		}},
		{Name: "addon-solr-operator", Summary: Summary{
			Requested:  Resources{CPUMilli: 3980, MemBytes: gib(108.49)},
			Used:       Resources{CPUMilli: 1220, MemBytes: gib(118.04)},
			UsageKnown: true,
		}},
		{Name: "addon-dragonfly-operator", Summary: Summary{
			Requested:  Resources{CPUMilli: 6620, MemBytes: gib(108.88)},
			Used:       Resources{CPUMilli: 1230, MemBytes: gib(4.23)},
			UsageKnown: true,
		}},
	}
}

func nsNames(items []Namespace) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortNamespacesByWasteCPU(t *testing.T) {
	items := namespaces()
	SortNamespaces(items, SortWasteCPU, true)
	want := []string{"hpe-nfs", "addon-dragonfly-operator", "addon-solr-operator"}
	if got := nsNames(items); !equal(got, want) {
		t.Fatalf("desc order = %v, want %v", got, want)
	}

	SortNamespaces(items, SortWasteCPU, false)
	want = []string{"addon-solr-operator", "addon-dragonfly-operator", "hpe-nfs"}
	if got := nsNames(items); !equal(got, want) {
		t.Fatalf("asc order = %v, want %v", got, want)
	}
}

func TestSortNamespacesByWasteMemoryKeepsNegativeLast(t *testing.T) {
	items := namespaces()
	SortNamespaces(items, SortWasteMem, true)
	if got := nsNames(items)[2]; got != "addon-solr-operator" {
		t.Fatalf("last = %q, want addon-solr-operator (negative waste)", got)
	}
}

func TestSortNamespacesByName(t *testing.T) {
	items := namespaces()
	SortNamespaces(items, SortName, false)
	want := []string{"addon-dragonfly-operator", "addon-solr-operator", "hpe-nfs"}
	if got := nsNames(items); !equal(got, want) {
		t.Fatalf("name order = %v, want %v", got, want)
	}
}

func TestSortPodsByNodeThenName(t *testing.T) {
	pods := []Pod{
		{Name: "b", Node: "worker-12"},
		{Name: "a", Node: "worker-12"},
		{Name: "c", Node: "worker-03"},
	}
	SortPods(pods, SortNode, false)
	if pods[0].Name != "c" || pods[1].Name != "a" || pods[2].Name != "b" {
		t.Fatalf("order = %v", pods)
	}
}

func TestSortContextsByCurrent(t *testing.T) {
	items := []ContextInfo{
		{Name: "development"},
		{Name: "production", Current: true},
		{Name: "staging"},
	}
	SortContexts(items, SortCurrent, true)
	if items[0].Name != "production" {
		t.Fatalf("first = %q, want production", items[0].Name)
	}
}

func TestFiltering(t *testing.T) {
	items := namespaces()
	if got := FilterNamespaces(items, "addon"); len(got) != 2 {
		t.Fatalf("filter addon matched %d namespaces, want 2", len(got))
	}
	if got := FilterNamespaces(items, "SOLR"); len(got) != 1 {
		t.Fatalf("filter should be case-insensitive, matched %d", len(got))
	}
	if got := FilterNamespaces(items, ""); len(got) != len(items) {
		t.Fatal("empty filter must match everything")
	}
	if got := FilterNamespaces(items, "nope"); len(got) != 0 {
		t.Fatalf("unmatched filter returned %d rows", len(got))
	}
}

func TestFilterPodsMatchesNode(t *testing.T) {
	pods := []Pod{
		{Name: "payments-api-7d8f6c9b7d-xk92m", Node: "worker-12"},
		{Name: "payments-worker-5c9d", Node: "database-01"},
	}
	if got := FilterPods(pods, "worker-12"); len(got) != 1 || got[0].Node != "worker-12" {
		t.Fatalf("node filter = %v", got)
	}
	if got := FilterPods(pods, "payments"); len(got) != 2 {
		t.Fatalf("name filter matched %d pods, want 2", len(got))
	}
}
