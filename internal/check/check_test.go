package check

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatesClassifyFindings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		exitCode    int
		gate        func(*Runner, context.Context) error
		wantFinding bool
	}{
		{name: "vet passes", exitCode: 0, gate: (*Runner).Vet},
		{name: "vet findings", exitCode: 1, gate: (*Runner).Vet, wantFinding: true},
		{name: "vet tool failure", exitCode: 2, gate: (*Runner).Vet},
		{name: "static findings", exitCode: 1, gate: (*Runner).Static, wantFinding: true},
		{name: "static tool failure", exitCode: 2, gate: (*Runner).Static},
		{name: "vulnerabilities findings", exitCode: 3, gate: (*Runner).Vulnerabilities, wantFinding: true},
		{name: "vulnerabilities tool failure", exitCode: 1, gate: (*Runner).Vulnerabilities},
		{name: "test findings", exitCode: 1, gate: (*Runner).Test, wantFinding: true},
		{name: "test tool failure", exitCode: 2, gate: (*Runner).Test},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := &Runner{Dir: t.TempDir(), Out: io.Discard, Exec: fakeExecutor{runErr: exitError(tt.exitCode)}}
			err := tt.gate(runner, context.Background())

			switch {
			case tt.exitCode == 0 && err != nil:
				t.Fatalf("error = %v, want nil", err)
			case tt.exitCode == 0:
			case tt.wantFinding && !isFinding(err):
				t.Fatalf("error = %v, want FindingError", err)
			case !tt.wantFinding && !isToolError(err):
				t.Fatalf("error = %v, want ToolError", err)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		output    string
		outputErr error
		want      string
	}{
		{name: "clean", want: ""},
		{name: "unformatted", output: "main.go\n", want: "FindingError"},
		{name: "exec failure", outputErr: errors.New("gofmt not found"), want: "ToolError"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := &Runner{Dir: t.TempDir(), Out: io.Discard, Exec: fakeExecutor{output: []byte(tt.output), outputErr: tt.outputErr}}
			err := runner.Format(context.Background())

			switch tt.want {
			case "":
				if err != nil {
					t.Fatalf("error = %v", err)
				}
			case "FindingError":
				if !isFinding(err) {
					t.Fatalf("error = %v, want FindingError", err)
				}
			default:
				if !isToolError(err) {
					t.Fatalf("error = %v, want ToolError", err)
				}
			}
		})
	}
}

func TestCoverageGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile string
		minimum float64
		want    string
	}{
		{name: "above minimum", profile: "mode: atomic\nexample.go:1.1,2.2 2 1\n", minimum: 100},
		{name: "below minimum", profile: "mode: atomic\nexample.go:1.1,2.2 2 1\nexample.go:3.1,4.2 2 0\n", minimum: 80, want: "FindingError"},
		{name: "no statements", profile: "mode: atomic\n", minimum: 100, want: "FindingError"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			runner := &Runner{
				Dir:  dir,
				Out:  io.Discard,
				Exec: fakeExecutor{profile: tt.profile},
			}

			err := runner.Coverage(context.Background(), tt.minimum)
			switch tt.want {
			case "":
				if err != nil {
					t.Fatalf("error = %v", err)
				}
			default:
				if !isFinding(err) {
					t.Fatalf("error = %v, want FindingError", err)
				}
			}
		})
	}
}

func TestCoverageGateTestFailure(t *testing.T) {
	t.Parallel()

	runner := &Runner{Dir: t.TempDir(), Out: io.Discard, Exec: fakeExecutor{runErr: exitError(1)}}
	if err := runner.Coverage(context.Background(), 100); !isFinding(err) {
		t.Fatalf("error = %v, want FindingError", err)
	}
}

func TestCoverageGateToolFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		exec fakeExecutor
	}{
		{name: "test setup failed", exec: fakeExecutor{runErr: exitError(2)}},
		{name: "malformed profile", exec: fakeExecutor{profile: "mode: atomic\nbroken\n"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := &Runner{Dir: t.TempDir(), Out: io.Discard, Exec: tt.exec}
			err := runner.Coverage(context.Background(), 100)
			if !isToolError(err) {
				t.Fatalf("error = %v, want ToolError", err)
			}
		})
	}
}

func isFinding(err error) bool {
	var finding *FindingError
	return errors.As(err, &finding)
}

func isToolError(err error) bool {
	var tool *ToolError
	return errors.As(err, &tool)
}

func exitError(code int) error {
	if code == 0 {
		return nil
	}
	return &ExitError{Code: code}
}

// fakeExecutor records nothing and returns canned results. When profile is set,
// it writes it to the -coverprofile path passed to Run.
type fakeExecutor struct {
	output    []byte
	outputErr error
	runErr    error
	profile   string
}

func (f fakeExecutor) Output(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
	return f.output, f.outputErr
}

func (f fakeExecutor) Run(_ context.Context, _ string, _ string, args ...string) error {
	if f.profile != "" {
		for _, arg := range args {
			path, ok := strings.CutPrefix(arg, "-coverprofile=")
			if !ok {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(f.profile), 0o600)
		}
	}
	return f.runErr
}
