package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
)

func TestPrepareUserDataDirCopyMode(t *testing.T) {
	root := t.TempDir()
	profileName := "Default"
	profileDir := filepath.Join(root, profileName)

	if err := os.MkdirAll(filepath.Join(profileDir, "Network"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "Cookies"), []byte("cookies"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "SingletonLock"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "Network", "LOCK"), []byte("network-lock"), 0o644); err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareUserDataDir(config.Config{
		ProfileDir:  root,
		ProfileName: profileName,
		ProfileMode: config.ProfileModeCopy,
	})
	if err != nil {
		t.Fatalf("prepareUserDataDir returned error: %v", err)
	}
	defer func() {
		if err := prepared.cleanup(); err != nil {
			t.Fatalf("cleanup returned error: %v", err)
		}
	}()

	if prepared.userDataDir == root {
		t.Fatalf("expected copied temp userDataDir, got source root")
	}

	if _, err := os.Stat(filepath.Join(prepared.userDataDir, "Local State")); err != nil {
		t.Fatalf("expected Local State to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prepared.userDataDir, profileName, "Cookies")); err != nil {
		t.Fatalf("expected Cookies to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prepared.userDataDir, profileName, "SingletonLock")); !os.IsNotExist(err) {
		t.Fatalf("expected SingletonLock to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(prepared.userDataDir, profileName, "Network", "LOCK")); !os.IsNotExist(err) {
		t.Fatalf("expected nested LOCK to be skipped, got err=%v", err)
	}
}

func TestPrepareUserDataDirDirectMode(t *testing.T) {
	root := t.TempDir()

	prepared, err := prepareUserDataDir(config.Config{
		ProfileDir:  root,
		ProfileName: "Default",
		ProfileMode: config.ProfileModeDirect,
	})
	if err != nil {
		t.Fatalf("prepareUserDataDir returned error: %v", err)
	}

	if prepared.userDataDir != root {
		t.Fatalf("expected direct mode to use original root, got %s", prepared.userDataDir)
	}
}

func TestPrepareUserDataDirPersistentModeInitializesSelectedProfileOnceAndKeepsDirectory(t *testing.T) {
	root := t.TempDir()
	profileName := "Default"
	profileDir := filepath.Join(root, profileName)
	cliProfileDir := filepath.Join(t.TempDir(), "cli-user-data")

	if err := os.MkdirAll(filepath.Join(profileDir, "IndexedDB"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Component State"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SingletonLock"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "Cookies"), []byte("cookies"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "IndexedDB", "LOCK"), []byte("db-lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Component State", "manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Profile 1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Profile 1", "Cookies"), []byte("other-profile"), 0o644); err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareUserDataDir(config.Config{
		ProfileDir:    root,
		ProfileName:   profileName,
		ProfileMode:   config.ProfileModePersistent,
		CLIProfileDir: cliProfileDir,
	})
	if err != nil {
		t.Fatalf("prepareUserDataDir returned error: %v", err)
	}
	if prepared.userDataDir != cliProfileDir {
		t.Fatalf("expected persistent mode to use cli-profile-dir, got %s", prepared.userDataDir)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, "Local State")); err != nil {
		t.Fatalf("expected Local State to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, profileName, "Cookies")); err != nil {
		t.Fatalf("expected Cookies to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, "Component State", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("expected unrelated root state to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, "Profile 1", "Cookies")); !os.IsNotExist(err) {
		t.Fatalf("expected non-selected profiles to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, "SingletonLock")); !os.IsNotExist(err) {
		t.Fatalf("expected root SingletonLock to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, profileName, "IndexedDB", "LOCK")); !os.IsNotExist(err) {
		t.Fatalf("expected nested LOCK to be skipped, got err=%v", err)
	}
	if err := prepared.cleanup(); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
	if _, err := os.Stat(cliProfileDir); err != nil {
		t.Fatalf("expected persistent directory to remain after cleanup: %v", err)
	}

	sentinelPath := filepath.Join(cliProfileDir, "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}

	preparedAgain, err := prepareUserDataDir(config.Config{
		ProfileDir:    root,
		ProfileName:   profileName,
		ProfileMode:   config.ProfileModePersistent,
		CLIProfileDir: cliProfileDir,
	})
	if err != nil {
		t.Fatalf("prepareUserDataDir second run returned error: %v", err)
	}
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatalf("expected existing persistent directory to be reused without recopy: %v", err)
	}
	if err := preparedAgain.cleanup(); err != nil {
		t.Fatalf("cleanup second run returned error: %v", err)
	}
}

func TestPrepareUserDataDirPersistentModeRefreshReinitializesCLIProfile(t *testing.T) {
	root := t.TempDir()
	profileName := "Default"
	profileDir := filepath.Join(root, profileName)
	cliProfileDir := filepath.Join(t.TempDir(), "cli-user-data")

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Local State"), []byte("fresh-state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "Cookies"), []byte("fresh-cookies"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cliProfileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliProfileDir, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cliProfileDir, "Local State"), []byte("stale-state"), 0o644); err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareUserDataDir(config.Config{
		ProfileDir:        root,
		ProfileName:       profileName,
		ProfileMode:       config.ProfileModePersistent,
		CLIProfileDir:     cliProfileDir,
		RefreshCLIProfile: true,
	})
	if err != nil {
		t.Fatalf("prepareUserDataDir returned error: %v", err)
	}
	if prepared.userDataDir != cliProfileDir {
		t.Fatalf("expected persistent mode to use cli-profile-dir, got %s", prepared.userDataDir)
	}
	if _, err := os.Stat(filepath.Join(cliProfileDir, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected refresh to replace stale directory contents, got err=%v", err)
	}
	payload, err := os.ReadFile(filepath.Join(cliProfileDir, "Local State"))
	if err != nil {
		t.Fatalf("read refreshed Local State: %v", err)
	}
	if string(payload) != "fresh-state" {
		t.Fatalf("expected refreshed Local State to come from source profile, got %q", string(payload))
	}
	if err := prepared.cleanup(); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
}
