package kube

import (
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/uozalp/kube-waste/internal/model"
)

// Snapshot is the raw material for aggregation. Keeping it a plain struct makes
// the aggregation logic testable without an API server.
type Snapshot struct {
	Nodes      []corev1.Node
	Pods       []corev1.Pod
	Namespaces []string

	// NodeUsage is keyed by node name, PodUsage by "namespace/name".
	NodeUsage map[string]model.Resources
	PodUsage  map[string]model.Resources

	NodeMetricsOK bool
	PodMetricsOK  bool

	MetricsError string
	Warnings     []string

	Strategies []GroupStrategy
	FetchedAt  time.Time
}

// PodKey is the lookup key used for pod metrics.
func PodKey(namespace, name string) string { return namespace + "/" + name }

type groupAccumulator struct {
	name        string
	nodes       int
	tainted     bool
	allocatable model.Resources
	requested   model.Resources
	used        model.Resources
	usageKnown  bool
}

// Aggregate turns a snapshot into the cluster model rendered by the TUI.
func Aggregate(contextName string, s Snapshot) *model.Cluster {
	return aggregate(contextName, s, true)
}

// AggregateOverview builds only the node-group level of the cluster model. The
// namespace and pod tables are left empty, so the caller can skip collecting
// the data they would need.
func AggregateOverview(contextName string, s Snapshot) *model.Cluster {
	return aggregate(contextName, s, false)
}

func aggregate(contextName string, s Snapshot, detail bool) *model.Cluster {
	strategies := s.Strategies
	if strategies == nil {
		strategies = DefaultStrategies()
	}

	cluster := &model.Cluster{
		Context:      contextName,
		MetricsError: s.MetricsError,
		Warnings:     s.Warnings,
		Pods:         map[string][]model.Pod{},
		FetchedAt:    s.FetchedAt,
	}
	cluster.UsageKnown = s.NodeMetricsOK

	groupOfNode := make(map[string]string, len(s.Nodes))
	groups := map[string]*groupAccumulator{}
	order := []string{}
	cluster.NodeGroupOf = groupOfNode

	for i := range s.Nodes {
		node := &s.Nodes[i]
		name := NodeGroupName(node, strategies)
		groupOfNode[node.Name] = name

		g, ok := groups[name]
		if !ok {
			g = &groupAccumulator{name: name, usageKnown: s.NodeMetricsOK}
			groups[name] = g
			order = append(order, name)
		}
		g.nodes++
		g.tainted = g.tainted || IsTainted(node)

		alloc := ResourcesFromList(node.Status.Allocatable)
		g.allocatable = g.allocatable.Add(alloc)
		cluster.Allocatable = cluster.Allocatable.Add(alloc)

		if s.NodeMetricsOK {
			usage := s.NodeUsage[node.Name]
			g.used = g.used.Add(usage)
			cluster.Used = cluster.Used.Add(usage)
		}
	}

	namespaces := map[string]*model.Namespace{}
	nsOrder := []string{}
	ensureNamespace := func(name string) *model.Namespace {
		ns, ok := namespaces[name]
		if !ok {
			ns = &model.Namespace{Name: name, Summary: model.Summary{UsageKnown: s.PodMetricsOK}}
			namespaces[name] = ns
			nsOrder = append(nsOrder, name)
		}
		return ns
	}
	if detail {
		for _, name := range s.Namespaces {
			ensureNamespace(name)
		}
	}

	for i := range s.Pods {
		pod := &s.Pods[i]
		if !CountsTowardsRequests(pod) {
			continue
		}

		requested := PodRequests(pod)
		cluster.Requested = cluster.Requested.Add(requested)

		if g, ok := groups[groupOfNode[pod.Spec.NodeName]]; ok {
			g.requested = g.requested.Add(requested)
		}

		if !detail {
			continue
		}

		var used model.Resources
		if s.PodMetricsOK {
			// A pod without a sample yet counts as zero usage, not as unknown.
			used = s.PodUsage[PodKey(pod.Namespace, pod.Name)]
		}

		entry := model.Pod{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Node:      pod.Spec.NodeName,
			Summary: model.Summary{
				Requested:  requested,
				Used:       used,
				UsageKnown: s.PodMetricsOK,
			},
		}
		cluster.Pods[pod.Namespace] = append(cluster.Pods[pod.Namespace], entry)

		ns := ensureNamespace(pod.Namespace)
		ns.Requested = ns.Requested.Add(requested)
		ns.Used = ns.Used.Add(used)
		ns.Pods++
	}

	if detail {
		cluster.Namespaces = make([]model.Namespace, 0, len(nsOrder))
		for _, name := range nsOrder {
			cluster.Namespaces = append(cluster.Namespaces, *namespaces[name])
		}
		model.SortNamespaces(cluster.Namespaces, model.SortWasteCPU, true)

		for _, pods := range cluster.Pods {
			model.SortPods(pods, model.SortWasteCPU, true)
		}
	}

	cluster.NodeGroups = make([]model.NodeGroup, 0, len(order))
	for _, name := range order {
		g := groups[name]
		cluster.NodeGroups = append(cluster.NodeGroups, model.NodeGroup{
			Name:        g.name,
			Nodes:       g.nodes,
			Tainted:     g.tainted,
			Allocatable: g.allocatable,
			Summary: model.Summary{
				Requested:  g.requested,
				Used:       g.used,
				UsageKnown: g.usageKnown,
			},
		})
	}
	sort.SliceStable(cluster.NodeGroups, func(i, j int) bool {
		a, b := cluster.NodeGroups[i], cluster.NodeGroups[j]
		if a.Allocatable.CPUMilli != b.Allocatable.CPUMilli {
			return a.Allocatable.CPUMilli > b.Allocatable.CPUMilli
		}
		return a.Name < b.Name
	})

	return cluster
}
