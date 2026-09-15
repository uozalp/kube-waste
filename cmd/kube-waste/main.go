// Command kube-waste is a read-only terminal UI for finding wasted CPU and
// memory requests in Kubernetes clusters. With --scope or --output it instead
// prints a single machine-readable report to stdout.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uozalp/kube-waste/internal/cli"
	"github.com/uozalp/kube-waste/internal/config"
	"github.com/uozalp/kube-waste/internal/tui"
)

func main() {
	var (
		opts       tui.Options
		cliOpts    cli.Options
		configPath string
	)
	flag.StringVar(&opts.Context, "context", "", "start directly in this kubeconfig context")
	flag.StringVar(&opts.KubeconfigPath, "kubeconfig", "", "path to the kubeconfig file")
	flag.StringVar(&configPath, "config", "", "path to the config file (default "+config.DefaultPath()+")")
	flag.StringVar(&cliOpts.Scope, "scope", "", "print data instead of the TUI: overview, namespace or pod")
	flag.StringVar(&cliOpts.Output, "output", "", "output format for --scope: json or csv (default json)")
	flag.StringVar(&cliOpts.NodeGroup, "nodegroup", "", "limit --scope output to a single node group")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kube-waste:", err)
		os.Exit(1)
	}
	opts.NodeGroupLabels = cfg.NodeGroupLabels

	if cliOpts.Enabled() {
		cliOpts.Context = opts.Context
		cliOpts.KubeconfigPath = opts.KubeconfigPath
		cliOpts.NodeGroupLabels = cfg.NodeGroupLabels
		os.Exit(cli.Run(context.Background(), cliOpts, os.Stdout, os.Stderr))
	}

	tui.InitColors()

	program := tea.NewProgram(tui.New(opts), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kube-waste:", err)
		os.Exit(1)
	}
}
