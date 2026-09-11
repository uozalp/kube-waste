package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func requests(cpu, mem string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{Requests: corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(mem),
	}}
}

func TestResourcesFromList(t *testing.T) {
	got := ResourcesFromList(corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("8"),
		corev1.ResourceMemory: resource.MustParse("9Gi"),
	})
	if got.CPUMilli != 8000 {
		t.Errorf("cpu = %d milli, want 8000", got.CPUMilli)
	}
	if want := int64(9) << 30; got.MemBytes != want {
		t.Errorf("memory = %d bytes, want %d", got.MemBytes, want)
	}
}

func TestPodRequestsSumsContainers(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "api", Resources: requests("2", "4Gi")},
		{Name: "sidecar", Resources: requests("500m", "512Mi")},
	}}}

	got := PodRequests(pod)
	if got.CPUMilli != 2500 {
		t.Errorf("cpu = %d milli, want 2500", got.CPUMilli)
	}
	if want := int64(4)<<30 + int64(512)<<20; got.MemBytes != want {
		t.Errorf("memory = %d bytes, want %d", got.MemBytes, want)
	}
}

func TestPodRequestsUsesLargestInitContainer(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{
		InitContainers: []corev1.Container{
			{Name: "migrate", Resources: requests("4", "1Gi")},
			{Name: "seed", Resources: requests("1", "8Gi")},
		},
		Containers: []corev1.Container{
			{Name: "api", Resources: requests("2", "4Gi")},
		},
	}}

	got := PodRequests(pod)
	// CPU is dominated by the init container, memory by the other one.
	if got.CPUMilli != 4000 {
		t.Errorf("cpu = %d milli, want 4000", got.CPUMilli)
	}
	if want := int64(8) << 30; got.MemBytes != want {
		t.Errorf("memory = %d bytes, want %d", got.MemBytes, want)
	}
}

func TestPodRequestsCountsSidecarsOnce(t *testing.T) {
	always := corev1.ContainerRestartPolicyAlways
	pod := &corev1.Pod{Spec: corev1.PodSpec{
		InitContainers: []corev1.Container{
			{Name: "proxy", RestartPolicy: &always, Resources: requests("500m", "256Mi")},
			{Name: "migrate", Resources: requests("1", "1Gi")},
		},
		Containers: []corev1.Container{
			{Name: "api", Resources: requests("2", "4Gi")},
		},
	}}

	got := PodRequests(pod)
	// Sidecar plus app containers (2.5) beats sidecar plus init container (1.5).
	if got.CPUMilli != 2500 {
		t.Errorf("cpu = %d milli, want 2500", got.CPUMilli)
	}
	if want := int64(4)<<30 + int64(256)<<20; got.MemBytes != want {
		t.Errorf("memory = %d bytes, want %d", got.MemBytes, want)
	}
}

func TestPodRequestsIncludesOverhead(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{
		Containers: []corev1.Container{{Name: "api", Resources: requests("1", "1Gi")}},
		Overhead: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("250m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}}

	got := PodRequests(pod)
	if got.CPUMilli != 1250 {
		t.Errorf("cpu = %d milli, want 1250", got.CPUMilli)
	}
	if want := int64(1)<<30 + int64(128)<<20; got.MemBytes != want {
		t.Errorf("memory = %d bytes, want %d", got.MemBytes, want)
	}
}

func TestCountsTowardsRequests(t *testing.T) {
	cases := []struct {
		name  string
		phase corev1.PodPhase
		node  string
		want  bool
	}{
		{"running", corev1.PodRunning, "worker-1", true},
		{"scheduled pending", corev1.PodPending, "worker-1", true},
		{"unscheduled pending", corev1.PodPending, "", false},
		{"succeeded", corev1.PodSucceeded, "worker-1", false},
		{"failed", corev1.PodFailed, "worker-1", false},
	}
	for _, c := range cases {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: c.name},
			Spec:       corev1.PodSpec{NodeName: c.node},
			Status:     corev1.PodStatus{Phase: c.phase},
		}
		if got := CountsTowardsRequests(pod); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
