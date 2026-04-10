package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAutoProfileUsesLastUsedFromLocalState(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Profile 1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Profile 1" {
		t.Fatalf("expected auto-detected profile name, got %q", cfg.ProfileName)
	}
}

func TestParseAutoProfileFallsBackToDefault(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-name", "auto",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Default" {
		t.Fatalf("expected fallback profile name Default, got %q", cfg.ProfileName)
	}
}

func TestParseExplicitProfileNameWins(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Profile 1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-name", "Profile 2",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Profile 2" {
		t.Fatalf("expected explicit profile name to be preserved, got %q", cfg.ProfileName)
	}
}

func TestParseDefaultsToPersistentMode(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileMode != ProfileModePersistent {
		t.Fatalf("expected default profile mode %q, got %q", ProfileModePersistent, cfg.ProfileMode)
	}
}

func TestParseExplicitProfileModeWins(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-mode", ProfileModeCopy,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileMode != ProfileModeCopy {
		t.Fatalf("expected explicit profile mode %q, got %q", ProfileModeCopy, cfg.ProfileMode)
	}
}

func TestParseGeneratesTimestampedOutputDirWhenUnset(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	prefix := filepath.Join(cfg.DefaultOutputBase, "doubao-")
	if !strings.HasPrefix(cfg.OutputDir, prefix) {
		t.Fatalf("expected output dir with prefix %q, got %q", prefix, cfg.OutputDir)
	}
}
