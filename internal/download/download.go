// Package download は HTTP ダウンロードとリトライ機能を提供します。
package download

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	defaultHTTPRequestTimeout     = 2 * time.Minute
	defaultTLSHandshakeTimeout    = 30 * time.Second
	defaultResponseHeaderTimeout  = 30 * time.Second
	defaultDownloadRetryCount     = 3
	defaultDownloadRetryBaseDelay = 1 * time.Second
)

// HTTPDownloader は HTTP GET で CSV をダウンロードします。
type HTTPDownloader struct {
	client         *http.Client
	retryCount     int
	retryBaseDelay time.Duration
}

// NewHTTPDownloader は HTTP クライアントからダウンローダーを構築します。
// client が nil の場合はデフォルトの HTTP クライアントを遅延生成します。
func NewHTTPDownloader(client *http.Client) *HTTPDownloader {
	if client == nil {
		client = newDefaultHTTPClient()
	}

	return &HTTPDownloader{
		client:         client,
		retryCount:     defaultDownloadRetryCount,
		retryBaseDelay: defaultDownloadRetryBaseDelay,
	}
}

// Download は指定 URL の CSV を取得します。
func (d *HTTPDownloader) Download(ctx context.Context, url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < d.retryCount; attempt++ {
		data, err := d.downloadOnce(ctx, url)
		if err == nil {
			return data, nil
		}

		lastErr = err
		if !shouldRetryDownload(err) || attempt == d.retryCount-1 {
			break
		}

		if err := waitBeforeRetry(ctx, d.retryBaseDelay, attempt); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

// downloadOnce は 1 回の HTTP リクエストで CSV を取得します。
func (d *HTTPDownloader) downloadOnce(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build download request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// newDefaultHTTPClient はレポートダウンロード向けの既定 HTTP クライアントを構築します。
func newDefaultHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12

	return &http.Client{
		Transport: transport,
		Timeout:   defaultHTTPRequestTimeout,
	}
}

// shouldRetryDownload は一時的なダウンロードエラーのみ再試行対象にします。
func shouldRetryDownload(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

// waitBeforeRetry は指数バックオフで次の再試行まで待機します。
func waitBeforeRetry(ctx context.Context, baseDelay time.Duration, attempt int) error {
	delay := baseDelay * time.Duration(1<<attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
