package browser

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestSessionCloseIsIdempotent(t *testing.T) {
	chromePath, err := resolveChromePath("")
	if err != nil {
		t.Skipf("chrome not available: %v", err)
	}

	previousSettleDelay := preCloseSettleDelay
	previousCloseTimeout := gracefulCloseTimeout
	preCloseSettleDelay = 0
	gracefulCloseTimeout = 2 * time.Second
	defer func() {
		preCloseSettleDelay = previousSettleDelay
		gracefulCloseTimeout = previousCloseTimeout
	}()

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(chromePath),
			chromedp.Flag("headless", true),
			chromedp.NoDefaultBrowserCheck,
			chromedp.NoFirstRun,
		)...,
	)
	ctx, cancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(ctx,
		chromedp.Navigate("data:text/html,<html><body>close</body></html>"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		cancel()
		allocCancel()
		t.Fatalf("bootstrap browser: %v", err)
	}

	cleanupCalls := 0
	session := &Session{
		ctx:         ctx,
		cancel:      cancel,
		allocCancel: allocCancel,
		cleanup: func() error {
			cleanupCalls++
			return nil
		},
	}

	session.Close()
	session.Close()

	if cleanupCalls != 1 {
		t.Fatalf("expected cleanup to run once, got %d", cleanupCalls)
	}
}
