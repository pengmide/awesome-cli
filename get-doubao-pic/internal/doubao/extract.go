package doubao

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var rawURLPattern = regexp.MustCompile(`(?s)"image_ori_raw"\s*:\s*\{.*?"url"\s*:\s*"([^"]+)"`)

type ImageAsset struct {
	URL       string
	SourceURL string
}

type Collector struct {
	mu         sync.RWMutex
	armed      bool
	lastUpdate time.Time
	seen       map[string]ImageAsset
}

func NewCollector() *Collector {
	return &Collector{
		seen: make(map[string]ImageAsset),
	}
}

func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = false
	c.lastUpdate = time.Time{}
	c.seen = make(map[string]ImageAsset)
}

func (c *Collector) Arm() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = true
}

func (c *Collector) IsArmed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.armed
}

func (c *Collector) AddFromPayload(body string, sourceURL string) {
	assets := ExtractImageAssets(body, sourceURL)
	if len(assets) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.armed {
		return
	}

	updated := false
	for _, asset := range assets {
		if _, exists := c.seen[asset.URL]; exists {
			continue
		}
		c.seen[asset.URL] = asset
		updated = true
	}
	if updated {
		c.lastUpdate = time.Now()
	}
}

func (c *Collector) Snapshot() []ImageAsset {
	c.mu.RLock()
	defer c.mu.RUnlock()

	assets := make([]ImageAsset, 0, len(c.seen))
	for _, asset := range c.seen {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].URL < assets[j].URL
	})
	return assets
}

func (c *Collector) Wait(ctx context.Context, stableWindow time.Duration, overallTimeout time.Duration) ([]ImageAsset, error) {
	deadline := time.Now().Add(overallTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			assets := c.Snapshot()
			if len(assets) > 0 {
				c.mu.RLock()
				lastUpdate := c.lastUpdate
				c.mu.RUnlock()
				if !lastUpdate.IsZero() && time.Since(lastUpdate) >= stableWindow {
					return assets, nil
				}
			}
			if time.Now().After(deadline) {
				return nil, ErrTimeout
			}
		}
	}
}

var ErrTimeout = &collectorError{message: "collector timeout"}

type collectorError struct {
	message string
}

func (e *collectorError) Error() string {
	return e.message
}

func ExtractImageAssets(payload string, sourceURL string) []ImageAsset {
	if !strings.Contains(payload, "image_ori_raw") && !strings.Contains(payload, "creations") {
		return nil
	}

	seen := make(map[string]ImageAsset)
	for _, text := range candidateJSONTexts(payload) {
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			addAssetsFromParsed(parsed, sourceURL, seen)
		}
	}

	if len(seen) == 0 {
		matches := rawURLPattern.FindAllStringSubmatch(payload, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			url := strings.TrimSpace(match[1])
			if url == "" {
				continue
			}
			seen[url] = ImageAsset{URL: url, SourceURL: sourceURL}
		}
	}

	assets := make([]ImageAsset, 0, len(seen))
	for _, asset := range seen {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].URL < assets[j].URL
	})
	return assets
}

func candidateJSONTexts(payload string) []string {
	candidates := []string{payload}
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if line != "" && line != "[DONE]" {
				candidates = append(candidates, line)
			}
		}
	}
	return candidates
}

func addAssetsFromParsed(value any, sourceURL string, seen map[string]ImageAsset) {
	switch typed := value.(type) {
	case map[string]any:
		if imageValue, ok := typed["image"].(map[string]any); ok {
			if rawImage, ok := imageValue["image_ori_raw"].(map[string]any); ok {
				if url, ok := rawImage["url"].(string); ok && strings.TrimSpace(url) != "" {
					seen[url] = ImageAsset{URL: url, SourceURL: sourceURL}
				}
			}
		}
		for _, item := range typed {
			addAssetsFromParsed(item, sourceURL, seen)
		}
	case []any:
		for _, item := range typed {
			addAssetsFromParsed(item, sourceURL, seen)
		}
	}
}
