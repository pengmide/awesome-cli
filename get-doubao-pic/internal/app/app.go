package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/browser"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/doubao"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/download"
)

var (
	ErrChromeNotFound  = errors.New("chrome executable not found")
	ErrProfileInUse    = errors.New("chrome profile is already in use; use --profile-mode copy or --profile-mode persistent, or close Chrome first")
	ErrInteractive     = errors.New("doubao page did not become interactive")
	ErrLoginRequired   = errors.New("doubao page is not logged in")
	ErrNoImages        = errors.New("no image URLs were captured")
	ErrTimeout         = errors.New("timed out waiting for image generation")
	ErrPartialDownload = errors.New("some images failed to download")
)

type Result struct {
	OutputDir  string
	Files      []string
	Warnings   []string
	WarningErr error
}

func Run(ctx context.Context, cfg config.Config) (Result, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output dir %s: %w", cfg.OutputDir, err)
	}

	collector := doubao.NewCollector()
	session, err := browser.NewSession(ctx, cfg, collector)
	if err != nil {
		if errors.Is(err, browser.ErrChromeNotFound) {
			return Result{}, ErrChromeNotFound
		}
		if errors.Is(err, browser.ErrProfileInUse) {
			return Result{}, ErrProfileInUse
		}
		return Result{}, err
	}
	defer session.Close()

	assets, err := session.Run(ctx, cfg)
	if err != nil {
		switch {
		case errors.Is(err, browser.ErrLoginRequired):
			return Result{}, loginRequiredError(cfg, err)
		case errors.Is(err, browser.ErrInteractiveNotReady):
			return Result{}, wrapUserVisibleError(ErrInteractive, err)
		case errors.Is(err, browser.ErrTimeout):
			return Result{}, ErrTimeout
		case errors.Is(err, context.DeadlineExceeded):
			return Result{}, ErrTimeout
		default:
			return Result{}, err
		}
	}
	if len(assets) == 0 {
		return Result{}, ErrNoImages
	}

	downloadCtx, cancel := context.WithTimeout(ctx, max(2*time.Minute, cfg.Timeout/2))
	defer cancel()

	files, warnings, err := download.DownloadAll(downloadCtx, cfg.OutputDir, assets)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		OutputDir: cfg.OutputDir,
		Files:     files,
		Warnings:  warnings,
	}
	if len(warnings) > 0 {
		result.WarningErr = ErrPartialDownload
	}
	return result, nil
}

func ExitCode(err error) int {
	switch {
	case err == nil:
		return config.ExitSuccess
	case errors.Is(err, ErrPartialDownload):
		return config.ExitPartial
	case errors.Is(err, ErrChromeNotFound),
		errors.Is(err, ErrProfileInUse),
		errors.Is(err, ErrInteractive),
		errors.Is(err, ErrLoginRequired),
		errors.Is(err, ErrNoImages),
		errors.Is(err, ErrTimeout):
		return 1
	default:
		return 1
	}
}

func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func loginRequiredError(cfg config.Config, cause error) error {
	if cfg.ProfileMode == config.ProfileModeCopy {
		return fmt.Errorf("%w: copy mode did not preserve a usable Doubao session; try --profile-mode persistent and sign in once; %v", ErrLoginRequired, cause)
	}
	return wrapUserVisibleError(ErrLoginRequired, cause)
}

func wrapUserVisibleError(base error, cause error) error {
	if cause == nil {
		return base
	}
	return fmt.Errorf("%w: %v", base, cause)
}
