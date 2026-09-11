package kube

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/uozalp/kube-waste/internal/model"
)

// ResourcesFromList converts a Kubernetes resource list into the internal
// numeric representation: CPU in milli-cores, memory in bytes.
func ResourcesFromList(list corev1.ResourceList) model.Resources {
	var r model.Resources
	if q, ok := list[corev1.ResourceCPU]; ok {
		r.CPUMilli = q.MilliValue()
	}
	if q, ok := list[corev1.ResourceMemory]; ok {
		r.MemBytes = q.Value()
	}
	return r
}

// PodRequests returns the effective resource requests of a pod.
//
// Regular containers run concurrently so their requests add up. Restartable
// init containers (sidecars) run alongside them and add up as well. Ordinary
// init containers run one at a time before the regular containers, so only the
// largest one counts - together with the sidecars already started before it.
func PodRequests(pod *corev1.Pod) model.Resources {
	var sidecars, maxInit model.Resources

	for _, c := range pod.Spec.InitContainers {
		r := ResourcesFromList(c.Resources.Requests)
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			sidecars = sidecars.Add(r)
			continue
		}
		maxInit = model.MaxResources(maxInit, sidecars.Add(r))
	}

	running := sidecars
	for _, c := range pod.Spec.Containers {
		running = running.Add(ResourcesFromList(c.Resources.Requests))
	}

	total := model.MaxResources(running, maxInit)
	if pod.Spec.Overhead != nil {
		total = total.Add(ResourcesFromList(pod.Spec.Overhead))
	}
	return total
}

// CountsTowardsRequests reports whether a pod currently reserves capacity on a
// node. Completed and failed pods release their requests.
func CountsTowardsRequests(pod *corev1.Pod) bool {
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return false
	}
	// Unscheduled pods reserve nothing yet.
	return pod.Spec.NodeName != ""
}
