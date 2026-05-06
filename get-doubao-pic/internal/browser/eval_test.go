package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestDetectInteractiveJSReadyChatComposer(t *testing.T) {
	state := evalInteractiveState(t, "/chat/", `<!doctype html>
<html>
  <body>
    <div style="position:fixed;left:24px;right:24px;bottom:24px;">
      <textarea style="width:100%;height:96px;"> </textarea>
      <button aria-label="发送">发送</button>
    </div>
  </body>
</html>`)

	if !state.Ready {
		t.Fatalf("expected ready state, got %#v", state)
	}
	if state.LoginRequired {
		t.Fatalf("expected logged-in state, got %#v", state)
	}
	if state.Reason != "ready" {
		t.Fatalf("expected ready reason, got %#v", state)
	}
}

func TestDetectInteractiveJSBlocksVisibleLoginUI(t *testing.T) {
	state := evalInteractiveState(t, "/chat/", `<!doctype html>
<html>
  <body>
    <div role="dialog" style="position:fixed;inset:0;background:rgba(0,0,0,.4);">
      <button style="margin:120px auto;display:block;">登录</button>
    </div>
    <div style="position:fixed;left:24px;right:24px;bottom:24px;">
      <textarea style="width:100%;height:96px;"></textarea>
      <button aria-label="发送">发送</button>
    </div>
  </body>
</html>`)

	if state.Ready {
		t.Fatalf("expected not ready when login UI is visible, got %#v", state)
	}
	if !state.LoginRequired {
		t.Fatalf("expected login required state, got %#v", state)
	}
	if state.Reason != "login-ui-visible" {
		t.Fatalf("expected login-ui-visible reason, got %#v", state)
	}
}

func TestDetectInteractiveJSRejectsNonBottomAnchoredInput(t *testing.T) {
	state := evalInteractiveState(t, "/chat/", `<!doctype html>
<html>
  <body>
    <div style="margin-top:120px;">
      <textarea style="width:480px;height:96px;"></textarea>
      <button aria-label="发送">发送</button>
    </div>
  </body>
</html>`)

	if state.Ready {
		t.Fatalf("expected not ready for non-bottom-anchored input, got %#v", state)
	}
	if state.LoginRequired {
		t.Fatalf("expected no login requirement, got %#v", state)
	}
	if state.Reason != "chat-input-not-bottom-anchored" {
		t.Fatalf("expected chat-input-not-bottom-anchored reason, got %#v", state)
	}
}

func TestSubmitPromptJSRefusesWhenLoginUIVisible(t *testing.T) {
	result := evalSubmitResult(t, "/chat/", `<!doctype html>
<html>
  <body>
    <div role="dialog" style="position:fixed;inset:0;background:rgba(0,0,0,.4);">
      <button style="margin:120px auto;display:block;">登录</button>
    </div>
    <div style="position:fixed;left:24px;right:24px;bottom:24px;">
      <textarea style="width:100%;height:96px;"></textarea>
      <button aria-label="发送">发送</button>
    </div>
  </body>
</html>`, "hello")

	if result.OK {
		t.Fatalf("expected submit to be blocked, got %#v", result)
	}
	if result.Reason != "login-required:login-ui-visible" {
		t.Fatalf("unexpected submit result: %#v", result)
	}
}

func TestWaitForInteractiveReturnsReadyAfterLogin(t *testing.T) {
	ctx, cancel := newHeadlessBrowserContext(t)
	defer cancel()

	server := newFixtureServer(t, `<!doctype html>
<html>
  <body>
    <div id="login" role="dialog" style="position:fixed;inset:0;background:rgba(0,0,0,.4);">
      <button style="margin:120px auto;display:block;">登录</button>
    </div>
    <div style="position:fixed;left:24px;right:24px;bottom:24px;">
      <textarea style="width:100%;height:96px;"></textarea>
      <button aria-label="发送">发送</button>
    </div>
    <script>
      setTimeout(() => {
        const login = document.getElementById('login');
        if (login) login.remove();
      }, 900);
    </script>
  </body>
</html>`)
	defer server.Close()

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+"/chat/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := waitForInteractive(ctx, 4*time.Second, false)
	if err != nil {
		t.Fatalf("waitForInteractive returned error: %v", err)
	}
	if !result.ReadyAfterLogin {
		t.Fatalf("expected login transition to be reported, got %#v", result)
	}
}

func TestWaitForInteractiveReturnsDirectReadyWithoutLogin(t *testing.T) {
	ctx, cancel := newHeadlessBrowserContext(t)
	defer cancel()

	server := newFixtureServer(t, `<!doctype html>
<html>
  <body>
    <div style="position:fixed;left:24px;right:24px;bottom:24px;">
      <textarea style="width:100%;height:96px;"></textarea>
      <button aria-label="发送">发送</button>
    </div>
  </body>
</html>`)
	defer server.Close()

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+"/chat/"),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	result, err := waitForInteractive(ctx, 3*time.Second, false)
	if err != nil {
		t.Fatalf("waitForInteractive returned error: %v", err)
	}
	if result.ReadyAfterLogin {
		t.Fatalf("expected direct ready state without login transition, got %#v", result)
	}
}

func evalInteractiveState(t *testing.T, path string, html string) interactiveState {
	t.Helper()

	ctx, cancel := newHeadlessBrowserContext(t)
	defer cancel()

	server := newFixtureServer(t, html)
	defer server.Close()

	var state interactiveState
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+path),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(detectInteractiveJS, &state),
	); err != nil {
		t.Fatalf("evaluate interactive state: %v", err)
	}

	return state
}

func evalSubmitResult(t *testing.T, path string, html string, prompt string) submitResult {
	t.Helper()

	ctx, cancel := newHeadlessBrowserContext(t)
	defer cancel()

	server := newFixtureServer(t, html)
	defer server.Close()

	var result submitResult
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL+path),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.EvaluateAsDevTools(submitPromptJS(prompt), &result),
	); err != nil {
		t.Fatalf("evaluate submit prompt: %v", err)
	}

	return result
}

func newHeadlessBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	chromePath, err := resolveChromePath("")
	if err != nil {
		t.Skipf("chrome not available: %v", err)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(chromePath),
			chromedp.Flag("headless", true),
			chromedp.NoDefaultBrowserCheck,
			chromedp.NoFirstRun,
		)...,
	)
	ctx, cancel := chromedp.NewContext(allocCtx)

	return ctx, func() {
		cancel()
		allocCancel()
	}
}

func newFixtureServer(t *testing.T, html string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
}
