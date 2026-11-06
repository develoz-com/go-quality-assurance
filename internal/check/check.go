// Package check runs the Go quality gates.
package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// FindingError reports a gate that failed because of project code.
type FindingError struct {
	Check  string
	Detail string
}

func (e *FindingError) Error() string {
	if e.Detail == "" {
		return e.Check + ": findings"
	}
	return e.Check + ": " + e.Detail
}

// ToolError reports a gate that could not run to completion.
type ToolError struct {
	Check string
	Err   error
}

func (e *ToolError) Error() string { return e.Check + ": " + e.Err.Error() }

func (e *ToolError) Unwrap() error { return e.Err }

// ExitError reports a command that ran and exited non-zero.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// Executor runs the tool commands used by the gates.
type Executor interface {
	Output(ctx context.Context, dir, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, dir, name string, args ...string) error
}

// Runner executes the quality gates for one module.
type Runner struct {
	Dir   string
	Out   io.Writer
	Exec  Executor
	Tools Installer
}

// NewRunner returns a Runner that executes tools on the host.
func NewRunner(dir string, out io.Writer) *Runner {
	return &Runner{Dir: dir, Out: out, Exec: commandExecutor{out: out}, Tools: NewGoInstaller(out)}
}

// Format reports files that gofmt would rewrite.
func (r *Runner) Format(ctx context.Context) error {
	out, err := r.Exec.Output(ctx, r.Dir, "gofmt", "-l", ".")
	if err != nil {
		return &ToolError{Check: "format", Err: err}
	}
	files := strings.TrimSpace(string(out))
	if files == "" {
		return nil
	}
	fmt.Fprintln(r.Out, files)
	return &FindingError{Check: "format", Detail: "files are not gofmt-formatted"}
}

// Vet runs go vet.
func (r *Runner) Vet(ctx context.Context) error {
	return r.gate(ctx, "vet", 1, "go", "vet", "./...")
}

// Static runs staticcheck at the version pinned by this module.
func (r *Runner) Static(ctx context.Context) error {
	return r.toolGate(ctx, "static", 1, Staticcheck)
}

// Vulnerabilities runs govulncheck at the version pinned by this module.
func (r *Runner) Vulnerabilities(ctx context.Context) error {
	return r.toolGate(ctx, "vuln", 3, Govulncheck)
}

// Test runs race-enabled tests.
func (r *Runner) Test(ctx context.Context) error {
	return r.gate(ctx, "test", 1, "go", "test", "-race", "./...")
}

// Coverage runs race-enabled tests with coverage and enforces the minimum percentage.
func (r *Runner) Coverage(ctx context.Context, minimum float64) error {
	profile, err := os.CreateTemp("", "gqa-cover-*.out")
	if err != nil {
		return &ToolError{Check: "coverage", Err: err}
	}
	path := profile.Name()
	profile.Close()
	defer os.Remove(path)

	if err := r.gate(ctx, "coverage", 1, "go", "test", "-race", "-covermode=atomic", "-coverprofile="+path, "./..."); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return &ToolError{Check: "coverage", Err: err}
	}
	defer file.Close()

	percent, err := CoveragePercent(file)
	if err != nil {
		if errors.Is(err, ErrNoStatements) {
			return &FindingError{Check: "coverage", Detail: "no statements measured"}
		}
		return &ToolError{Check: "coverage", Err: err}
	}

	fmt.Fprintf(r.Out, "coverage %.1f%% (minimum %.1f%%)\n", percent, minimum)
	if percent < minimum {
		return &FindingError{Check: "coverage", Detail: fmt.Sprintf("%.1f%% below minimum %.1f%%", percent, minimum)}
	}
	return nil
}

// toolGate installs the pinned tool when needed and runs it over the module.
func (r *Runner) toolGate(ctx context.Context, name string, findingsCode int, tool Tool) error {
	path, err := r.Tools.Install(ctx, tool)
	if err != nil {
		return &ToolError{Check: name, Err: err}
	}
	return r.gate(ctx, name, findingsCode, path, "./...")
}

func (r *Runner) gate(ctx context.Context, name string, findingsCode int, executable string, args ...string) error {
	err := r.Exec.Run(ctx, r.Dir, executable, args...)
	if err == nil {
		return nil
	}
	var exit *ExitError
	if errors.As(err, &exit) && exit.Code == findingsCode {
		return &FindingError{Check: name, Detail: "tool reported findings"}
	}
	return &ToolError{Check: name, Err: err}
}

type commandExecutor struct {
	out io.Writer
}

func (c commandExecutor) Output(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return out, fmt.Errorf("%w: %s", normalizeExit(err), message)
		}
		return out, normalizeExit(err)
	}
	return out, nil
}

func (c commandExecutor) Run(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = c.out
	cmd.Stderr = c.out
	return normalizeExit(cmd.Run())
}

func normalizeExit(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ExitError{Code: exit.ExitCode()}
	}
	return err
}
