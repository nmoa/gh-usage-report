package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

var httpDefaultClient = http.DefaultClient

// HTTPDownloader は HTTP GET で CSV をダウンロードします。
type HTTPDownloader struct {
	client *http.Client
}

// NewHTTPDownloader は HTTP クライアントからダウンローダーを構築します。
func NewHTTPDownloader(client *http.Client) *HTTPDownloader {
	return &HTTPDownloader{client: client}
}

// Download は指定 URL の CSV を取得します。
func (downloader *HTTPDownloader) Download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ダウンロードリクエストの生成に失敗しました: %w", err)
	}

	resp, err := downloader.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ダウンロードに失敗しました: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
