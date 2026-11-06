package check

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Tool is a pinned command-line tool.
type Tool struct {
	Name    string // executable name
	Package string // package to install
	Version string // version, mirrored by a tool directive in this module's go.mod
}

// Pinned tools. TestPinsMatchGoMod keeps the versions in step with go.mod.
var (
	Staticcheck = Tool{Name: "staticcheck", Package: "honnef.co/go/tools/cmd/staticcheck", Version: "v0.8.2"}
	Govulncheck = Tool{Name: "govulncheck", Package: "golang.org/x/vuln/cmd/govulncheck", Version: "v1.9.0"}
)

// Installer resolves a pinned tool to an executable, installing it when absent.
type Installer interface {
	Install(ctx context.Context, tool Tool) (string, error)
}

// GoInstaller installs pinned tools with go install into a version-keyed cache.
type GoInstaller struct {
	Root string // cache root; empty when the platform has no user cache directory
	Out  io.Writer
	err  error // why Root is empty
}

// NewGoInstaller returns an installer rooted in the user cache directory.
func NewGoInstaller(out io.Writer) GoInstaller {
	installer := GoInstaller{Out: out}
	root, err := os.UserCacheDir()
	if err != nil {
		installer.err = err
		return installer
	}
	installer.Root = filepath.Join(root, "gqa")
	return installer
}

// Path returns the cache path of a pinned tool executable. The platform is part
// of the key so a shared home directory cannot serve a foreign-architecture
// binary.
func (g GoInstaller) Path(tool Tool) string {
	return filepath.Join(g.Root, "tools", tool.Name+"@"+tool.Version, runtime.GOOS+"-"+runtime.GOARCH, tool.Name)
}

// Install returns the cached executable, installing the pinned version when absent.
func (g GoInstaller) Install(ctx context.Context, tool Tool) (string, error) {
	if g.Root == "" {
		return "", errors.Join(errors.New("no user cache directory"), g.err)
	}

	path := g.Path(tool)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// go install copies into GOBIN, so a concurrent reader could exec a partial
	// binary. Install beside the cache path and rename, which is atomic.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(dir, "install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	install := exec.CommandContext(ctx, "go", "install", tool.Package+"@"+tool.Version)
	install.Env = append(os.Environ(), "GOBIN="+staging)
	install.Stdout = g.Out
	install.Stderr = g.Out
	if err := normalizeExit(install.Run()); err != nil {
		return "", fmt.Errorf("install %s@%s: %w", tool.Package, tool.Version, err)
	}

	if err := os.Rename(filepath.Join(staging, tool.Name), path); err != nil {
		return "", err
	}
	return path, nil
}
