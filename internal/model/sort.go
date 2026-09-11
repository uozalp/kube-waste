package model

import (
	"sort"
	"strings"
)

// SortKey identifies a sortable column. Not every key applies to every table.
type SortKey int

const (
	SortName SortKey = iota
	SortNode
	SortReqCPU
	SortUsedCPU
	SortWasteCPU
	SortReqMem
	SortUsedMem
	SortWasteMem
	SortCurrent
	SortPodCount
	SortStatus
	SortCluster
)

// Label returns the short header label used in the sort indicator.
func (k SortKey) Label() string {
	switch k {
	case SortName:
		return "NAME"
	case SortNode:
		return "NODE"
	case SortReqCPU:
		return "REQ CPU"
	case SortUsedCPU:
		return "USED CPU"
	case SortWasteCPU:
		return "WASTE CPU"
	case SortReqMem:
		return "REQ MEM"
	case SortUsedMem:
		return "USED MEM"
	case SortWasteMem:
		return "WASTE MEM"
	case SortCurrent:
		return "CURRENT"
	case SortPodCount:
		return "PODS"
	case SortStatus:
		return "STATUS"
	case SortCluster:
		return "CLUSTER"
	}
	return "?"
}

// Textual reports whether the key sorts strings; those read best ascending.
func (k SortKey) Textual() bool {
	switch k {
	case SortName, SortNode, SortStatus, SortCluster:
		return true
	}
	return false
}

func summaryValue(s Summary, key SortKey) int64 {
	switch key {
	case SortReqCPU:
		return s.Requested.CPUMilli
	case SortUsedCPU:
		return s.Used.CPUMilli
	case SortWasteCPU:
		return s.Waste().CPUMilli
	case SortReqMem:
		return s.Requested.MemBytes
	case SortUsedMem:
		return s.Used.MemBytes
	case SortWasteMem:
		return s.Waste().MemBytes
	}
	return 0
}

// SortNamespaces sorts in place. Ties fall back to the namespace name so the
// order stays stable across refreshes.
func SortNamespaces(items []Namespace, key SortKey, desc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch key {
		case SortName:
			return lessString(a.Name, b.Name, desc)
		case SortPodCount:
			if a.Pods == b.Pods {
				return a.Name < b.Name
			}
			return lessInt(int64(a.Pods), int64(b.Pods), desc)
		}
		av, bv := summaryValue(a.Summary, key), summaryValue(b.Summary, key)
		if av == bv {
			return a.Name < b.Name
		}
		return lessInt(av, bv, desc)
	})
}

// SortPods sorts in place.
func SortPods(items []Pod, key SortKey, desc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch key {
		case SortName:
			return lessString(a.Name, b.Name, desc)
		case SortNode:
			if a.Node == b.Node {
				return a.Name < b.Name
			}
			return lessString(a.Node, b.Node, desc)
		}
		av, bv := summaryValue(a.Summary, key), summaryValue(b.Summary, key)
		if av == bv {
			return a.Name < b.Name
		}
		return lessInt(av, bv, desc)
	})
}

// SortContexts sorts in place. Ties fall back to the context name.
func SortContexts(items []ContextInfo, key SortKey, desc bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		switch key {
		case SortCurrent:
			if a.Current != b.Current {
				if desc {
					return a.Current
				}
				return b.Current
			}
		case SortStatus:
			if a.Status != b.Status {
				return lessString(a.Status, b.Status, desc)
			}
			return a.Name < b.Name
		case SortCluster:
			if a.Cluster != b.Cluster {
				return lessString(a.Cluster, b.Cluster, desc)
			}
			return a.Name < b.Name
		}
		return lessString(a.Name, b.Name, desc)
	})
}

func lessInt(a, b int64, desc bool) bool {
	if desc {
		return a > b
	}
	return a < b
}

func lessString(a, b string, desc bool) bool {
	if desc {
		return a > b
	}
	return a < b
}

// Matches reports whether any of the fields contains query, case-insensitively.
// An empty query matches everything.
func Matches(query string, fields ...string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// FilterNamespaces returns the namespaces whose name matches query.
func FilterNamespaces(items []Namespace, query string) []Namespace {
	if query == "" {
		return items
	}
	out := make([]Namespace, 0, len(items))
	for _, it := range items {
		if Matches(query, it.Name) {
			out = append(out, it)
		}
	}
	return out
}

// FilterPods returns the pods whose name or node matches query.
func FilterPods(items []Pod, query string) []Pod {
	if query == "" {
		return items
	}
	out := make([]Pod, 0, len(items))
	for _, it := range items {
		if Matches(query, it.Name, it.Node) {
			out = append(out, it)
		}
	}
	return out
}

// FilterContexts returns the contexts whose name or cluster matches query.
func FilterContexts(items []ContextInfo, query string) []ContextInfo {
	if query == "" {
		return items
	}
	out := make([]ContextInfo, 0, len(items))
	for _, it := range items {
		if Matches(query, it.Name, it.Cluster) {
			out = append(out, it)
		}
	}
	return out
}
