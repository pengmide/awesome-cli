package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
)

func TestLoginRequiredErrorCopyModeSuggestsPersistent(t *testing.T) {
	err := loginRequiredError(config.Config{ProfileMode: config.ProfileModeCopy}, errors.New("login-ui-visible"))
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("expected ErrLoginRequired wrapper, got %v", err)
	}
	if !strings.Contains(err.Error(), "--profile-mode persistent") {
		t.Fatalf("expected persistent mode guidance, got %q", err.Error())
	}
}

func TestWrapUserVisibleErrorPreservesBase(t *testing.T) {
	err := wrapUserVisibleError(ErrInteractive, errors.New("chat-input-not-found"))
	if !errors.Is(err, ErrInteractive) {
		t.Fatalf("expected wrapped interactive error, got %v", err)
	}
	if !strings.Contains(err.Error(), "chat-input-not-found") {
		t.Fatalf("expected detailed reason, got %q", err.Error())
	}
}
