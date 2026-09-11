package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/uozalp/kube-waste/internal/model"
)

// DefaultTimeout bounds a full cluster collection.
const DefaultTimeout = 30 * time.Second

// listChunkSize keeps single responses small on clusters with many pods.
const listChunkSize = 500

// Collector fetches everything the TUI needs for one context in as few bulk
// requests as possible.
type Collector struct {
	Clients    *Clients
	Timeout    time.Duration
	Strategies []GroupStrategy
}

// NewCollector builds a collector for a kubeconfig context.
func NewCollector(loader *Loader, contextName string) (*Collector, error) {
	clients, err := loader.ClientsFor(contextName)
	if err != nil {
		return nil, err
	}
	return &Collector{Clients: clients, Timeout: DefaultTimeout}, nil
}

// Collect returns a complete cluster snapshot. Missing metrics are reported
// through the cluster model instead of as an error.
func (c *Collector) Collect(ctx context.Context, contextName string) (*model.Cluster, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		snap     = Snapshot{FetchedAt: time.Now(), Strategies: c.Strategies}
		nodeErr  error
		podErr   error
		metricsE []string
	)

	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	run(func() {
		nodes, err := c.listNodes(ctx)
		mu.Lock()
		defer mu.Unlock()
		snap.Nodes, nodeErr = nodes, err
	})

	run(func() {
		pods, err := c.listPods(ctx)
		mu.Lock()
		defer mu.Unlock()
		snap.Pods, podErr = pods, err
	})

	run(func() {
		names, err := c.listNamespaces(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			// Namespaces are derived from the pod list in this case.
			snap.Warnings = append(snap.Warnings, fmt.Sprintf("namespaces: %v", err))
			return
		}
		snap.Namespaces = names
	})

	run(func() {
		usage, err := c.nodeMetrics(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			metricsE = append(metricsE, fmt.Sprintf("node metrics: %v", err))
			return
		}
		snap.NodeUsage, snap.NodeMetricsOK = usage, true
	})

	run(func() {
		usage, err := c.podMetrics(ctx)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			metricsE = append(metricsE, fmt.Sprintf("pod metrics: %v", err))
			return
		}
		snap.PodUsage, snap.PodMetricsOK = usage, true
	})

	wg.Wait()

	if nodeErr != nil {
		return nil, fmt.Errorf("list nodes: %w", nodeErr)
	}
	if podErr != nil {
		return nil, fmt.Errorf("list pods: %w", podErr)
	}
	if len(metricsE) > 0 {
		snap.MetricsError = "metrics.k8s.io API could not be queried (" + strings.Join(metricsE, "; ") + ")"
	}

	return Aggregate(contextName, snap), nil
}

func (c *Collector) listNodes(ctx context.Context) ([]corev1.Node, error) {
	var out []corev1.Node
	opts := metav1.ListOptions{Limit: listChunkSize}
	for {
		page, err := c.Clients.Core.CoreV1().Nodes().List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.Continue == "" {
			return out, nil
		}
		opts.Continue = page.Continue
	}
}

func (c *Collector) listPods(ctx context.Context) ([]corev1.Pod, error) {
	var out []corev1.Pod
	opts := metav1.ListOptions{Limit: listChunkSize}
	for {
		page, err := c.Clients.Core.CoreV1().Pods(metav1.NamespaceAll).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.Continue == "" {
			return out, nil
		}
		opts.Continue = page.Continue
	}
}

func (c *Collector) listNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.Clients.Core.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	return names, nil
}

func (c *Collector) nodeMetrics(ctx context.Context) (map[string]model.Resources, error) {
	list, err := c.Clients.Metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.Resources, len(list.Items))
	for _, m := range list.Items {
		out[m.Name] = ResourcesFromList(m.Usage)
	}
	return out, nil
}

func (c *Collector) podMetrics(ctx context.Context) (map[string]model.Resources, error) {
	out := map[string]model.Resources{}
	opts := metav1.ListOptions{Limit: listChunkSize}
	for {
		page, err := c.Clients.Metrics.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, m := range page.Items {
			var total model.Resources
			for _, container := range m.Containers {
				total = total.Add(ResourcesFromList(container.Usage))
			}
			out[PodKey(m.Namespace, m.Name)] = total
		}
		if page.Continue == "" {
			return out, nil
		}
		opts.Continue = page.Continue
	}
}
