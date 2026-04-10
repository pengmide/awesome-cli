package download

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/doubao"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func DownloadAll(ctx context.Context, outputDir string, assets []doubao.ImageAsset) ([]string, []string, error) {
	client := &http.Client{}
	files := make([]string, 0, len(assets))
	warnings := make([]string, 0)
	usedNames := make(map[string]int)

	for _, asset := range assets {
		filePath, err := downloadOne(ctx, client, outputDir, asset, usedNames)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", asset.URL, err))
			continue
		}
		files = append(files, filePath)
	}

	if len(files) == 0 && len(warnings) > 0 {
		return nil, warnings, fmt.Errorf("all downloads failed")
	}
	return files, warnings, nil
}

func downloadOne(ctx context.Context, client *http.Client, outputDir string, asset doubao.ImageAsset, usedNames map[string]int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Referer", "https://www.doubao.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	name := deriveFileName(asset.URL, resp.Header.Get("Content-Type"))
	name = uniqueName(name, usedNames)
	fullPath := filepath.Join(outputDir, name)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}

	return fullPath, nil
}

func deriveFileName(rawURL string, contentType string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		name := path.Base(parsed.Path)
		name = strings.TrimSpace(name)
		if name != "" && name != "." && name != "/" {
			if filepath.Ext(name) == "" {
				name += extensionFromContentType(contentType)
			}
			return sanitizeFileName(name)
		}
	}

	return "image" + extensionFromContentType(contentType)
}

func extensionFromContentType(contentType string) string {
	if contentType == "" {
		return ".png"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ".png"
	}
	extensions, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(extensions) == 0 {
		return ".png"
	}
	return extensions[0]
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}

func uniqueName(name string, used map[string]int) string {
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s-%d%s", base, count+1, ext)
}
