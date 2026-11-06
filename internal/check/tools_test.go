package check

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoInstallerPath(t *testing.T) {
	t.Parallel()

	installer := GoInstaller{Root: "/cache/gqa"}
	want := filepath.Join("/cache/gqa", "tools", "staticcheck@v0.8.2", runtime.GOOS+"-"+runtime.GOARCH, "staticcheck")
	if got := installer.Path(Staticcheck); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestGoInstallerReusesCachedTool(t *testing.T) {
	root := t.TempDir()
	installer := GoInstaller{Root: root}
	path := installer.Path(Staticcheck)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write tool: %v", err)
	}

	// A failing go binary proves Install did not shell out for a cached tool.
	fakeGo(t, "exit 9\n")

	got, err := installer.Install(context.Background(), Staticcheck)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got != path {
		t.Fatalf("Install() = %q, want %q", got, path)
	}
}

func TestGoInstallerInstallsMissingTool(t *testing.T) {
	root := t.TempDir()
	installer := GoInstaller{Root: root}
	log := filepath.Join(t.TempDir(), "go.log")
	fakeGo(t, `echo "$@" > `+log+`
printf '#!/bin/sh\n' > "$GOBIN/staticcheck"
chmod +x "$GOBIN/staticcheck"
`)

	got, err := installer.Install(context.Background(), Staticcheck)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if got != installer.Path(Staticcheck) {
		t.Fatalf("Install() = %q, want %q", got, installer.Path(Staticcheck))
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("installed tool: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("installed tool %q is a directory", got)
	}

	recorded, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	want := "install honnef.co/go/tools/cmd/staticcheck@v0.8.2"
	if got := strings.TrimSpace(string(recorded)); got != want {
		t.Fatalf("go %s, want go %s", got, want)
	}
	assertNoStagingDirs(t, filepath.Dir(got))
}

func TestGoInstallerReportsMissingCacheDirectory(t *testing.T) {
	t.Parallel()

	if _, err := (GoInstaller{}).Install(context.Background(), Staticcheck); err == nil {
		t.Fatal("Install() error = nil, want error")
	}
}

// TestGoInstallerKeepsCacheDirectoryCause covers the platform error behind a
// missing user cache directory, which the installer must not swallow.
func TestGoInstallerKeepsCacheDirectoryCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("neither $XDG_CACHE_HOME nor $HOME are defined")
	_, err := (GoInstaller{err: cause}).Install(context.Background(), Staticcheck)
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want wrapped cause", err)
	}
}

func TestGoInstallerReportsInstallFailure(t *testing.T) {
	fakeGo(t, "exit 1\n")

	installer := GoInstaller{Root: t.TempDir()}
	_, err := installer.Install(context.Background(), Staticcheck)
	if err == nil {
		t.Fatal("Install() error = nil, want error")
	}
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("error = %v, want wrapped ExitError", err)
	}
	assertNoStagingDirs(t, filepath.Dir(installer.Path(Staticcheck)))
}

// TestPinsMatchGoMod keeps the versions the installer uses identical to the
// tool directives in go.mod, so the pin has one recorded source of truth.
func TestPinsMatchGoMod(t *testing.T) {
	t.Parallel()

	for _, tool := range []Tool{Staticcheck, Govulncheck} {
		module, _, ok := strings.Cut(tool.Package, "/cmd/")
		if !ok {
			t.Fatalf("%s: not a command package", tool.Package)
		}
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", module).Output()
		if err != nil {
			t.Fatalf("go list -m %s: %v", module, err)
		}
		if got := strings.TrimSpace(string(out)); got != tool.Version {
			t.Fatalf("%s pinned at %s, go.mod records %s", module, tool.Version, got)
		}
	}
}

// fakeGo puts a go binary with the given shell body first in PATH.
func fakeGo(t *testing.T, body string) {
	t.Helper()

	bin := t.TempDir()
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertNoStagingDirs(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "install-") {
			t.Fatalf("staging directory %q was not cleaned up", entry.Name())
		}
	}
}
