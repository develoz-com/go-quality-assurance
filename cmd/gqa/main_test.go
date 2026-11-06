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
