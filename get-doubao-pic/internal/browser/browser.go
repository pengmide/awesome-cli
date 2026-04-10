package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/doubao"
)

const chatURL = "https://www.doubao.com/chat/"

var (
	ErrChromeNotFound      = errors.New("chrome not found")
	ErrInteractiveNotReady = errors.New("doubao page not interactive")
	ErrLoginRequired       = errors.New("doubao login required")
	ErrProfileInUse        = errors.New("chrome profile is already in use")
	ErrTimeout             = errors.New("wait for images timeout")
)

const (
	interactivePollInterval    = 800 * time.Millisecond
	requiredReadyPolls         = 2
	requiredLoginRequiredPolls = 2
)

type Session struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
	cleanup     func() error
	collector   *doubao.Collector
	reqMetaMu   sync.RWMutex
	reqMeta     map[network.RequestID]requestMeta
}

type requestMeta struct {
	URL       string
	Timestamp time.Time
}

func NewSession(parent context.Context, cfg config.Config, collector *doubao.Collector) (*Session, error) {
	chromePath, err := resolveChromePath(cfg.ChromePath)
	if err != nil {
		return nil, err
	}
	prepared, err := prepareUserDataDir(cfg)
	if err != nil {
		return nil, err
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", cfg.Headless),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("user-data-dir", prepared.userDataDir),
		chromedp.Flag("profile-directory", cfg.ProfileName),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-features", "Translate,OptimizationHints"),
		chromedp.Flag("enable-automation", false),
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(parent, opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	session := &Session{
		ctx:         ctx,
		cancel:      cancel,
		allocCancel: allocCancel,
		cleanup:     prepared.cleanup,
		collector:   collector,
		reqMeta:     make(map[network.RequestID]requestMeta),
	}
	session.listen()

	if err := chromedp.Run(ctx,
		network.Enable(),
		network.SetBypassServiceWorker(true),
	); err != nil {
		cancel()
		allocCancel()
		_ = prepared.cleanup()
		if cfg.ProfileMode == config.ProfileModeDirect && looksLikeProfileInUseError(err) {
			return nil, ErrProfileInUse
		}
		return nil, fmt.Errorf("enable network domain: %w", err)
	}

	return session, nil
}

func (s *Session) Close() {
	s.cancel()
	s.allocCancel()
	if s.cleanup != nil {
		_ = s.cleanup()
	}
}

func (s *Session) Run(ctx context.Context, cfg config.Config) ([]doubao.ImageAsset, error) {
	if err := chromedp.Run(s.ctx,
		chromedp.Navigate(chatURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("open doubao chat page: %w", err)
	}

	if err := waitForInteractive(s.ctx, cfg.InteractiveWait); err != nil {
		return nil, err
	}

	if err := installJSONHook(s.ctx); err != nil {
		return nil, fmt.Errorf("install fallback json hook: %w", err)
	}

	s.collector.Reset()
	s.collector.Arm()
	stopPolling := make(chan struct{})
	defer close(stopPolling)
	go pollHookURLs(s.ctx, s.collector, stopPolling)

	if err := submitPrompt(s.ctx, cfg.Prompt); err != nil {
		return nil, err
	}

	assets, err := s.collector.Wait(ctx, cfg.StableWindow, cfg.Timeout)
	if err != nil {
		if errors.Is(err, doubao.ErrTimeout) {
			return nil, ErrTimeout
		}
		return nil, err
	}
	return assets, nil
}

func (s *Session) listen() {
	chromedp.ListenTarget(s.ctx, func(ev interface{}) {
		switch event := ev.(type) {
		case *network.EventResponseReceived:
			if event.Type != network.ResourceTypeFetch && event.Type != network.ResourceTypeXHR {
				return
			}
			s.reqMetaMu.Lock()
			s.reqMeta[event.RequestID] = requestMeta{
				URL:       event.Response.URL,
				Timestamp: time.Now(),
			}
			s.reqMetaMu.Unlock()
		case *network.EventLoadingFinished:
			go s.handleLoadingFinished(event.RequestID)
		case *network.EventLoadingFailed:
			s.reqMetaMu.Lock()
			delete(s.reqMeta, event.RequestID)
			s.reqMetaMu.Unlock()
		}
	})
}

func (s *Session) handleLoadingFinished(requestID network.RequestID) {
	s.reqMetaMu.RLock()
	metadata, ok := s.reqMeta[requestID]
	s.reqMetaMu.RUnlock()
	if !ok || !s.collector.IsArmed() {
		return
	}

	body, err := network.GetResponseBody(requestID).Do(s.ctx)
	if err != nil || len(body) == 0 {
		return
	}

	s.collector.AddFromPayload(string(body), metadata.URL)
}

func resolveChromePath(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("chrome-path %s: %w", override, err)
		}
		return override, nil
	}

	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	for _, path := range knownChromePaths() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", ErrChromeNotFound
}

func knownChromePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("PROGRAMFILES")
		programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
		return []string{
			filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
		}
	default:
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
	}
}

func waitForInteractive(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	readyPolls := 0
	loginPolls := 0
	lastReason := ""

	for time.Now().Before(deadline) {
		state := interactiveState{}
		if err := chromedp.Run(ctx, chromedp.Evaluate(detectInteractiveJS, &state)); err != nil {
			return fmt.Errorf("evaluate interactive state: %w", err)
		}
		lastReason = state.Reason

		if state.Ready {
			readyPolls++
			loginPolls = 0
			if readyPolls >= requiredReadyPolls {
				return nil
			}
		} else {
			readyPolls = 0
		}

		if state.LoginRequired {
			loginPolls++
			if loginPolls >= requiredLoginRequiredPolls {
				return wrapStateError(ErrLoginRequired, state.Reason)
			}
		} else {
			loginPolls = 0
		}

		time.Sleep(interactivePollInterval)
	}
	return wrapStateError(ErrInteractiveNotReady, lastReason)
}

func submitPrompt(ctx context.Context, prompt string) error {
	result := submitResult{}
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(submitPromptJS(prompt), &result)); err != nil {
		return fmt.Errorf("submit prompt: %w", err)
	}
	if !result.OK {
		reason := strings.TrimSpace(result.Reason)
		lowerReason := strings.ToLower(reason)
		if strings.HasPrefix(lowerReason, "login-required:") {
			return wrapStateError(ErrLoginRequired, strings.TrimSpace(reason[len("login-required:"):]))
		}
		if strings.HasPrefix(lowerReason, "not-ready:") {
			return wrapStateError(ErrInteractiveNotReady, strings.TrimSpace(reason[len("not-ready:"):]))
		}
		if strings.Contains(lowerReason, "login") {
			return wrapStateError(ErrLoginRequired, reason)
		}
		return fmt.Errorf("submit prompt failed: %s", reason)
	}
	return nil
}

func wrapStateError(base error, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return base
	}
	return fmt.Errorf("%w: %s", base, reason)
}

func looksLikeProfileInUseError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "existing browser session") ||
		strings.Contains(message, "profile appears to be in use") ||
		strings.Contains(err.Error(), "正在现有的浏览器会话中打开")
}
