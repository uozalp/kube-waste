// Command kube-waste is a read-only terminal UI for finding wasted CPU and
// memory requests in Kubernetes clusters.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uozalp/kube-waste/internal/config"
	"github.com/uozalp/kube-waste/internal/tui"
)

func main() {
	var (
		opts       tui.Options
		configPath string
	)
	flag.StringVar(&opts.Context, "context", "", "start directly in this kubeconfig context")
	flag.StringVar(&opts.KubeconfigPath, "kubeconfig", "", "path to the kubeconfig file")
	flag.StringVar(&configPath, "config", "", "path to the config file (default "+config.DefaultPath()+")")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kube-waste:", err)
		os.Exit(1)
	}
	opts.NodeGroupLabels = cfg.NodeGroupLabels

	tui.InitColors()

	program := tea.NewProgram(tui.New(opts), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kube-waste:", err)
		os.Exit(1)
	}
}
