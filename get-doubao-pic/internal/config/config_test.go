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
		"--profile-mode", ProfileModeCopy,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Profile 1" {
		t.Fatalf("expected auto-detected profile name, got %q", cfg.ProfileName)
	}
}

func TestParsePersistentAutoProfileUsesCLIProfileLastUsedWhenPresent(t *testing.T) {
	root := t.TempDir()
	cliRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Profile 1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliRoot, "Local State"), []byte(`{"profile":{"last_used":"Default"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-mode", ProfileModePersistent,
		"--cli-profile-dir", cliRoot,
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Default" {
		t.Fatalf("expected persistent mode to prefer CLI profile last_used, got %q", cfg.ProfileName)
	}
}

func TestParseRefreshCLIProfileUsesSourceLastUsed(t *testing.T) {
	root := t.TempDir()
	cliRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Profile 1"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliRoot, "Local State"), []byte(`{"profile":{"last_used":"Default"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-mode", ProfileModePersistent,
		"--cli-profile-dir", cliRoot,
		"--refresh-cli-profile",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.ProfileName != "Profile 1" {
		t.Fatalf("expected refresh to use source profile last_used, got %q", cfg.ProfileName)
	}
}

func TestParseAutoProfileFallsBackToDefault(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--profile-mode", ProfileModeCopy,
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
		"--profile-mode", ProfileModeCopy,
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
		"--cli-profile-dir", t.TempDir(),
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

func TestParseRefreshCLIProfileFlag(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--refresh-cli-profile",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if !cfg.RefreshCLIProfile {
		t.Fatalf("expected refresh flag to be enabled")
	}
}

func TestParseGeneratesTimestampedOutputDirWhenUnset(t *testing.T) {
	root := t.TempDir()

	cfg, err := Parse([]string{
		"--prompt", "test prompt",
		"--profile-dir", root,
		"--cli-profile-dir", t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	prefix := filepath.Join(cfg.DefaultOutputBase, "doubao-")
	if !strings.HasPrefix(cfg.OutputDir, prefix) {
		t.Fatalf("expected output dir with prefix %q, got %q", prefix, cfg.OutputDir)
	}
}
