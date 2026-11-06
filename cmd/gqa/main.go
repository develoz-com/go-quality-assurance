// Command gqa runs Go quality gates.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/develoz-com/go-quality-assurance/internal/check"
	"github.com/develoz-com/go-quality-assurance/internal/config"
)

// Exit codes distinguish findings from failures to run the tooling.
const (
	exitOK       = 0
	exitFindings = 1
	exitUsage    = 2
	exitTool     = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}

	command := args[0]
	switch command {
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "gqa %s\n", version())
		return exitOK
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "gqa: %v\n", err)
		return exitTool
	}

	cfg, err := config.Load(dir, os.Getenv)
	if err != nil {
		fmt.Fprintf(stderr, "gqa: %v\n", err)
		return exitUsage
	}

	runner := check.NewRunner(dir, stdout)
	gates := []gate{
		{name: "format", run: runner.Format},
		{name: "vet", run: runner.Vet},
		{name: "static", run: runner.Static},
		{name: "vuln", run: runner.Vulnerabilities},
		{name: "test", run: runner.Test},
		{name: "coverage", run: func(ctx context.Context) error { return runner.Coverage(ctx, cfg.Coverage.Minimum) }},
	}

	if command == "ci" {
		return runAll(gates, stdout, stderr)
	}

	for _, candidate := range gates {
		if candidate.name != command {
			continue
		}
		return runGate(candidate, stdout, stderr)
	}

	fmt.Fprintf(stderr, "gqa: unknown command %q\n\n", command)
	printUsage(stderr)
	return exitUsage
}

type gate struct {
	name string
	run  func(context.Context) error
}

func runAll(gates []gate, stdout, stderr io.Writer) int {
	for _, candidate := range gates {
		if code := runGate(candidate, stdout, stderr); code != exitOK {
			return code
		}
	}
	return exitOK
}

func runGate(candidate gate, stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "==> %s\n", candidate.name)

	if err := candidate.run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "FAIL %v\n", err)
		return exitCode(err)
	}

	fmt.Fprintf(stdout, "ok   %s\n", candidate.name)
	return exitOK
}

func exitCode(err error) int {
	var finding *check.FindingError
	if errors.As(err, &finding) {
		return exitFindings
	}
	return exitTool
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `gqa runs Go quality gates.

Usage:
  gqa <command>

Commands:
  format    Check gofmt formatting
  vet       Run go vet
  static    Run staticcheck
  vuln      Run govulncheck
  test      Run race-enabled tests
  coverage  Run race-enabled tests and enforce the coverage minimum
  ci        Run every gate in order, stopping at the first failure
  version   Print the version
  help      Print this help

Configuration:
  qa.yaml in the module root; environment overrides it:
    QA_CONFIG            configuration file path
    QA_COVERAGE_MINIMUM  coverage threshold percentage

Exit codes:
  0  all gates passed
  1  a gate found problems in the code
  2  configuration or usage error
  3  a gate could not run
`)
}
