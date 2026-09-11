// Package tui implements the Bubble Tea application. It only ever reads data
// that the kube package has already translated into model types.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uozalp/kube-waste/internal/kube"
	"github.com/uozalp/kube-waste/internal/model"
)

type screen int

const (
	screenContexts screen = iota
	screenCluster
	screenPods
)

const probeTimeout = 8 * time.Second

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// tableState is the per-view cursor, scroll, sort and filter state.
type tableState struct {
	cursor   int
	offset   int
	sortKey  model.SortKey
	sortDesc bool
	filter   string
}

// Options configures the application at start-up.
type Options struct {
	KubeconfigPath string
	Context        string

	// NodeGroupLabels are node label keys that take precedence over the
	// built-in node-group detection.
	NodeGroupLabels []string
}

// Model is the root Bubble Tea model.
type Model struct {
	loader     *kube.Loader
	opts       Options
	strategies []kube.GroupStrategy

	width  int
	height int

	screen  screen
	pinned  bool // started with --context, contexts screen is skipped
	quiting bool

	contexts   []model.ContextInfo
	probes     map[string]string
	contextErr string

	cluster     *model.Cluster
	contextName string
	namespace   string

	loading      bool
	spinnerIndex int
	errMsg       string

	ctxTable tableState
	nsTable  tableState
	podTable tableState

	filtering  bool
	showHelp   bool
	showGroups bool
}

// New builds the root model.
func New(opts Options) Model {
	m := Model{
		loader:     &kube.Loader{KubeconfigPath: opts.KubeconfigPath},
		opts:       opts,
		strategies: kube.Strategies(opts.NodeGroupLabels),
		probes:     map[string]string{},
		showGroups: true,
		ctxTable:   tableState{sortKey: model.SortName},
		nsTable:    tableState{sortKey: model.SortWasteCPU, sortDesc: true},
		podTable:   tableState{sortKey: model.SortWasteCPU, sortDesc: true},
	}
	if opts.Context != "" {
		m.pinned = true
		m.screen = screenCluster
		m.contextName = opts.Context
	}
	return m
}

// Init starts the first data load.
func (m Model) Init() tea.Cmd {
	if m.pinned {
		return tea.Batch(m.loadClusterCmd(m.contextName), spinnerTick())
	}
	return loadContextsCmd(m.loader)
}

type contextsMsg struct {
	items []model.ContextInfo
	err   error
}

type probeMsg struct {
	name   string
	status string
}

type clusterMsg struct {
	context string
	cluster *model.Cluster
	err     error
}

type spinnerMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return spinnerMsg{} })
}

func loadContextsCmd(loader *kube.Loader) tea.Cmd {
	return func() tea.Msg {
		items, err := loader.Contexts()
		return contextsMsg{items: items, err: err}
	}
}

func probeContextCmd(loader *kube.Loader, name string) tea.Cmd {
	return func() tea.Msg {
		if err := loader.Probe(context.Background(), name, probeTimeout); err != nil {
			return probeMsg{name: name, status: "Error: " + shortError(err)}
		}
		return probeMsg{name: name, status: "Ready"}
	}
}

func (m Model) loadClusterCmd(contextName string) tea.Cmd {
	loader, strategies := m.loader, m.strategies
	return func() tea.Msg {
		collector, err := kube.NewCollector(loader, contextName)
		if err != nil {
			return clusterMsg{context: contextName, err: err}
		}
		collector.Strategies = strategies
		cluster, err := collector.Collect(context.Background(), contextName)
		return clusterMsg{context: contextName, cluster: cluster, err: err}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinnerMsg:
		if !m.loading {
			return m, nil
		}
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		return m, spinnerTick()

	case contextsMsg:
		if msg.err != nil {
			m.contextErr = msg.err.Error()
			return m, nil
		}
		m.contexts = msg.items
		m.applyContextSort()
		cmds := make([]tea.Cmd, 0, len(msg.items))
		for _, c := range msg.items {
			cmds = append(cmds, probeContextCmd(m.loader, c.Name))
		}
		return m, tea.Batch(cmds...)

	case probeMsg:
		m.setProbe(msg.name, msg.status)
		return m, nil

	case clusterMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.setProbe(msg.context, "Error: "+shortError(msg.err))
			if m.cluster == nil || m.cluster.Context != msg.context {
				// Nothing to show for this context; stay on the context list.
				if !m.pinned {
					m.screen = screenContexts
				}
			}
			return m, nil
		}
		m.errMsg = ""
		m.cluster = msg.cluster
		m.contextName = msg.context
		m.setProbe(msg.context, "Ready")
		m.applyNamespaceSort()
		m.applyPodSort()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		return m.handleFilterKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "esc":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if st := m.state(); st.filter != "" {
			st.filter = ""
			st.cursor, st.offset = 0, 0
			return m, nil
		}
		return m.back()
	case "up", "k":
		m.moveCursor(-1)
		return m, nil
	case "down", "j":
		m.moveCursor(1)
		return m, nil
	case "left", "h":
		m.cycleSort(-1)
		return m, nil
	case "right", "l":
		m.cycleSort(1)
		return m, nil
	case "pgup", "ctrl+b":
		m.moveCursor(-m.pageSize())
		return m, nil
	case "pgdown", "ctrl+f":
		m.moveCursor(m.pageSize())
		return m, nil
	case "home", "g":
		m.setCursor(0)
		return m, nil
	case "end", "G":
		m.setCursor(m.rowCount() - 1)
		return m, nil
	case "enter":
		return m.open()
	case "r":
		return m.refresh()
	case "/":
		m.filtering = true
		m.showHelp = false
		return m, nil
	case "s":
		m.cycleSort(1)
		return m, nil
	case "S":
		m.toggleSortDirection()
		return m, nil
	case "c":
		m.setSort(model.SortWasteCPU)
		return m, nil
	case "m":
		m.setSort(model.SortWasteMem)
		return m, nil
	case "n":
		m.showGroups = !m.showGroups
		return m, nil
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	st := m.state()
	switch msg.String() {
	case "esc":
		st.filter = ""
		m.filtering = false
	case "enter":
		m.filtering = false
	case "backspace":
		if n := len(st.filter); n > 0 {
			st.filter = st.filter[:n-1]
		}
	case "ctrl+u":
		st.filter = ""
	default:
		if msg.Type == tea.KeyRunes {
			st.filter += string(msg.Runes)
		}
	}
	st.cursor, st.offset = 0, 0
	m.setState(*st)
	return m, nil
}

func (m Model) back() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenPods:
		m.screen = screenCluster
		m.namespace = ""
		m.podTable = tableState{sortKey: m.podTable.sortKey, sortDesc: m.podTable.sortDesc}
	case screenCluster:
		if m.pinned {
			return m, tea.Quit
		}
		m.screen = screenContexts
		m.errMsg = ""
	default:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) open() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenContexts:
		rows := m.visibleContexts()
		if len(rows) == 0 {
			return m, nil
		}
		name := rows[clamp(m.ctxTable.cursor, 0, len(rows)-1)].Name
		m.contextName = name
		m.screen = screenCluster
		m.cluster = nil
		m.errMsg = ""
		m.loading = true
		m.nsTable.cursor, m.nsTable.offset, m.nsTable.filter = 0, 0, ""
		return m, tea.Batch(m.loadClusterCmd(name), spinnerTick())

	case screenCluster:
		rows := m.visibleNamespaces()
		if len(rows) == 0 {
			return m, nil
		}
		m.namespace = rows[clamp(m.nsTable.cursor, 0, len(rows)-1)].Name
		m.screen = screenPods
		m.podTable.cursor, m.podTable.offset, m.podTable.filter = 0, 0, ""
		m.applyPodSort()
	}
	return m, nil
}

func (m Model) refresh() (tea.Model, tea.Cmd) {
	if m.screen == screenContexts {
		return m, loadContextsCmd(m.loader)
	}
	if m.loading {
		return m, nil
	}
	m.loading = true
	m.errMsg = ""
	return m, tea.Batch(m.loadClusterCmd(m.contextName), spinnerTick())
}

func (m *Model) state() *tableState {
	switch m.screen {
	case screenContexts:
		return &m.ctxTable
	case screenPods:
		return &m.podTable
	default:
		return &m.nsTable
	}
}

func (m *Model) setState(st tableState) { *m.state() = st }

func (m Model) rowCount() int {
	switch m.screen {
	case screenContexts:
		return len(m.visibleContexts())
	case screenPods:
		return len(m.visiblePods())
	default:
		return len(m.visibleNamespaces())
	}
}

func (m *Model) moveCursor(delta int) { m.setCursor(m.state().cursor + delta) }

func (m *Model) setCursor(idx int) {
	st := m.state()
	total := m.rowCount()
	st.cursor = clamp(idx, 0, max(0, total-1))
	st.offset = scrollOffset(st.offset, st.cursor, m.tableHeight()-1, total)
}

func (m *Model) pageSize() int { return max(1, m.tableHeight()-1) }

func shortError(err error) string {
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	if len(msg) > 80 {
		msg = msg[:77] + "…"
	}
	return msg
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m Model) spinner() string {
	if !m.loading {
		return ""
	}
	return fmt.Sprintf("%s loading", spinnerFrames[m.spinnerIndex])
}
