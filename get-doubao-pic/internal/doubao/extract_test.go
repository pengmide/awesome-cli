package doubao

import "testing"

func TestExtractImageAssetsFromJSON(t *testing.T) {
	payload := `{"data":{"creations":[[{"image":{"image_ori_raw":{"url":"https://img.example.com/a.png"}}},{"image":{"image_ori_raw":{"url":"https://img.example.com/b.png"}}}]]}}`

	assets := ExtractImageAssets(payload, "https://api.example.com/result")
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
	if assets[0].URL != "https://img.example.com/a.png" {
		t.Fatalf("unexpected first url: %s", assets[0].URL)
	}
	if assets[1].URL != "https://img.example.com/b.png" {
		t.Fatalf("unexpected second url: %s", assets[1].URL)
	}
}

func TestExtractImageAssetsFromEventStream(t *testing.T) {
	payload := "event: message\ndata: {\"creations\":[[{\"image\":{\"image_ori_raw\":{\"url\":\"https://img.example.com/c.png\"}}}]]}\n\n"

	assets := ExtractImageAssets(payload, "https://api.example.com/stream")
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].URL != "https://img.example.com/c.png" {
		t.Fatalf("unexpected url: %s", assets[0].URL)
	}
}
