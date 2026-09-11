// Package kube isolates all Kubernetes API access. Everything it returns is
// expressed with the application's own model types.
package kube

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/uozalp/kube-waste/internal/model"
)

// Loader resolves kubeconfig contexts and builds per-context clients.
type Loader struct {
	// KubeconfigPath overrides the normal kubeconfig discovery when set.
	KubeconfigPath string
}

func (l *Loader) rules() *clientcmd.ClientConfigLoadingRules {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if l.KubeconfigPath != "" {
		rules.ExplicitPath = l.KubeconfigPath
	}
	return rules
}

// Contexts returns every context defined in the kubeconfig.
func (l *Loader) Contexts() ([]model.ContextInfo, error) {
	raw, err := l.rules().Load()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return contextsFromConfig(raw), nil
}

func contextsFromConfig(raw *clientcmdapi.Config) []model.ContextInfo {
	out := make([]model.ContextInfo, 0, len(raw.Contexts))
	for name, kctx := range raw.Contexts {
		out = append(out, model.ContextInfo{
			Name:      name,
			Cluster:   kctx.Cluster,
			Namespace: kctx.Namespace,
			Current:   name == raw.CurrentContext,
		})
	}
	model.SortContexts(out, model.SortName, false)
	return out
}

// CurrentContext returns the kubeconfig's current-context, if any.
func (l *Loader) CurrentContext() (string, error) {
	raw, err := l.rules().Load()
	if err != nil {
		return "", fmt.Errorf("load kubeconfig: %w", err)
	}
	return raw.CurrentContext, nil
}

// Clients bundles the API clients needed for one context.
type Clients struct {
	Core    kubernetes.Interface
	Metrics metricsclient.Interface
}

// ClientsFor builds clients for the given context name. An empty name uses the
// kubeconfig's current context.
func (l *Loader) ClientsFor(contextName string) (*Clients, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(l.rules(), overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build client config for context %q: %w", contextName, err)
	}
	return clientsFromConfig(cfg)
}

func clientsFromConfig(cfg *rest.Config) (*Clients, error) {
	// Large clusters return thousands of pods; the defaults throttle hard.
	cfg.QPS = 100
	cfg.Burst = 200

	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	metrics, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create metrics client: %w", err)
	}
	return &Clients{Core: core, Metrics: metrics}, nil
}

// Probe checks whether a context is reachable and authorised, without pulling
// any cluster data.
func (l *Loader) Probe(ctx context.Context, contextName string, timeout time.Duration) error {
	clients, err := l.ClientsFor(contextName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err = clients.Core.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}
