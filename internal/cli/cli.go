// Package cli implements the non-interactive modes of kube-waste: a single
// collection rendered to stdout as JSON or CSV, with diagnostics on stderr.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/uozalp/kube-waste/internal/kube"
	"github.com/uozalp/kube-waste/internal/model"
	"github.com/uozalp/kube-waste/internal/output"
)

// Exit codes returned by Run.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Options are the command-line flags of the non-interactive modes.
type Options struct {
	KubeconfigPath string
	Context        string
	Scope          string
	Output         string
	NodeGroup      string

	// NodeGroupLabels come from the config file and take precedence over the
	// built-in node-group detection, exactly as in the TUI.
	NodeGroupLabels []string
}

// Enabled reports whether the flags ask for non-interactive output instead of
// the TUI.
func (o Options) Enabled() bool { return o.Scope != "" || o.Output != "" }

// Run performs one collection and writes the report to stdout. It returns the
// process exit code; every error is written to stderr only.
func Run(ctx context.Context, opts Options, stdout, stderr io.Writer) int {
	scope, format, err := opts.resolve()
	if err != nil {
		fmt.Fprintln(stderr, "kube-waste:", err)
		return ExitUsage
	}

	cluster, err := Collect(ctx, opts, scope)
	if err != nil {
		fmt.Fprintln(stderr, "kube-waste:", err)
		return ExitError
	}

	if err := Render(stdout, scope, format, cluster, opts.NodeGroup); err != nil {
		fmt.Fprintln(stderr, "kube-waste:", err)
		return ExitError
	}
	return ExitOK
}

// resolve validates the scope and output flags. Either one on its own implies
// the usual default for the other.
func (o Options) resolve() (output.Scope, output.Format, error) {
	scopeFlag, outputFlag := o.Scope, o.Output
	if scopeFlag == "" {
		scopeFlag = string(output.ScopeOverview)
	}
	if outputFlag == "" {
		outputFlag = string(output.FormatJSON)
	}

	scope, err := output.ParseScope(scopeFlag)
	if err != nil {
		return "", "", err
	}
	format, err := output.ParseFormat(outputFlag)
	if err != nil {
		return "", "", err
	}
	return scope, format, nil
}

// Collect fetches exactly the data the scope needs. The overview scope uses the
// collector's fast path, which skips pod metrics and the namespace list.
func Collect(ctx context.Context, opts Options, scope output.Scope) (*model.Cluster, error) {
	loader := &kube.Loader{KubeconfigPath: opts.KubeconfigPath}

	contextName := opts.Context
	if contextName == "" {
		current, err := loader.CurrentContext()
		if err != nil {
			return nil, err
		}
		contextName = current
	}

	collector, err := kube.NewCollector(loader, contextName)
	if err != nil {
		return nil, err
	}
	collector.Strategies = kube.Strategies(opts.NodeGroupLabels)

	if scope == output.ScopeOverview {
		return collector.CollectOverview(ctx, contextName)
	}
	return collector.Collect(ctx, contextName)
}

// Render writes the cluster snapshot at the given scope and format.
func Render(w io.Writer, scope output.Scope, format output.Format, cluster *model.Cluster, nodeGroup string) error {
	report, err := output.Build(scope, cluster, nodeGroup)
	if err != nil {
		return err
	}
	return output.Write(w, format, report)
}
