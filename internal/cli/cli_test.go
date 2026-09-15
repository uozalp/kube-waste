package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uozalp/kube-waste/internal/model"
	"github.com/uozalp/kube-waste/internal/output"
)

func testCluster() *model.Cluster {
	return &model.Cluster{
		Context:   "test",
		FetchedAt: time.Date(2026, 9, 11, 21, 0, 0, 0, time.UTC),
		NodeGroups: []model.NodeGroup{{
			Name: "worker",
			Summary: model.Summary{
				Requested:  model.Resources{CPUMilli: 32000, MemBytes: 64 << 30},
				Used:       model.Resources{CPUMilli: 8000, MemBytes: 16 << 30},
				UsageKnown: true,
			},
		}},
		Pods:        map[string][]model.Pod{},
		NodeGroupOf: map[string]string{},
	}
}

func TestEnabled(t *testing.T) {
	cases := []struct {
		opts Options
		want bool
	}{
		{Options{}, false},
		{Options{Context: "test"}, false},
		{Options{NodeGroup: "worker"}, false},
		{Options{Scope: "overview"}, true},
		{Options{Output: "json"}, true},
	}
	for _, tc := range cases {
		if got := tc.opts.Enabled(); got != tc.want {
			t.Fatalf("%+v Enabled() = %v, want %v", tc.opts, got, tc.want)
		}
	}
}

func TestResolveDefaults(t *testing.T) {
	scope, format, err := Options{Output: "csv"}.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if scope != output.ScopeOverview || format != output.FormatCSV {
		t.Fatalf("got %s/%s, want overview/csv", scope, format)
	}

	scope, format, err = Options{Scope: "pod"}.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if scope != output.ScopePod || format != output.FormatJSON {
		t.Fatalf("got %s/%s, want pod/json", scope, format)
	}
}

func TestRenderWritesOnlyJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, output.ScopeOverview, output.FormatJSON, testCluster(), ""); err != nil {
		t.Fatalf("Render: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not plain JSON: %v\n%s", err, buf.String())
	}
	if doc["context"] != "test" {
		t.Fatalf("context = %v", doc["context"])
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("stdout carries extra lines: %q", buf.String())
	}
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Options{Scope: "nodes", Output: "json"}, &stdout, &stderr)

	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", stdout.String())
	}
	if !strings.Contains(stderr.String(), "nodes") {
		t.Fatalf("stderr = %q, want the rejected scope", stderr.String())
	}
}

func TestRunUnknownFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), Options{Scope: "overview", Output: "yaml"}, &stdout, &stderr); code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", stdout.String())
	}
}

func TestRunCollectionFailure(t *testing.T) {
	opts := Options{
		KubeconfigPath: filepath.Join(t.TempDir(), "missing-kubeconfig"),
		Context:        "test",
		Scope:          "overview",
		Output:         "json",
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), opts, &stdout, &stderr)

	if code != ExitError {
		t.Fatalf("exit code = %d, want %d", code, ExitError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing on failure", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want a diagnostic")
	}
}

func TestRunUnknownNodeGroupFails(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, output.ScopeOverview, output.FormatJSON, testCluster(), "nope")
	if err == nil {
		t.Fatal("want an error for an unknown node group")
	}
	if buf.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing", buf.String())
	}
}
