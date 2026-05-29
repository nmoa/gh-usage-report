package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHTTPDownloader_Download_RetriesTemporaryErrors は一時的なネットワークエラーで再試行することを検証します。
func TestHTTPDownloader_Download_RetriesTemporaryErrors(t *testing.T) {
	attemptCount := 0
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attemptCount++
			if attemptCount < 3 {
				return nil, timeoutError{}
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("header\nvalue\n")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	downloader := NewHTTPDownloader(client)
	downloader.retryBaseDelay = time.Millisecond

	result, err := downloader.Download(context.Background(), "https://example.com/report.csv")

	require.NoError(t, err)
	require.Equal(t, "header\nvalue\n", string(result))
	require.Equal(t, 3, attemptCount)
}

// TestHTTPDownloader_Download_DoesNotRetryContextErrors はコンテキストエラーを再試行しないことを検証します。
func TestHTTPDownloader_Download_DoesNotRetryContextErrors(t *testing.T) {
	attemptCount := 0
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attemptCount++
			return nil, context.DeadlineExceeded
		}),
	}

	downloader := NewHTTPDownloader(client)
	downloader.retryBaseDelay = time.Millisecond

	result, err := downloader.Download(context.Background(), "https://example.com/report.csv")

	require.Nil(t, result)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, attemptCount)
}

// TestHTTPDownloader_Download_ReturnsEnglishStatusError は HTTP エラー文言が英語であることを検証します。
func TestHTTPDownloader_Download_ReturnsEnglishStatusError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	downloader := NewHTTPDownloader(client)
	downloader.retryBaseDelay = time.Millisecond

	result, err := downloader.Download(context.Background(), "https://example.com/report.csv")

	require.Nil(t, result)
	require.EqualError(t, err, "download failed: status 502")
}

// TestShouldRetryDownload は再試行対象のエラー種別を検証します。
func TestShouldRetryDownload(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error is retried",
			err:      timeoutError{},
			expected: true,
		},
		{
			name:     "tls handshake timeout is retried",
			err:      tlsHandshakeTimeoutError{},
			expected: true,
		},
		{
			name:     "context deadline is not retried",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "plain error is not retried",
			err:      errors.New("permanent failure"),
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := shouldRetryDownload(testCase.err)
			require.Equal(t, testCase.expected, result)
		})
	}
}

// TestNewDefaultHTTPClient は既定の HTTP クライアント設定を検証します。
func TestNewDefaultHTTPClient(t *testing.T) {
	client := newDefaultHTTPClient()
	transport, ok := client.Transport.(*http.Transport)

	require.True(t, ok)
	require.Equal(t, defaultHTTPRequestTimeout, client.Timeout)
	require.Equal(t, defaultTLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	require.Equal(t, defaultResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	require.NotNil(t, transport.TLSClientConfig)
	require.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
}

// roundTripFunc は RoundTripper を関数で差し替えるテスト用アダプターです。
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip は関数呼び出しに委譲します。
func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

// timeoutError は一時的なタイムアウトエラーです。
type timeoutError struct{}

// Error はエラーメッセージを返します。
func (timeoutError) Error() string {
	return "i/o timeout"
}

// Timeout はタイムアウトエラーであることを返します。
func (timeoutError) Timeout() bool {
	return true
}

// Temporary は一時的なエラーであることを返します。
func (timeoutError) Temporary() bool {
	return true
}

// tlsHandshakeTimeoutError は TLS handshake timeout を模したエラーです。
type tlsHandshakeTimeoutError struct{}

// Error はエラーメッセージを返します。
func (tlsHandshakeTimeoutError) Error() string {
	return "net/http: TLS handshake timeout"
}

// Timeout はタイムアウトエラーであることを返します。
func (tlsHandshakeTimeoutError) Timeout() bool {
	return true
}

// Temporary は一時的なエラーであることを返します。
func (tlsHandshakeTimeoutError) Temporary() bool {
	return true
}

var _ net.Error = timeoutError{}
var _ net.Error = tlsHandshakeTimeoutError{}