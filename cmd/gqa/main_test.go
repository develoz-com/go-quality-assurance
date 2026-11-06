package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/develoz-com/go-quality-assurance/internal/check"
	"github.com/develoz-com/go-quality-assurance/internal/config"
)

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		qa   string
		want int
	}{
		{name: "no command", want: exitUsage},
		{name: "help", args: []string{"help"}, want: exitOK},
		{name: "version", args: []string{"version"}, want: exitOK},
		{name: "unknown command", args: []string{"lint"}, want: exitUsage},
		{name: "invalid configuration", args: []string{"format"}, qa: "coverge:\n  minimum: 90\n", want: exitUsage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if tt.qa != "" {
				if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(tt.qa), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
			}

			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Fatalf("run(%v) = %d, want %d\nstdout: %s\nstderr: %s", tt.args, got, tt.want, stdout.String(), stderr.String())
			}
		})
	}
}

func TestUsageListsCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"help"}, &stdout, &stderr)

	for _, command := range []string{"format", "vet", "static", "vuln", "test", "coverage", "ci"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("usage does not list %q:\n%s", command, stdout.String())
		}
	}
}

func TestRunAllStopsAtFirstFailure(t *testing.T) {
	calls := 0
	gates := []gate{
		{name: "first", run: func(context.Context) error {
			calls++
			return &check.FindingError{Check: "first"}
		}},
		{name: "second", run: func(context.Context) error {
			calls++
			return nil
		}},
	}

	var stdout, stderr bytes.Buffer
	if got := runAll(gates, &stdout, &stderr); got != exitFindings {
		t.Fatalf("runAll() = %d, want %d", got, exitFindings)
	}
	if calls != 1 {
		t.Fatalf("ran %d gates, want 1", calls)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "finding", err: &check.FindingError{Check: "vet"}, want: exitFindings},
		{name: "tool", err: &check.ToolError{Check: "vet", Err: errors.New("missing")}, want: exitTool},
		{name: "unexpected", err: errors.New("boom"), want: exitTool},
	}

	for _, tt := range tests {
		if got := exitCode(tt.err); got != tt.want {
			t.Fatalf("%s: exitCode() = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestModuleRoot(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string) string
		want  func(string) string
	}{
		{name: "module root", setup: func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
			return dir
		}, want: func(dir string) string { return dir }},
		{name: "nested package", setup: func(t *testing.T, dir string) string {
			writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
			nested := filepath.Join(dir, "internal", "pkg")
			if err := os.MkdirAll(nested, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			return nested
		}, want: func(dir string) string { return dir }},
		{name: "outside a module", setup: func(_ *testing.T, dir string) string {
			return dir
		}, want: func(dir string) string { return dir }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(tt.setup(t, dir))

			got, err := moduleRoot()
			if err != nil {
				t.Fatalf("moduleRoot() error = %v", err)
			}
			if want := tt.want(dir); got != want {
				t.Fatalf("moduleRoot() = %q, want %q", got, want)
			}
		})
	}
}

// TestRunReadsConfigurationFromModuleRoot covers running a gate inside a
// package directory: the configuration file lives at the module root.
func TestRunReadsConfigurationFromModuleRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	// The misspelled key is the assertion: only the module-root file can
	// reject it, so a passing gate run would mean the file was not read.
	writeFile(t, filepath.Join(dir, config.FileName), "coverge:\n  minimum: 90\n")

	nested := filepath.Join(dir, "internal", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(nested)

	var stdout, stderr bytes.Buffer
	if got := run([]string{"format"}, &stdout, &stderr); got != exitUsage {
		t.Fatalf("run() = %d, want %d\nstderr: %s", got, exitUsage, stderr.String())
	}
}

// TestRunGateRunsAtModuleRoot covers gate execution from a package directory:
// an unformatted file at the module root fails the format gate.
func TestRunGateRunsAtModuleRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	writeFile(t, filepath.Join(dir, "unformatted.go"), "package m\nfunc  Unformatted( ) {}\n")

	nested := filepath.Join(dir, "internal", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(nested)

	var stdout, stderr bytes.Buffer
	if got := run([]string{"format"}, &stdout, &stderr); got != exitFindings {
		t.Fatalf("run() = %d, want %d\nstdout: %s\nstderr: %s", got, exitFindings, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "unformatted.go") {
		t.Fatalf("stdout = %q, want the module-root file", stdout.String())
	}
}

func TestRunGate(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context) error
		want int
		ok   bool
	}{
		{name: "passes", run: func(context.Context) error { return nil }, want: exitOK, ok: true},
		{name: "findings", run: func(context.Context) error {
			return &check.FindingError{Check: "vet"}
		}, want: exitFindings},
		{name: "tool failure", run: func(context.Context) error {
			return &check.ToolError{Check: "vet", Err: errors.New("missing")}
		}, want: exitTool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := runGate(gate{name: "gate", run: tt.run}, &stdout, &stderr)
			if got != tt.want {
				t.Fatalf("runGate() = %d, want %d", got, tt.want)
			}
			if tt.ok && !strings.Contains(stdout.String(), "ok   gate") {
				t.Fatalf("stdout = %q, want success line", stdout.String())
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
