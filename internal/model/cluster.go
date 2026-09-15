package model

import "time"

// ContextInfo describes a single kubeconfig context on the context screen.
type ContextInfo struct {
	Name      string
	Cluster   string
	Namespace string
	Current   bool

	// Status mirrors the reachability probe result so the table can sort on it.
	Status string
}

// Namespace is one row of the namespace table.
type Namespace struct {
	Name string
	Summary
	Pods int
}

// Pod is one row of the pod table.
type Pod struct {
	Name      string
	Namespace string
	Node      string
	Summary
}

// NodeGroup aggregates the nodes of a node pool.
type NodeGroup struct {
	Name        string
	Nodes       int
	Tainted     bool
	Allocatable Resources
	Summary
}

// Cluster is the complete snapshot the TUI renders for one kubeconfig context.
type Cluster struct {
	Context     string
	Allocatable Resources
	Summary

	// MetricsError explains why usage data is missing. It is empty when
	// metrics.k8s.io was queried successfully.
	MetricsError string

	NodeGroups []NodeGroup
	Namespaces []Namespace

	// Pods are indexed by namespace so navigating does not re-query the API.
	Pods map[string][]Pod

	// NodeGroupOf maps a node name to its node-group name.
	NodeGroupOf map[string]string

	// Warnings collects non-fatal problems, e.g. a partially failed list.
	Warnings []string

	FetchedAt time.Time
}

// NamespacePods returns the cached pods of a namespace.
func (c *Cluster) NamespacePods(namespace string) []Pod {
	if c == nil {
		return nil
	}
	return c.Pods[namespace]
}
