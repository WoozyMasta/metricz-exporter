// Package config loads and validates configuration and CLI flags.
package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/woozymasta/metricz-exporter/internal/vars"
)

//go:embed example-config.yaml
var embeddedExampleConfig []byte

// BaseConfig describes CLI-level configuration
type BaseConfig struct {
	// betteralign:ignore

	// Use an absolute path in production to avoid dependency on working directory.
	ConfigPath string `short:"c" long:"config" env:"METRICZ_CONFIG" description:"Path to YAML/JSON configuration file" default:"config.yaml"`

	// Create config file from embedded example-config.yaml if ConfigPath is missing or empty.
	InitConfig bool `short:"i" long:"init-config" env:"METRICZ_CONFIG_INIT" description:"Create config at path from embedded example if missing/empty"`

	// Print config to stdout: file at ConfigPath if exists+non-empty, otherwise embedded example-config.yaml.
	PrintConfig bool `short:"p" long:"print-config" env:"METRICZ_CONFIG_PRINT" description:"Print embedded example config if missing/empty, else print path content"`

	// Version prints build info (version, commit, build date, etc.) and exits.
	Version bool `short:"v" long:"version" description:"Print version and build info"`
}

// ParseFlags parses command line arguments and returns options.
func ParseFlags() *BaseConfig {
	var opts BaseConfig
	parser := flags.NewParser(&opts, flags.Default|flags.PassDoubleDash)

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if opts.Version {
		vars.Print()
		os.Exit(0)
	}

	if opts.InitConfig {
		if err := EnsureConfigFile(opts.ConfigPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if opts.PrintConfig {
		if err := PrintConfigOrExample(os.Stdout, opts.ConfigPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	return &opts
}

// PrintConfigOrExample writes to w:
// - file content at path if file exists and non-empty
// - otherwise embedded example-config.yaml
func PrintConfigOrExample(w io.Writer, path string) error {
	ok, err := isNonEmptyFile(path)
	if err != nil {
		return err
	}

	if ok {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}

	_, err = w.Write(embeddedExampleConfig)
	return err
}

// EnsureConfigFile creates config file at path from embedded example-config.yaml
// if file does not exist or is empty/whitespace.
// If file exists and non-empty, does nothing.
func EnsureConfigFile(path string) error {
	ok, err := isNonEmptyFile(path)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	// 0600 because config may contain secrets.
	return writeFileAtomic(path, embeddedExampleConfig, 0o600)
}

func isNonEmptyFile(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if st.Size() == 0 {
		return false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(b)) > 0, nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}

	// Windows: rename fails if destination exists.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}
