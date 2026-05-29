package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDetermineReportTypes は有効なレポート種別の解釈を検証します。
func TestDetermineReportTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "detailed を単独で返す",
			input:    reportTypeDetailed,
			expected: []string{reportTypeDetailed},
		},
		{
			name:     "summarized を単独で返す",
			input:    reportTypeSummarized,
			expected: []string{reportTypeSummarized},
		},
		{
			name:     "both は detailed と summarized を返す",
			input:    reportTypeBoth,
			expected: []string{reportTypeDetailed, reportTypeSummarized},
		},
		{
			name:     "大文字小文字を無視する",
			input:    "Detailed",
			expected: []string{reportTypeDetailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := determineReportTypes(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestDetermineReportTypes_InvalidType は不正なレポート種別を拒否することを検証します。
func TestDetermineReportTypes_InvalidType(t *testing.T) {
	result, err := determineReportTypes("invalid")

	require.Error(t, err)
	require.Nil(t, result)
	require.EqualError(t, err, "--report-type must be one of: detailed, summarized, both")
}

// TestBuildFilename は CSV 出力ファイル名の組み立てを検証します。
func TestBuildFilename(t *testing.T) {
	billingCycle := NewBillingCycle(InputCycle{Year: 2026, Month: 4, BillingCycle: 1})
	enterprise := "octo-enterprise"

	tests := []struct {
		name       string
		reportType string
		index      int
		total      int
		expected   string
	}{
		{
			name:       "単一ファイルの detailed 名を返す",
			reportType: reportTypeDetailed,
			index:      0,
			total:      1,
			expected:   "GitHub_Usage_octo-enterprise_2026-04-01_to_2026-04-30_detailed.csv",
		},
		{
			name:       "複数ファイルの 2 件目に連番を付ける",
			reportType: reportTypeDetailed,
			index:      1,
			total:      2,
			expected:   "GitHub_Usage_octo-enterprise_2026-04-01_to_2026-04-30_detailed_2.csv",
		},
		{
			name:       "summarized 名を返す",
			reportType: reportTypeSummarized,
			index:      0,
			total:      1,
			expected:   "GitHub_Usage_octo-enterprise_2026-04-01_to_2026-04-30_summarized.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFilename(enterprise, billingCycle, tt.reportType, tt.index, tt.total)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildReportWaitingMessage はポーリング中メッセージの組み立てを検証します。
func TestBuildReportWaitingMessage(t *testing.T) {
	result := buildReportWaitingMessage(15 * time.Second)

	require.Equal(t, "Waiting for report completion... (15s elapsed)", result)
}

// TestBuildReportCompletedMessage はポーリング完了メッセージの組み立てを検証します。
func TestBuildReportCompletedMessage(t *testing.T) {
	result := buildReportCompletedMessage(15 * time.Second)

	require.Equal(t, "Report completed after 15s.", result)
}

// TestBuildDownloadingReportMessage はダウンロード中メッセージの組み立てを検証します。
func TestBuildDownloadingReportMessage(t *testing.T) {
	result := buildDownloadingReportMessage(reportTypeDetailed, 1, 2)

	require.Equal(t, "Downloading detailed report file 2/2...", result)
}

// TestBuildDownloadedReportMessage はダウンロード完了メッセージの組み立てを検証します。
func TestBuildDownloadedReportMessage(t *testing.T) {
	result := buildDownloadedReportMessage(reportTypeSummarized, 0, 1)

	require.Equal(t, "Downloaded summarized report file 1/1.", result)
}
