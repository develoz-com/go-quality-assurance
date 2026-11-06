package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		writeQa bool
		file    string
		env     map[string]string
		want    float64
		wantErr bool
	}{
		{name: "default", want: DefaultCoverageMinimum},
		{name: "file overrides default", writeQa: true, file: "coverage:\n  minimum: 85.5\n", want: 85.5},
		{name: "partial file keeps other defaults", writeQa: true, file: "coverage: {}\n", want: DefaultCoverageMinimum},
		{name: "empty file keeps default", writeQa: true, want: DefaultCoverageMinimum},
		{name: "environment overrides file", writeQa: true, file: "coverage:\n  minimum: 85\n", env: map[string]string{EnvCoverageMinimum: "90"}, want: 90},
		{name: "environment without file", env: map[string]string{EnvCoverageMinimum: "42"}, want: 42},
		{name: "explicit config path", env: map[string]string{EnvConfigPath: "custom.yaml"}, want: 70},
		{name: "unknown field", writeQa: true, file: "coverge:\n  minimum: 80\n", wantErr: true},
		{name: "malformed yaml", writeQa: true, file: "coverage: [\n", wantErr: true},
		{name: "non numeric environment", env: map[string]string{EnvCoverageMinimum: "high"}, wantErr: true},
		{name: "negative environment", env: map[string]string{EnvCoverageMinimum: "-1"}, wantErr: true},
		{name: "file above range", writeQa: true, file: "coverage:\n  minimum: 101\n", wantErr: true},
		{name: "missing explicit file", env: map[string]string{EnvConfigPath: "missing.yaml", EnvCoverageMinimum: "50"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.writeQa {
				writeFile(t, filepath.Join(dir, FileName), tt.file)
			}
			if tt.env[EnvConfigPath] == "custom.yaml" {
				writeFile(t, filepath.Join(dir, "custom.yaml"), "coverage:\n  minimum: 70\n")
			}

			getenv := func(key string) string { return tt.env[key] }
			cfg, err := Load(dir, getenv)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = %v, want error", cfg.Coverage.Minimum)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Coverage.Minimum != tt.want {
				t.Fatalf("minimum = %v, want %v", cfg.Coverage.Minimum, tt.want)
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
