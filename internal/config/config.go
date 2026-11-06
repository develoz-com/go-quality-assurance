// Package config resolves quality-gate configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

const (
	// FileName is the project configuration file read from the module root.
	FileName = "qa.yaml"
	// EnvConfigPath overrides the configuration file path.
	EnvConfigPath = "QA_CONFIG"
	// EnvCoverageMinimum overrides the coverage threshold percentage.
	EnvCoverageMinimum = "QA_COVERAGE_MINIMUM"
	// DefaultCoverageMinimum is the coverage threshold used when nothing overrides it.
	DefaultCoverageMinimum = 100.0
)

// Config is the resolved quality-gate configuration.
type Config struct {
	Coverage Coverage `yaml:"coverage"`
}

// Coverage holds the coverage gate settings.
type Coverage struct {
	Minimum float64 `yaml:"minimum"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{Coverage: Coverage{Minimum: DefaultCoverageMinimum}}
}

// Load resolves configuration with precedence environment > project file > default.
func Load(dir string, getenv func(string) string) (Config, error) {
	cfg := Default()

	path := getenv(EnvConfigPath)
	explicit := path != ""
	if explicit {
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
	} else {
		path = filepath.Join(dir, FileName)
	}
	if err := applyFile(&cfg, path, explicit); err != nil {
		return Config{}, err
	}

	if raw := getenv(EnvCoverageMinimum); raw != "" {
		minimum, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%s: not a number: %q", EnvCoverageMinimum, raw)
		}
		cfg.Coverage.Minimum = minimum
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyFile(cfg *Config, path string, required bool) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !required {
			return nil
		}
		return fmt.Errorf("config %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("config %s: %w", path, err)
	}
	return nil
}

func validate(cfg Config) error {
	minimum := cfg.Coverage.Minimum
	if math.IsNaN(minimum) || math.IsInf(minimum, 0) {
		return fmt.Errorf("coverage minimum: must be a finite number")
	}
	if minimum < 0 || minimum > 100 {
		return fmt.Errorf("coverage minimum: %v outside 0-100", minimum)
	}
	return nil
}
