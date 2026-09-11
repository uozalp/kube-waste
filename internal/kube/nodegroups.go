package kube

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// GroupStrategy derives a node-group name from a node. It returns false when
// the strategy does not apply, so the next strategy can be tried.
type GroupStrategy interface {
	Group(node *corev1.Node) (string, bool)
}

// NodePoolLabels are the well known node-pool labels, tried in order.
var NodePoolLabels = []string{
	"cloud.google.com/gke-nodepool",
	"eks.amazonaws.com/nodegroup",
	"karpenter.sh/nodepool",
	"kubernetes.azure.com/agentpool",
	"agentpool",
	"node.kubernetes.io/instancegroup",
	"nodepool",
	"node-pool",
	"node-group",
}

// LabelStrategy groups nodes by the first label that is present.
type LabelStrategy struct{ Labels []string }

func (s LabelStrategy) Group(node *corev1.Node) (string, bool) {
	for _, key := range s.Labels {
		if v, ok := node.Labels[key]; ok && v != "" {
			return v, true
		}
	}
	return "", false
}

const rolePrefix = "node-role.kubernetes.io/"

// RoleStrategy groups nodes by their node-role label, which is the closest
// thing to a pool on bare-metal and kubeadm clusters.
type RoleStrategy struct{}

func (RoleStrategy) Group(node *corev1.Node) (string, bool) {
	roles := make([]string, 0, 2)
	for key := range node.Labels {
		if role := strings.TrimPrefix(key, rolePrefix); role != key && role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "", false
	}
	// Deterministic name when a node carries several roles.
	sortStrings(roles)
	return strings.Join(roles, ","), true
}

// randomSuffix matches numeric or hex-looking node-name suffixes such as the
// "-12" in "worker-12" or the "-a1b2" in "gke-pool-a1b2".
var randomSuffix = regexp.MustCompile(`^[0-9]+$|^[0-9a-f]{4,10}$`)

// NamePrefixStrategy is the fallback used when no label identifies a pool. It
// strips generated suffixes from the node name.
type NamePrefixStrategy struct{}

func (NamePrefixStrategy) Group(node *corev1.Node) (string, bool) {
	parts := strings.Split(node.Name, "-")
	for len(parts) > 1 && randomSuffix.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "-"), true
}

// DefaultStrategies is the ordered strategy chain used by the collector.
// Additional detection strategies can simply be inserted here.
func DefaultStrategies() []GroupStrategy {
	return []GroupStrategy{
		LabelStrategy{Labels: NodePoolLabels},
		RoleStrategy{},
		NamePrefixStrategy{},
	}
}

// Strategies puts the user-configured node labels in front of the built-in
// chain, so a cluster-specific label such as node.example.com/workload wins.
func Strategies(labels []string) []GroupStrategy {
	if len(labels) == 0 {
		return DefaultStrategies()
	}
	return append([]GroupStrategy{LabelStrategy{Labels: labels}}, DefaultStrategies()...)
}

// NodeGroupName applies the strategies in order and returns the first match.
func NodeGroupName(node *corev1.Node, strategies []GroupStrategy) string {
	for _, s := range strategies {
		if name, ok := s.Group(node); ok && name != "" {
			return name
		}
	}
	return node.Name
}

// IsTainted reports whether the node repels ordinary workloads.
func IsTainted(node *corev1.Node) bool {
	for _, t := range node.Spec.Taints {
		switch t.Effect {
		case corev1.TaintEffectNoSchedule, corev1.TaintEffectNoExecute:
			return true
		}
	}
	return false
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
