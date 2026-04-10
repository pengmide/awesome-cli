package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ExitSuccess = 0
	ExitUsage   = 2
	ExitPartial = 3

	ProfileModeCopy       = "copy"
	ProfileModeDirect     = "direct"
	ProfileModePersistent = "persistent"
	ProfileNameAuto       = "auto"
)

type Config struct {
	Prompt            string
	OutputDir         string
	Timeout           time.Duration
	StableWindow      time.Duration
	InteractiveWait   time.Duration
	ProfileDir        string
	ProfileName       string
	ProfileMode       string
	CLIProfileDir     string
	ChromePath        string
	Headless          bool
	DefaultOutputBase string
}

func Parse(args []string) (Config, error) {
	cfg := Config{
		Timeout:           180 * time.Second,
		StableWindow:      8 * time.Second,
		InteractiveWait:   30 * time.Second,
		ProfileName:       ProfileNameAuto,
		ProfileMode:       ProfileModePersistent,
		CLIProfileDir:     defaultCLIChromeUserDataDir(),
		DefaultOutputBase: "downloads",
	}

	fs := flag.NewFlagSet("doubao-pic", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.Prompt, "prompt", "", "Prompt to send to Doubao")
	fs.StringVar(&cfg.OutputDir, "output", "", "Directory to save downloaded images")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Overall timeout for the generation round")
	fs.DurationVar(&cfg.StableWindow, "stable-window", cfg.StableWindow, "No-new-image window used to detect completion")
	fs.DurationVar(&cfg.InteractiveWait, "interactive-wait", cfg.InteractiveWait, "How long to wait for the page to become interactive")
	fs.StringVar(&cfg.ProfileDir, "profile-dir", defaultChromeUserDataDir(), "Chrome user data directory")
	fs.StringVar(&cfg.ProfileName, "profile-name", cfg.ProfileName, "Chrome profile directory name")
	fs.StringVar(&cfg.ProfileMode, "profile-mode", cfg.ProfileMode, "Chrome profile strategy: copy, direct, or persistent")
	fs.StringVar(&cfg.CLIProfileDir, "cli-profile-dir", cfg.CLIProfileDir, "Persistent CLI-owned Chrome user data directory")
	fs.StringVar(&cfg.ChromePath, "chrome-path", "", "Chrome executable path")
	fs.BoolVar(&cfg.Headless, "headless", false, "Run Chrome in headless mode")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Prompt = strings.TrimSpace(cfg.Prompt)
	if cfg.Prompt == "" {
		return Config{}, errors.New("missing required --prompt")
	}
	cfg.ProfileDir = strings.TrimSpace(cfg.ProfileDir)
	cfg.ProfileName = strings.TrimSpace(cfg.ProfileName)
	cfg.ProfileMode = strings.TrimSpace(strings.ToLower(cfg.ProfileMode))
	cfg.CLIProfileDir = strings.TrimSpace(cfg.CLIProfileDir)

	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("--timeout must be positive")
	}
	if cfg.StableWindow <= 0 {
		return Config{}, fmt.Errorf("--stable-window must be positive")
	}
	if cfg.InteractiveWait <= 0 {
		return Config{}, fmt.Errorf("--interactive-wait must be positive")
	}
	if cfg.ProfileMode != ProfileModeCopy && cfg.ProfileMode != ProfileModeDirect && cfg.ProfileMode != ProfileModePersistent {
		return Config{}, fmt.Errorf("--profile-mode must be one of: %s, %s, %s", ProfileModeCopy, ProfileModeDirect, ProfileModePersistent)
	}
	if cfg.ProfileMode == ProfileModePersistent && cfg.CLIProfileDir == "" {
		return Config{}, fmt.Errorf("--cli-profile-dir must not be empty in persistent mode")
	}

	cfg.ProfileName = resolveProfileName(cfg.ProfileDir, cfg.ProfileName)

	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(
			cfg.DefaultOutputBase,
			fmt.Sprintf("doubao-%s", time.Now().Format("20060102-150405")),
		)
	}

	return cfg, nil
}

func defaultChromeUserDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return filepath.Join(home, "AppData", "Local", "Google", "Chrome", "User Data")
		}
		return filepath.Join(localAppData, "Google", "Chrome", "User Data")
	default:
		return filepath.Join(home, ".config", "google-chrome")
	}
}

func defaultCLIChromeUserDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "get-doubao-pic", "chrome-user-data")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return filepath.Join(home, "AppData", "Local", "get-doubao-pic", "chrome-user-data")
		}
		return filepath.Join(localAppData, "get-doubao-pic", "chrome-user-data")
	default:
		return filepath.Join(home, ".config", "get-doubao-pic", "chrome-user-data")
	}
}

func resolveProfileName(profileDir string, requested string) string {
	if requested != "" && !strings.EqualFold(requested, ProfileNameAuto) {
		return requested
	}

	if detected, err := detectLastUsedProfile(profileDir); err == nil && detected != "" {
		return detected
	}

	return "Default"
}

func detectLastUsedProfile(profileDir string) (string, error) {
	if profileDir == "" {
		return "", fmt.Errorf("profile-dir is empty")
	}

	localStatePath := filepath.Join(profileDir, "Local State")
	payload, err := os.ReadFile(localStatePath)
	if err != nil {
		return "", err
	}

	var state struct {
		Profile struct {
			LastUsed string `json:"last_used"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		return "", fmt.Errorf("parse %s: %w", localStatePath, err)
	}

	lastUsed := strings.TrimSpace(state.Profile.LastUsed)
	if lastUsed == "" {
		return "", fmt.Errorf("profile.last_used is empty")
	}
	return lastUsed, nil
}
