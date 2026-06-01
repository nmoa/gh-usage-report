package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
)

// newIntegrationDependencies は結合テスト用の依存を構築します。
func newIntegrationDependencies(t *testing.T, server *httptest.Server) commandDependencies {
	t.Helper()

	return commandDependencies{
		newRESTClient: func(authToken string, logOutput io.Writer) (*api.RESTClient, error) {
			return api.NewRESTClient(api.ClientOptions{
				AuthToken: "test-token",
				Host:      strings.TrimPrefix(server.URL, "https://"),
				Log:       logOutput,
				Transport: server.Client().Transport,
			})
		},
		downloader:   NewHTTPDownloader(server.Client()),
		pollInterval: time.Millisecond,
		mkdirAll:     os.MkdirAll,
		writeFile:    os.WriteFile,
	}
}

// TestRootCommand_Run は CLI からの一連のレポート取得フローを検証します。
func TestRootCommand_Run(t *testing.T) {
	type reportRequest struct {
		ReportType string `json:"report_type"`
		StartDate  string `json:"start_date"`
		EndDate    string `json:"end_date"`
	}

	var mutex sync.Mutex
	statuses := map[string]int{}
	createdRequests := []reportRequest{}
	enterpriseReportsPath := "/api/v3/enterprises/" + exampleEnterpriseSlug + "/settings/billing/reports"

	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == enterpriseReportsPath:
			request := reportRequest{}
			err := json.NewDecoder(r.Body).Decode(&request)
			require.NoError(t, err)

			mutex.Lock()
			createdRequests = append(createdRequests, request)
			mutex.Unlock()

			w.WriteHeader(http.StatusAccepted)
			_, err = w.Write([]byte(`{"id":"` + request.ReportType + `-1","status":"processing"}`))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == enterpriseReportsPath+"/detailed-1":
			mutex.Lock()
			statuses["detailed-1"]++
			count := statuses["detailed-1"]
			mutex.Unlock()

			switch count {
			case 1:
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"id":"detailed-1","status":"processing"}`))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{"id":"detailed-1","status":"completed","download_urls":["` + server.URL + `/download/detailed-1.csv","` + server.URL + `/download/detailed-2.csv"]}`))
				require.NoError(t, err)
			}
		case r.Method == http.MethodGet && r.URL.Path == enterpriseReportsPath+"/summarized-1":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"id":"summarized-1","status":"completed","download_urls":["` + server.URL + `/download/summarized.csv"]}`))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/download/detailed-1.csv":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("date,amount\n2026-04-01,10\n"))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/download/detailed-2.csv":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("date,amount\n2026-04-02,20\n"))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/download/summarized.csv":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("organization,amount\nexample,30\n"))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cmd := newRootCmd(newIntegrationDependencies(t, server))
	var stderr bytes.Buffer
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--enterprise", exampleEnterpriseSlug,
		"--year", "2026",
		"--month", "4",
		"--billing-cycle", "1",
		"--report-path", tmpDir,
		"--report-type", "both",
		"--timeout", "1",
	})

	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "Reporting range: 2026-04-01_to_2026-04-30")
	require.Contains(t, stderr.String(), "Generating detailed report")
	require.Contains(t, stderr.String(), "Waiting for report completion...")
	require.Contains(t, stderr.String(), "Report completed after")
	require.Contains(t, stderr.String(), "Downloaded detailed report file 1/2.")
	require.Contains(t, stderr.String(), "Downloaded detailed report file 2/2.")
	require.Contains(t, stderr.String(), "Generating summarized report")
	require.Contains(t, stderr.String(), "Downloaded summarized report file 1/1.")
	require.Contains(t, stderr.String(), "Saved: ")
	require.Contains(t, stderr.String(), "Saved reports to ")
	require.NotContains(t, stderr.String(), "* Request at")
	require.NotContains(t, stderr.String(), "* Request to")
	require.NotContains(t, stderr.String(), "対象期間")
	require.NotContains(t, stderr.String(), "保存完了")
	require.NotContains(t, stderr.String(), "レポートを生成します")
	require.Equal(t, []reportRequest{
		{ReportType: reportTypeDetailed, StartDate: "2026-04-01", EndDate: "2026-04-30"},
		{ReportType: reportTypeSummarized, StartDate: "2026-04-01", EndDate: "2026-04-30"},
	}, createdRequests)

	detailedPath := filepath.Join(tmpDir, "GitHub_Usage_"+exampleEnterpriseSlug+"_2026-04-01_to_2026-04-30_detailed.csv")
	detailedSecondPath := filepath.Join(tmpDir, "GitHub_Usage_"+exampleEnterpriseSlug+"_2026-04-01_to_2026-04-30_detailed_2.csv")
	summarizedPath := filepath.Join(tmpDir, "GitHub_Usage_"+exampleEnterpriseSlug+"_2026-04-01_to_2026-04-30_summarized.csv")

	detailedContent, err := os.ReadFile(detailedPath)
	require.NoError(t, err)
	detailedSecondContent, err := os.ReadFile(detailedSecondPath)
	require.NoError(t, err)
	summarizedContent, err := os.ReadFile(summarizedPath)
	require.NoError(t, err)

	require.Equal(t, "date,amount\n2026-04-01,10\n", string(detailedContent))
	require.Equal(t, "date,amount\n2026-04-02,20\n", string(detailedSecondContent))
	require.Equal(t, "organization,amount\nexample,30\n", string(summarizedContent))
}
