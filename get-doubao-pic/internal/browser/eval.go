package browser

import "fmt"

type interactiveState struct {
	Ready         bool   `json:"ready"`
	LoginRequired bool   `json:"loginRequired"`
	Reason        string `json:"reason"`
	Path          string `json:"path"`
	HasChatRoute  bool   `json:"hasChatRoute"`
	InputCount    int    `json:"inputCount"`
	SendAvailable bool   `json:"sendAvailable"`
	SendVisible   bool   `json:"sendVisible"`
	SendEnabled   bool   `json:"sendEnabled"`
	LoginSignals  int    `json:"loginSignals"`
}

type submitResult struct {
	OK       bool   `json:"ok"`
	Strategy string `json:"strategy"`
	Reason   string `json:"reason"`
}

const browserProbeJS = `(() => {
  const LOGIN_TOKENS = ['登录', 'login', 'sign in', '手机号', 'phone', '验证码', 'verification code', '短信'];
  const SEND_TOKENS = ['发送', 'send', 'submit', '提交', '生成'];

  const lower = (value) => String(value || '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();

  const isVisible = (el) => {
    if (!el) return false;
    const style = window.getComputedStyle(el);
    if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') {
      return false;
    }
    const rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  };

  const containsAny = (text, tokens) => {
    return tokens.some((token) => text.includes(token));
  };

  const textOf = (el) => {
    if (!el) return '';
    return lower([
      el.innerText || '',
      el.textContent || '',
      el.getAttribute && el.getAttribute('aria-label') || '',
      el.getAttribute && el.getAttribute('title') || '',
      el.getAttribute && el.getAttribute('placeholder') || '',
      el.getAttribute && el.getAttribute('data-testid') || '',
      el.getAttribute && el.getAttribute('name') || '',
      el.className || '',
    ].join(' '));
  };

  const isEditableInput = (el) => {
    if (!el || !isVisible(el)) return false;
    if (el.disabled || el.getAttribute('aria-disabled') === 'true') return false;
    if (el.readOnly || el.getAttribute('readonly') !== null) return false;
    if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) return true;
    if (el.isContentEditable) return true;
    return lower(el.getAttribute('role')) === 'textbox';
  };

  const findActionScope = (input) => {
    if (!input) return document.body;
    return input.closest('form, section, article, main, [data-testid], [class]') || document.body;
  };

  const collectLoginSignals = () => {
    const selectors = [
      'dialog',
      '[role="dialog"]',
      'form',
      'button',
      'a',
      '[role="button"]',
      'input[type="password"]',
      'input[type="tel"]',
      'input[type="email"]',
      'input[autocomplete="one-time-code"]',
      '[data-testid*="login" i]',
      '[class*="login" i]',
    ];

    let count = 0;
    const seen = new Set();
    for (const el of document.querySelectorAll(selectors.join(', '))) {
      if (!isVisible(el)) continue;
      if (seen.has(el)) continue;
      seen.add(el);

      if (el.matches && el.matches('input[type="password"], input[type="tel"], input[type="email"], input[autocomplete="one-time-code"]')) {
        count++;
      } else if (containsAny(textOf(el), LOGIN_TOKENS)) {
        count++;
      }

      if (count >= 3) {
        break;
      }
    }

    return count;
  };

  const inputScore = (input) => {
    const rect = input.getBoundingClientRect();
    const scopeText = textOf(findActionScope(input));
    let score = rect.bottom + Math.min(rect.width, window.innerWidth);

    if (rect.bottom >= window.innerHeight * 0.55) score += 400;
    if (rect.width >= Math.min(window.innerWidth * 0.3, 260)) score += 120;
    if (rect.height >= 28) score += 20;
    if (containsAny(scopeText, LOGIN_TOKENS)) score -= 600;

    return score;
  };

  const pickMainInput = () => {
    const candidates = Array.from(document.querySelectorAll('textarea, [contenteditable="true"], [role="textbox"]'))
      .filter((el) => isEditableInput(el));

    candidates.sort((a, b) => inputScore(b) - inputScore(a));
    return {
      input: candidates[0] || null,
      count: candidates.length,
    };
  };

  const containsSendText = (text) => containsAny(text, SEND_TOKENS);

  const scoreButton = (button, input) => {
    const rect = button.getBoundingClientRect();
    const inputRect = input.getBoundingClientRect();
    const text = textOf(button);

    let score = 0;
    if (containsAny(text, LOGIN_TOKENS)) score -= 300;
    if (containsSendText(text)) score += 200;
    if (button.type === 'submit') score += 40;
    if (button.querySelector && button.querySelector('svg')) score += 10;
    if (rect.right >= inputRect.right - 120) score += 20;
    if (Math.abs(rect.bottom - inputRect.bottom) <= 160) score += 20;
    if (rect.top >= inputRect.top - 80 && rect.bottom <= inputRect.bottom + 180) score += 20;
    if (!button.disabled && button.getAttribute('aria-disabled') !== 'true') score += 15;
    return score;
  };

  const inspectSend = (input) => {
    if (!input) {
      return {
        available: false,
        visible: false,
        enabled: false,
        button: null,
        reason: 'input-not-found',
      };
    }

    const scope = findActionScope(input);
    const buttons = Array.from(scope.querySelectorAll('button, [role="button"]'))
      .filter((el) => isVisible(el));

    let bestVisible = null;
    let bestVisibleScore = -Infinity;
    let bestEnabled = null;
    let bestEnabledScore = -Infinity;

    for (const button of buttons) {
      const score = scoreButton(button, input);
      if (score > bestVisibleScore) {
        bestVisible = button;
        bestVisibleScore = score;
      }
      if (!button.disabled && button.getAttribute('aria-disabled') !== 'true' && score > bestEnabledScore) {
        bestEnabled = button;
        bestEnabledScore = score;
      }
    }

    const visibleCandidate = bestVisibleScore > 0 ? bestVisible : null;
    const enabledCandidate = bestEnabledScore > 0 ? bestEnabled : null;
    const form = input.closest('form');
    const enterCapable = input instanceof HTMLTextAreaElement || input.isContentEditable || lower(input.getAttribute('role')) === 'textbox';
    const available = !!visibleCandidate || !!form || enterCapable;

    let reason = 'send-unavailable';
    if (enabledCandidate) {
      reason = 'send-enabled';
    } else if (visibleCandidate) {
      reason = 'send-visible-disabled';
    } else if (form) {
      reason = 'form-submit-available';
    } else if (enterCapable) {
      reason = 'enter-submit-available';
    }

    return {
      available,
      visible: !!visibleCandidate,
      enabled: !!enabledCandidate,
      button: enabledCandidate || visibleCandidate || null,
      reason,
    };
  };

  const readValue = (el) => {
    if (!el) return '';
    if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) {
      return String(el.value || '');
    }
    return String(el.innerText || el.textContent || '');
  };

  const inspect = () => {
    const picked = pickMainInput();
    const input = picked.input;
    const send = inspectSend(input);
    const path = window.location.pathname || '';
    const onChatRoute = path.includes('/chat');
    const loginSignals = collectLoginSignals();
    const inputRect = input ? input.getBoundingClientRect() : null;
    const bottomAnchored = !!inputRect && inputRect.bottom >= window.innerHeight * 0.55;

    let ready = false;
    let loginRequired = false;
    let reason = 'unknown';

    if (loginSignals > 0) {
      loginRequired = true;
      reason = 'login-ui-visible';
    } else if (!onChatRoute) {
      reason = 'not-chat-route';
    } else if (!input) {
      reason = 'chat-input-not-found';
    } else if (!bottomAnchored) {
      reason = 'chat-input-not-bottom-anchored';
    } else if (!send.available) {
      reason = 'send-unavailable';
    } else {
      ready = true;
      reason = 'ready';
    }

    return {
      ready,
      loginRequired,
      reason,
      path,
      hasChatRoute: onChatRoute,
      inputCount: picked.count,
      sendAvailable: send.available,
      sendVisible: send.visible,
      sendEnabled: send.enabled,
      loginSignals,
    };
  };

  const setValue = (el, value) => {
    el.focus();
    if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) {
      const prototype = Object.getPrototypeOf(el);
      const descriptor = Object.getOwnPropertyDescriptor(prototype, 'value');
      if (descriptor && descriptor.set) {
        descriptor.set.call(el, value);
      } else {
        el.value = value;
      }
      el.dispatchEvent(new Event('input', { bubbles: true }));
      el.dispatchEvent(new Event('change', { bubbles: true }));
      return;
    }

    el.textContent = value;
    el.dispatchEvent(new InputEvent('input', {
      bubbles: true,
      data: value,
      inputType: 'insertText',
    }));
  };

  const submit = (prompt) => {
    const state = inspect();
    if (state.loginRequired) {
      return { ok: false, strategy: '', reason: 'login-required:' + state.reason };
    }
    if (!state.ready) {
      return { ok: false, strategy: '', reason: 'not-ready:' + state.reason };
    }

    const picked = pickMainInput();
    const input = picked.input;
    if (!input) {
      return { ok: false, strategy: '', reason: 'input-not-found' };
    }

    setValue(input, prompt);
    const currentValue = readValue(input);
    if (!currentValue || !currentValue.includes(prompt)) {
      return { ok: false, strategy: '', reason: 'input-value-not-applied' };
    }

    const send = inspectSend(input);
    if (send.enabled && send.button) {
      send.button.click();
      return { ok: true, strategy: 'button', reason: send.reason };
    }

    const form = input.closest('form');
    if (form && typeof form.requestSubmit === 'function') {
      form.requestSubmit();
      return { ok: true, strategy: 'requestSubmit', reason: send.reason };
    }

    const enterCapable = input instanceof HTMLTextAreaElement || input.isContentEditable || lower(input.getAttribute('role')) === 'textbox';
    if (enterCapable) {
      ['keydown', 'keypress', 'keyup'].forEach((type) => {
        input.dispatchEvent(new KeyboardEvent(type, {
          key: 'Enter',
          code: 'Enter',
          keyCode: 13,
          which: 13,
          bubbles: true,
        }));
      });
      return { ok: true, strategy: 'enter', reason: send.reason };
    }

    return { ok: false, strategy: '', reason: 'submit-action-unavailable' };
  };

  return { inspect, submit };
})()`

const detectInteractiveJS = browserProbeJS + `.inspect()`

func submitPromptJS(prompt string) string {
	return browserProbeJS + fmt.Sprintf(".submit(%q)", prompt)
}

const installHookJS = `(() => {
  if (window.__doubaoImageCollectorInstalled) {
    return true;
  }

  window.__doubaoImageCollectorInstalled = true;
  window.__doubaoImageCollector = window.__doubaoImageCollector || { urls: [], seen: {} };
  const originalParse = JSON.parse.bind(JSON);

  const walk = (value, found) => {
    if (!value || typeof value !== 'object') return;
    if (Array.isArray(value)) {
      value.forEach((item) => walk(item, found));
      return;
    }
    const rawUrl = value && value.image && value.image.image_ori_raw && value.image.image_ori_raw.url;
    if (typeof rawUrl === 'string' && !found.has(rawUrl)) {
      found.add(rawUrl);
    }
    Object.values(value).forEach((item) => walk(item, found));
  };

  JSON.parse = function patchedParse(text, reviver) {
    const parsed = originalParse(text, reviver);
    try {
      if (typeof text === 'string' && (text.includes('creations') || text.includes('image_ori_raw'))) {
        const found = new Set();
        walk(parsed, found);
        found.forEach((url) => {
          if (!window.__doubaoImageCollector.seen[url]) {
            window.__doubaoImageCollector.seen[url] = true;
            window.__doubaoImageCollector.urls.push(url);
          }
        });
      }
    } catch (error) {
      console.warn('doubao image collector hook failed', error);
    }
    return parsed;
  };

  return true;
})()`

const drainHookURLsJS = `(() => {
  const collector = window.__doubaoImageCollector;
  if (!collector || !Array.isArray(collector.urls) || collector.urls.length === 0) {
    return [];
  }
  const urls = collector.urls.slice();
  collector.urls.length = 0;
  return urls;
})()`
