package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
)

// Octokit は Usage Reports API を呼び出すクライアントです。
type Octokit struct {
	client *ghapi.RESTClient
}

// NewOctokit は REST クライアントから Octokit を構築します。
func NewOctokit(client *ghapi.RESTClient) *Octokit {
	return &Octokit{client: client}
}

// CreateReport はレポート生成を要求します。
func (octokit *Octokit) CreateReport(ctx context.Context, enterprise string, req CreateReportRequest) (*ReportExport, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report creation request: %w", err)
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
