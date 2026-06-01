// Package api は Usage Reports API のクライアントと型を提供します。
package api

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
