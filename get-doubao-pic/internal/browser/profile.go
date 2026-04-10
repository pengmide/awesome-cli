package browser

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
)

type preparedProfile struct {
	userDataDir string
	cleanup     func() error
}

func prepareUserDataDir(cfg config.Config) (preparedProfile, error) {
	switch cfg.ProfileMode {
	case config.ProfileModeDirect:
		if cfg.ProfileDir == "" {
			return preparedProfile{}, fmt.Errorf("profile-dir is required in direct mode")
		}
		return preparedProfile{
			userDataDir: cfg.ProfileDir,
			cleanup: func() error {
				return nil
			},
		}, nil
	case config.ProfileModeCopy:
		return prepareCopiedUserDataDir(cfg.ProfileDir, cfg.ProfileName)
	case config.ProfileModePersistent:
		return preparePersistentUserDataDir(cfg.ProfileDir, cfg.ProfileName, cfg.CLIProfileDir)
	default:
		return preparedProfile{}, fmt.Errorf("unsupported profile mode: %s", cfg.ProfileMode)
	}
}

func prepareCopiedUserDataDir(sourceRoot string, profileName string) (preparedProfile, error) {
	if sourceRoot == "" {
		return preparedProfile{}, fmt.Errorf("profile-dir is required")
	}
	if profileName == "" {
		return preparedProfile{}, fmt.Errorf("profile-name is required")
	}

	sourceProfileDir := filepath.Join(sourceRoot, profileName)
	info, err := os.Stat(sourceProfileDir)
	if err != nil {
		return preparedProfile{}, fmt.Errorf("source profile %s: %w", sourceProfileDir, err)
	}
	if !info.IsDir() {
		return preparedProfile{}, fmt.Errorf("source profile %s is not a directory", sourceProfileDir)
	}

	tempRoot, err := os.MkdirTemp("", "doubao-chrome-*")
	if err != nil {
		return preparedProfile{}, fmt.Errorf("create temp chrome dir: %w", err)
	}
	cleanup := func() error {
		return os.RemoveAll(tempRoot)
	}

	if err := copyChromeRootFile(sourceRoot, tempRoot, "Local State"); err != nil {
		_ = cleanup()
		return preparedProfile{}, err
	}
	if err := copyDirTree(sourceProfileDir, filepath.Join(tempRoot, profileName)); err != nil {
		_ = cleanup()
		return preparedProfile{}, err
	}

	return preparedProfile{
		userDataDir: tempRoot,
		cleanup:     cleanup,
	}, nil
}

func preparePersistentUserDataDir(sourceRoot string, profileName string, cliProfileDir string) (preparedProfile, error) {
	if sourceRoot == "" {
		return preparedProfile{}, fmt.Errorf("profile-dir is required")
	}
	if profileName == "" {
		return preparedProfile{}, fmt.Errorf("profile-name is required")
	}
	if cliProfileDir == "" {
		return preparedProfile{}, fmt.Errorf("cli-profile-dir is required in persistent mode")
	}

	sourceProfileDir := filepath.Join(sourceRoot, profileName)
	info, err := os.Stat(sourceProfileDir)
	if err != nil {
		return preparedProfile{}, fmt.Errorf("source profile %s: %w", sourceProfileDir, err)
	}
	if !info.IsDir() {
		return preparedProfile{}, fmt.Errorf("source profile %s is not a directory", sourceProfileDir)
	}

	empty, err := dirEmpty(cliProfileDir)
	if err != nil {
		return preparedProfile{}, fmt.Errorf("check cli-profile-dir %s: %w", cliProfileDir, err)
	}
	if empty {
		if err := os.RemoveAll(cliProfileDir); err != nil && !os.IsNotExist(err) {
			return preparedProfile{}, fmt.Errorf("reset cli-profile-dir %s: %w", cliProfileDir, err)
		}
		if err := copyChromeUserDataDir(sourceRoot, cliProfileDir); err != nil {
			return preparedProfile{}, err
		}
	}

	return preparedProfile{
		userDataDir: cliProfileDir,
		cleanup: func() error {
			return nil
		},
	}, nil
}

func copyChromeRootFile(sourceRoot string, targetRoot string, name string) error {
	src := filepath.Join(sourceRoot, name)
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat chrome root file %s: %w", src, err)
	}
	if info.IsDir() {
		return nil
	}
	dst := filepath.Join(targetRoot, name)
	if err := copyFile(src, dst, info.Mode()); err != nil {
		return fmt.Errorf("copy chrome root file %s: %w", src, err)
	}
	return nil
}

func copyChromeUserDataDir(sourceRoot string, targetRoot string) error {
	info, err := os.Stat(sourceRoot)
	if err != nil {
		return fmt.Errorf("stat chrome user data dir %s: %w", sourceRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("chrome user data dir %s is not a directory", sourceRoot)
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("create target user data dir %s: %w", targetRoot, err)
	}
	if err := copyDirTree(sourceRoot, targetRoot); err != nil {
		return fmt.Errorf("copy chrome user data dir %s: %w", sourceRoot, err)
	}
	return nil
}

func copyDirTree(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if shouldSkipChromeEntry(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, targetPath, info.Mode())
	})
}

func dirEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func shouldSkipChromeEntry(name string) bool {
	switch strings.ToLower(name) {
	case "singletonlock", "singletonsocket", "singletoncookie", "lockfile", "lock", "devtoolsactiveport":
		return true
	default:
		return false
	}
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return nil
}
