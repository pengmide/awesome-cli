package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/doubao"
)

func installJSONHook(ctx context.Context) error {
	var ok bool
	if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(installHookJS, &ok)); err != nil {
		return fmt.Errorf("evaluate install hook script: %w", err)
	}
	if !ok {
		return fmt.Errorf("hook script returned false")
	}
	return nil
}

func pollHookURLs(ctx context.Context, collector *doubao.Collector, stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			if !collector.IsArmed() {
				continue
			}
			var urls []string
			if err := chromedp.Run(ctx, chromedp.EvaluateAsDevTools(drainHookURLsJS, &urls)); err != nil {
				continue
			}
			for _, rawURL := range urls {
				collector.AddFromPayload(`{"image":{"image_ori_raw":{"url":"`+rawURL+`"}}}`, "hook://json-parse")
			}
		}
	}
}
