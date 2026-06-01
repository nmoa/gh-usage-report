package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
)

// newTestRESTClient は httptest サーバーへ向く REST クライアントを作ります。
func newTestRESTClient(t *testing.T, server *httptest.Server) *api.RESTClient {
	t.Helper()

	host := strings.TrimPrefix(server.URL, "https://")
	client, err := api.NewRESTClient(api.ClientOptions{
		AuthToken: "test-token",
		Host:      host,
		Transport: server.Client().Transport,
	})
	require.NoError(t, err)

	return client
}

// TestOctokit_CreateReport はレポート生成 API 呼び出しを検証します。
func TestOctokit_CreateReport(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotBody string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = string(body)

		w.WriteHeader(http.StatusAccepted)
		_, err = w.Write([]byte(`{"id":"report-1","status":"processing","report_type":"summarized","start_date":"2026-04-01","end_date":"2026-04-30"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	octokit := NewOctokit(newTestRESTClient(t, server))
	result, err := octokit.CreateReport(context.Background(), exampleEnterpriseSlug, CreateReportRequest{
		ReportType: reportTypeSummarized,
		StartDate:  "2026-04-01",
		EndDate:    "2026-04-30",
	})

	require.NoError(t, err)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/api/v3/enterprises/"+exampleEnterpriseSlug+"/settings/billing/reports", gotPath)
	require.JSONEq(t, `{"report_type":"summarized","start_date":"2026-04-01","end_date":"2026-04-30"}`, gotBody)
	require.Equal(t, "report-1", result.ID)
	require.Equal(t, "processing", result.Status)
}

// TestOctokit_GetReport はレポート状態取得 API 呼び出しを検証します。
func TestOctokit_GetReport(t *testing.T) {
	var gotMethod string
	var gotPath string

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"id":"report-1","status":"completed","download_urls":["https://example.com/report.csv"]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	octokit := NewOctokit(newTestRESTClient(t, server))
	result, err := octokit.GetReport(context.Background(), exampleEnterpriseSlug, "report-1")

	require.NoError(t, err)
	require.Equal(t, http.MethodGet, gotMethod)
	require.Equal(t, "/api/v3/enterprises/"+exampleEnterpriseSlug+"/settings/billing/reports/report-1", gotPath)
	require.Equal(t, []string{"https://example.com/report.csv"}, result.DownloadURLs)
}

// TestHTTPDownloader_Download は CSV ダウンロードを検証します。
func TestHTTPDownloader_Download(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("header\nvalue\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	downloader := NewHTTPDownloader(server.Client())
	result, err := downloader.Download(context.Background(), server.URL)

	require.NoError(t, err)
	require.Equal(t, "header\nvalue\n", string(result))
}
