package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseUsageCSV_Detailed は detailed CSV を正しく解釈できることを検証します。
func TestParseUsageCSV_Detailed(t *testing.T) {
	input := strings.Join([]string{
		"date,product,sku,quantity,unit_type,applied_cost_per_quantity,gross_amount,discount_amount,net_amount,username,organization,repository,workflow_path,cost_center_name",
		"2026-04-01,actions,actions_linux,10,minutes,0.006,1.2,0.2,1.0,octocat,octo-org,repo,.github/workflows/ci.yml,cc-1",
	}, "\n") + "\n"

	records, format, err := parseUsageCSV(strings.NewReader(input))

	require.NoError(t, err)
	require.Equal(t, usageCSVFormatDetailed, format)
	require.Equal(t, []usageRecord{{
		Product:        "actions",
		GrossAmount:    1.2,
		DiscountAmount: 0.2,
		NetAmount:      1.0,
		Username:       "octocat",
		Organization:   "octo-org",
		CostCenterName: "cc-1",
	}}, records)
}

// TestParseUsageCSV_Summarized は summarized CSV を正しく解釈できることを検証します。
func TestParseUsageCSV_Summarized(t *testing.T) {
	input := strings.Join([]string{
		"date,product,sku,quantity,unit_type,applied_cost_per_quantity,gross_amount,discount_amount,net_amount,organization,repository,cost_center_name",
		"2026-04-01,copilot,copilot_business,2,seats,19,38,3,35,octo-org,repo,cc-1",
	}, "\n") + "\n"

	records, format, err := parseUsageCSV(strings.NewReader(input))

	require.NoError(t, err)
	require.Equal(t, usageCSVFormatSummarized, format)
	require.Equal(t, []usageRecord{{
		Product:        "copilot",
		GrossAmount:    38,
		DiscountAmount: 3,
		NetAmount:      35,
		Username:       "",
		Organization:   "octo-org",
		CostCenterName: "cc-1",
	}}, records)
}

// TestParseUsageCSV_EmptyCSV は空 CSV を拒否することを検証します。
func TestParseUsageCSV_EmptyCSV(t *testing.T) {
	records, format, err := parseUsageCSV(strings.NewReader(""))

	require.Error(t, err)
	require.Nil(t, records)
	require.Equal(t, usageCSVFormat(""), format)
	require.EqualError(t, err, "csv is empty")
}

// TestParseUsageCSV_UTF8BOM は UTF-8 BOM 付き CSV を読み取れることを検証します。
func TestParseUsageCSV_UTF8BOM(t *testing.T) {
	input := "\uFEFF" + strings.Join([]string{
		"\"date\",\"product\",\"sku\",\"quantity\",\"unit_type\",\"applied_cost_per_quantity\",\"gross_amount\",\"discount_amount\",\"net_amount\",\"organization\",\"repository\",\"cost_center_name\"",
		"\"2026-04-01\",\"actions\",\"actions_linux\",\"10\",\"minutes\",\"0.006\",\"12\",\"2\",\"10\",\"org-a\",\"repo-a\",\"cc-1\"",
	}, "\n") + "\n"

	records, format, err := parseUsageCSV(strings.NewReader(input))

	require.NoError(t, err)
	require.Equal(t, usageCSVFormatSummarized, format)
	require.Equal(t, []usageRecord{{
		Product:        "actions",
		GrossAmount:    12,
		DiscountAmount: 2,
		NetAmount:      10,
		Organization:   "org-a",
		CostCenterName: "cc-1",
	}}, records)
}

// TestAggregateUsageRecords_ByOrg は product ごとに org 集計できることを検証します。
func TestAggregateUsageRecords_ByOrg(t *testing.T) {
	records := []usageRecord{
		{Product: "actions", GrossAmount: 12, DiscountAmount: 2, NetAmount: 10, Organization: "org-a", CostCenterName: "cc-1", Username: "alice"},
		{Product: "actions", GrossAmount: 6, DiscountAmount: 1, NetAmount: 5, Organization: "org-b", CostCenterName: "cc-1", Username: "bob"},
		{Product: "actions", GrossAmount: 3, DiscountAmount: 0, NetAmount: 3, Organization: "org-a", CostCenterName: "cc-2", Username: "alice"},
		{Product: "copilot", GrossAmount: 7, DiscountAmount: 0, NetAmount: 7, Organization: "org-a", CostCenterName: "cc-3", Username: "carol"},
	}

	result, err := aggregateUsageRecords(records, usageCSVFormatDetailed, aggregateGroupingOrg)

	require.NoError(t, err)
	require.Equal(t, []aggregateRow{
		{Key: "org-a", GrossAmount: 15, DiscountAmount: 2, NetAmount: 13, Ratio: 13.0 / 18.0},
		{Key: "org-b", GrossAmount: 6, DiscountAmount: 1, NetAmount: 5, Ratio: 5.0 / 18.0},
	}, result["actions"])
	require.Equal(t, []aggregateRow{{Key: "org-a", GrossAmount: 7, DiscountAmount: 0, NetAmount: 7, Ratio: 1}}, result["copilot"])
}

// TestAggregateUsageRecords_UserRequiresDetailed は summarized CSV で user 集計を拒否することを検証します。
func TestAggregateUsageRecords_UserRequiresDetailed(t *testing.T) {
	records := []usageRecord{{Product: "actions", NetAmount: 10, Organization: "org-a"}}

	result, err := aggregateUsageRecords(records, usageCSVFormatSummarized, aggregateGroupingUser)

	require.Error(t, err)
	require.Nil(t, result)
	require.EqualError(t, err, "--group-by user requires a detailed CSV with username column")
}

// TestAggregateUsageRecords_EmptyValuesBecomeUnassigned は空の集計キーを補完することを検証します。
func TestAggregateUsageRecords_EmptyValuesBecomeUnassigned(t *testing.T) {
	records := []usageRecord{{Product: "actions", GrossAmount: 5, DiscountAmount: 0, NetAmount: 5, Organization: "org-a", CostCenterName: "", Username: ""}}

	result, err := aggregateUsageRecords(records, usageCSVFormatDetailed, aggregateGroupingCostCenter)

	require.NoError(t, err)
	require.Equal(t, []aggregateRow{{Key: unassignedGroupValue, GrossAmount: 5, DiscountAmount: 0, NetAmount: 5, Ratio: 1}}, result["actions"])
}

// TestAggregateUsageRecords_ZeroTotalNetAmount は比率の分母が 0 の場合に 0 を返すことを検証します。
func TestAggregateUsageRecords_ZeroTotalNetAmount(t *testing.T) {
	records := []usageRecord{
		{Product: "actions", GrossAmount: 2, DiscountAmount: 2, NetAmount: 0, Organization: "org-a"},
		{Product: "actions", GrossAmount: 3, DiscountAmount: 3, NetAmount: 0, Organization: "org-b"},
	}

	result, err := aggregateUsageRecords(records, usageCSVFormatDetailed, aggregateGroupingOrg)

	require.NoError(t, err)
	require.Equal(t, []aggregateRow{
		{Key: "org-a", GrossAmount: 2, DiscountAmount: 2, NetAmount: 0, Ratio: 0},
		{Key: "org-b", GrossAmount: 3, DiscountAmount: 3, NetAmount: 0, Ratio: 0},
	}, result["actions"])
}

// TestFormatAggregatedCSV は集計結果を CSV として出力できることを検証します。
func TestFormatAggregatedCSV(t *testing.T) {
	rows := []aggregateRow{
		{Key: "org-a", GrossAmount: 15, DiscountAmount: 2, NetAmount: 13, Ratio: 13.0 / 18.0},
		{Key: "org-b", GrossAmount: 6, DiscountAmount: 1, NetAmount: 5, Ratio: 5.0 / 18.0},
	}

	content, err := formatAggregatedCSV(rows, aggregateGroupingOrg)

	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"org,gross_amount,discount_amount,net_amount,ratio",
		"org-a,15,2,13,0.722222",
		"org-b,6,1,5,0.277778",
		"",
	}, "\n"), string(content))
}

// TestParseAggregateGrouping は許容する集計単位を正規化できることを検証します。
func TestParseAggregateGrouping(t *testing.T) {
	grouping, err := parseAggregateGrouping("cost-center")

	require.NoError(t, err)
	require.Equal(t, aggregateGroupingCostCenter, grouping)
}

// TestParseAggregateGrouping_InvalidValue は不正な集計単位を拒否することを検証します。
func TestParseAggregateGrouping_InvalidValue(t *testing.T) {
	grouping, err := parseAggregateGrouping("repo")

	require.Error(t, err)
	require.Equal(t, aggregateGrouping(""), grouping)
	require.EqualError(t, err, "--group-by must be one of: org, cost_center, user")
}

// TestBuildAggregateOutputDirName は入力ファイル名から共通ディレクトリ名を作ることを検証します。
func TestBuildAggregateOutputDirName(t *testing.T) {
	result := buildAggregateOutputDirName("reports/GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30_summarized.csv")

	require.Equal(t, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", result)
}
