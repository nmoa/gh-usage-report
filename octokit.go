package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// ReportExport は Usage Reports API のレスポンスを表します。
type ReportExport struct {
	ID           string   `json:"id"`
	ReportType   string   `json:"report_type"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	Status       string   `json:"status"`
	DownloadURLs []string `json:"download_urls"`
	CreatedAt    string   `json:"created_at"`
	Actor        string   `json:"actor"`
}

// CreateReportRequest はレポート生成 API のリクエストボディです。
type CreateReportRequest struct {
	ReportType string `json:"report_type"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

// Octokit は Usage Reports API を呼び出すクライアントです。
type Octokit struct {
	client *api.RESTClient
}

// NewOctokit は REST クライアントから Octokit を構築します。
func NewOctokit(client *api.RESTClient) *Octokit {
	return &Octokit{client: client}
}

// CreateReport はレポート生成を要求します。
func (octokit *Octokit) CreateReport(ctx context.Context, enterprise string, req CreateReportRequest) (*ReportExport, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("レポート生成リクエストの JSON 化に失敗しました: %w", err)
	}

	url := fmt.Sprintf("enterprises/%s/settings/billing/reports", enterprise)
	response := ReportExport{}
	if err := octokit.client.DoWithContext(ctx, http.MethodPost, url, bytes.NewReader(body), &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetReport はレポート状態を取得します。
func (octokit *Octokit) GetReport(ctx context.Context, enterprise, reportID string) (*ReportExport, error) {
	url := fmt.Sprintf("enterprises/%s/settings/billing/reports/%s", enterprise, reportID)
	response := ReportExport{}
	if err := octokit.client.DoWithContext(ctx, http.MethodGet, url, nil, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
