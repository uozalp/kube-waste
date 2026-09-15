package output

import (
	"fmt"
	"sort"

	"github.com/uozalp/kube-waste/internal/model"
)

const (
	milliPerCore = 1000.0
	bytesPerGibi = 1 << 30
)

// NewGroup converts a summary into numeric CPU (cores) and memory (GiB)
// metrics. Waste is requested minus used and is never clamped, so usage above
// the request stays negative.
func NewGroup(s model.Summary) Group {
	return Group{
		CPU:    newMetric(s.Requested.CPUMilli, s.Used.CPUMilli, s.UsageKnown, milliPerCore),
		Memory: newMetric(s.Requested.MemBytes, s.Used.MemBytes, s.UsageKnown, bytesPerGibi),
	}
}

// NewNodeGroup is NewGroup plus the allocatable capacity of the group's nodes.
func NewNodeGroup(g model.NodeGroup) NodeGroup {
	base := NewGroup(g.Summary)
	return NodeGroup{
		Nodes: g.Nodes,
		CPU: NodeGroupMetric{
			Allocatable: float64(g.Allocatable.CPUMilli) / milliPerCore,
			Metric:      base.CPU,
		},
		Memory: NodeGroupMetric{
			Allocatable: float64(g.Allocatable.MemBytes) / bytesPerGibi,
			Metric:      base.Memory,
		},
	}
}

func newMetric(requested, used int64, usageKnown bool, scale float64) Metric {
	m := Metric{Requested: float64(requested) / scale}
	if !usageKnown {
		return m
	}
	usedValue := float64(used) / scale
	wasteValue := float64(requested-used) / scale
	m.Used, m.Waste = &usedValue, &wasteValue
	if percent, ok := model.Percent(used, requested); ok {
		m.Percent = &percent
	}
	return m
}

// UnknownNodeGroupError is returned when --nodegroup names a group the cluster
// does not have.
type UnknownNodeGroupError struct{ Name string }

func (e UnknownNodeGroupError) Error() string {
	return fmt.Sprintf("unknown node group %q", e.Name)
}

func checkNodeGroup(c *model.Cluster, nodeGroup string) error {
	if nodeGroup == "" {
		return nil
	}
	for _, g := range c.NodeGroups {
		if g.Name == nodeGroup {
			return nil
		}
	}
	return UnknownNodeGroupError{Name: nodeGroup}
}

// BuildOverview reports every node group, or only nodeGroup when it is set.
func BuildOverview(c *model.Cluster, nodeGroup string) (Overview, error) {
	if err := checkNodeGroup(c, nodeGroup); err != nil {
		return Overview{}, err
	}

	out := Overview{
		Context:    c.Context,
		Timestamp:  c.FetchedAt.UTC(),
		NodeGroups: map[string]NodeGroup{},
	}
	for _, g := range c.NodeGroups {
		if nodeGroup != "" && g.Name != nodeGroup {
			continue
		}
		out.NodeGroups[g.Name] = NewNodeGroup(g)
		out.order = append(out.order, g.Name)
	}
	return out, nil
}

// BuildNamespaces reports one row per namespace. With nodeGroup set, the rows
// are re-aggregated from only the pods scheduled onto that node group.
func BuildNamespaces(c *model.Cluster, nodeGroup string) (NamespaceReport, error) {
	if err := checkNodeGroup(c, nodeGroup); err != nil {
		return NamespaceReport{}, err
	}

	out := NamespaceReport{
		Context:    c.Context,
		Timestamp:  c.FetchedAt.UTC(),
		Namespaces: []NamespaceRow{},
	}

	if nodeGroup == "" {
		names := make([]string, 0, len(c.Namespaces))
		summaries := make(map[string]model.Summary, len(c.Namespaces))
		for _, ns := range c.Namespaces {
			names = append(names, ns.Name)
			summaries[ns.Name] = ns.Summary
		}
		sort.Strings(names)
		for _, name := range names {
			out.Namespaces = append(out.Namespaces, NamespaceRow{Namespace: name, Group: NewGroup(summaries[name])})
		}
		return out, nil
	}

	summaries := map[string]model.Summary{}
	names := []string{}
	for _, pod := range selectPods(c, nodeGroup) {
		if _, ok := summaries[pod.Namespace]; !ok {
			names = append(names, pod.Namespace)
			summaries[pod.Namespace] = model.Summary{UsageKnown: pod.UsageKnown}
		}
		summaries[pod.Namespace] = summaries[pod.Namespace].Add(pod.Summary)
	}
	sort.Strings(names)
	for _, name := range names {
		out.Namespaces = append(out.Namespaces, NamespaceRow{Namespace: name, Group: NewGroup(summaries[name])})
	}
	return out, nil
}

// BuildPods reports one row per pod, optionally limited to a node group.
func BuildPods(c *model.Cluster, nodeGroup string) (PodReport, error) {
	if err := checkNodeGroup(c, nodeGroup); err != nil {
		return PodReport{}, err
	}

	out := PodReport{
		Context:   c.Context,
		Timestamp: c.FetchedAt.UTC(),
		Pods:      []PodRow{},
	}
	for _, pod := range selectPods(c, nodeGroup) {
		out.Pods = append(out.Pods, PodRow{
			Namespace: pod.Namespace,
			Pod:       pod.Name,
			Node:      pod.Node,
			Group:     NewGroup(pod.Summary),
		})
	}
	return out, nil
}

// selectPods returns the pods of the cluster in a stable namespace/name order,
// keeping only those on nodeGroup when it is set.
func selectPods(c *model.Cluster, nodeGroup string) []model.Pod {
	namespaces := make([]string, 0, len(c.Pods))
	for name := range c.Pods {
		namespaces = append(namespaces, name)
	}
	sort.Strings(namespaces)

	var out []model.Pod
	for _, ns := range namespaces {
		pods := append([]model.Pod(nil), c.Pods[ns]...)
		sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })
		for _, pod := range pods {
			if nodeGroup != "" && c.NodeGroupOf[pod.Node] != nodeGroup {
				continue
			}
			out = append(out, pod)
		}
	}
	return out
}

// Build renders the cluster at the requested scope.
func Build(scope Scope, c *model.Cluster, nodeGroup string) (Report, error) {
	switch scope {
	case ScopeOverview:
		return BuildOverview(c, nodeGroup)
	case ScopeNamespace:
		return BuildNamespaces(c, nodeGroup)
	case ScopePod:
		return BuildPods(c, nodeGroup)
	}
	return nil, fmt.Errorf("unknown scope %q", scope)
}
