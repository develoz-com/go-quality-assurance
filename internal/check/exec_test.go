package check

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCommandExecutorRun(t *testing.T) {
	t.Parallel()

	exec := commandExecutor{out: io.Discard}

	if err := exec.Run(context.Background(), t.TempDir(), "go", "version"); err != nil {
		t.Fatalf("go version: %v", err)
	}

	err := exec.Run(context.Background(), t.TempDir(), "go", "not-a-command")
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code == 0 {
		t.Fatalf("error = %v, want non-zero ExitError", err)
	}

	if err := exec.Run(context.Background(), t.TempDir(), "gqa-missing-tool"); err == nil {
		t.Fatal("missing tool: want error")
	} else {
		var exit *ExitError
		if errors.As(err, &exit) {
			t.Fatalf("missing tool: error = %v, want non-exit error", err)
		}
	}
}

func TestCommandExecutorOutput(t *testing.T) {
	t.Parallel()

	exec := commandExecutor{out: io.Discard}

	out, err := exec.Output(context.Background(), t.TempDir(), "go", "version")
	if err != nil {
		t.Fatalf("go version: %v", err)
	}
	if !strings.HasPrefix(string(out), "go version") {
		t.Fatalf("output = %q", out)
	}

	_, err = exec.Output(context.Background(), t.TempDir(), "go", "not-a-command")
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code == 0 {
		t.Fatalf("error = %v, want non-zero ExitError", err)
	}
}
