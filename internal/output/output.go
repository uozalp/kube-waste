// Package output renders cluster snapshots as machine-readable JSON and CSV.
// All values stay numeric; display formatting belongs to the TUI.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// Scope selects which level of the cluster snapshot is reported.
type Scope string

// The supported scopes.
const (
	ScopeOverview  Scope = "overview"
	ScopeNamespace Scope = "namespace"
	ScopePod       Scope = "pod"
)

// ParseScope validates a --scope value.
func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeOverview, ScopeNamespace, ScopePod:
		return Scope(s), nil
	}
	return "", fmt.Errorf("unknown scope %q (want overview, namespace or pod)", s)
}

// Format selects the serialization of a report.
type Format string

// The supported output formats.
const (
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
)

// ParseFormat validates an --output value.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatJSON, FormatCSV:
		return Format(s), nil
	}
	return "", fmt.Errorf("unknown output format %q (want json or csv)", s)
}

// Metric is the requested/used/waste/percent view of one resource kind.
// Used, Waste and Percent are nil when the value cannot be calculated, which
// serializes to JSON null and to an empty CSV field.
type Metric struct {
	Requested float64  `json:"requested"`
	Used      *float64 `json:"used"`
	Waste     *float64 `json:"waste"`
	Percent   *float64 `json:"percent"`
}

func (m Metric) fields() []string {
	return []string{number(&m.Requested), number(m.Used), number(m.Waste), number(m.Percent)}
}

func number(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// Group holds the CPU and memory metrics of one aggregate row.
type Group struct {
	CPU    Metric `json:"cpu"`
	Memory Metric `json:"memory"`
}

func (g Group) fields() []string { return append(g.CPU.fields(), g.Memory.fields()...) }

var metricHeader = []string{
	"cpu_requested", "cpu_used", "cpu_waste", "cpu_percent",
	"memory_requested", "memory_used", "memory_waste", "memory_percent",
}

// Report is a scope-specific result that can be written as JSON or CSV.
type Report interface {
	// Header returns the CSV column names.
	Header() []string
	// Rows returns the CSV records, without the header.
	Rows() [][]string
}

// Overview is the node-group report consumed by tmux and other automation.
type Overview struct {
	Context    string           `json:"context"`
	Timestamp  time.Time        `json:"timestamp"`
	NodeGroups map[string]Group `json:"nodegroups"`

	// order keeps the cluster's node-group ordering for CSV output.
	order []string
}

// Header implements Report.
func (o Overview) Header() []string { return append([]string{"nodegroup"}, metricHeader...) }

// Rows implements Report.
func (o Overview) Rows() [][]string {
	rows := make([][]string, 0, len(o.order))
	for _, name := range o.order {
		rows = append(rows, append([]string{name}, o.NodeGroups[name].fields()...))
	}
	return rows
}

// NamespaceRow is one namespace of the namespace report.
type NamespaceRow struct {
	Namespace string `json:"namespace"`
	Group
}

// NamespaceReport lists every namespace with its requests, usage and waste.
type NamespaceReport struct {
	Context    string         `json:"context"`
	Timestamp  time.Time      `json:"timestamp"`
	Namespaces []NamespaceRow `json:"namespaces"`
}

// Header implements Report.
func (r NamespaceReport) Header() []string { return append([]string{"namespace"}, metricHeader...) }

// Rows implements Report.
func (r NamespaceReport) Rows() [][]string {
	rows := make([][]string, 0, len(r.Namespaces))
	for _, ns := range r.Namespaces {
		rows = append(rows, append([]string{ns.Namespace}, ns.fields()...))
	}
	return rows
}

// PodRow is one pod of the pod report.
type PodRow struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Node      string `json:"node"`
	Group
}

// PodReport lists every pod with its requests, usage and waste.
type PodReport struct {
	Context   string    `json:"context"`
	Timestamp time.Time `json:"timestamp"`
	Pods      []PodRow  `json:"pods"`
}

// Header implements Report.
func (r PodReport) Header() []string {
	return append([]string{"namespace", "pod", "node"}, metricHeader...)
}

// Rows implements Report.
func (r PodReport) Rows() [][]string {
	rows := make([][]string, 0, len(r.Pods))
	for _, p := range r.Pods {
		rows = append(rows, append([]string{p.Namespace, p.Pod, p.Node}, p.fields()...))
	}
	return rows
}

// Write serializes a report in the requested format.
func Write(w io.Writer, format Format, report Report) error {
	switch format {
	case FormatCSV:
		return WriteCSV(w, report)
	case FormatJSON:
		return WriteJSON(w, report)
	}
	return fmt.Errorf("unknown output format %q", format)
}

// WriteJSON writes the report as a single JSON object.
func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(report)
}

// WriteCSV writes the report as a header row followed by one row per record.
func WriteCSV(w io.Writer, report Report) error {
	out := csv.NewWriter(w)
	if err := out.Write(report.Header()); err != nil {
		return err
	}
	if err := out.WriteAll(report.Rows()); err != nil {
		return err
	}
	out.Flush()
	return out.Error()
}
