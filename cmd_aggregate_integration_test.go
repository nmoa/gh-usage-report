package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAggregateCommand_RunByOrg は summarized CSV を org 単位に集計できることを検証します。
func TestAggregateCommand_RunByOrg(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30_summarized.csv")
	input := strings.Join([]string{
		"date,product,sku,quantity,unit_type,applied_cost_per_quantity,gross_amount,discount_amount,net_amount,organization,repository,cost_center_name",
		"2026-04-01,actions,actions_linux,10,minutes,0.006,12,2,10,org-a,repo-a,cc-1",
		"2026-04-01,actions,actions_linux,5,minutes,0.006,6,1,5,org-b,repo-b,cc-2",
		"2026-04-01,copilot,copilot_business,1,seats,19,9,0,9,org-a,repo-c,cc-3",
	}, "\n") + "\n"
	err := os.WriteFile(inputPath, []byte(input), outputFilePermission)
	require.NoError(t, err)

	cmd := newRootCmdWithAggregateDependencies(commandDependencies{}, newDefaultAggregateDependencies())
	var stderr bytes.Buffer
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"aggregate", "--input", inputPath, "--group-by", "org", "--output-dir", tmpDir})

	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	actionsPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", "actions_by_org.csv")
	copilotPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", "copilot_by_org.csv")
	actionsContent, err := os.ReadFile(actionsPath)
	require.NoError(t, err)
	copilotContent, err := os.ReadFile(copilotPath)
	require.NoError(t, err)

	require.Equal(t, strings.Join([]string{
		"org,gross_amount,discount_amount,net_amount,ratio",
		"org-a,12,2,10,0.666667",
		"org-b,6,1,5,0.333333",
		"",
	}, "\n"), string(actionsContent))
	require.Equal(t, strings.Join([]string{
		"org,gross_amount,discount_amount,net_amount,ratio",
		"org-a,9,0,9,1",
		"",
	}, "\n"), string(copilotContent))
	require.Contains(t, stderr.String(), "Saved aggregated CSVs to")
	require.NotContains(t, stderr.String(), "--enterprise")
	require.NotContains(t, stderr.String(), "レポート")
}

// TestAggregateCommand_RunByCostCenter は detailed CSV を cost_center 単位に集計できることを検証します。
func TestAggregateCommand_RunByCostCenter(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30_detailed.csv")
	input := strings.Join([]string{
		"date,product,sku,quantity,unit_type,applied_cost_per_quantity,gross_amount,discount_amount,net_amount,username,organization,repository,workflow_path,cost_center_name",
		"2026-04-01,actions,actions_linux,10,minutes,0.006,12,2,10,alice,org-a,repo-a,.github/workflows/ci.yml,cc-1",
		"2026-04-01,actions,actions_linux,5,minutes,0.006,6,1,5,bob,org-a,repo-b,.github/workflows/test.yml,",
	}, "\n") + "\n"
	err := os.WriteFile(inputPath, []byte(input), outputFilePermission)
	require.NoError(t, err)

	cmd := newRootCmdWithAggregateDependencies(commandDependencies{}, newDefaultAggregateDependencies())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"aggregate", "--input", inputPath, "--group-by", "cost_center", "--output-dir", tmpDir})

	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	actionsPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", "actions_by_cost_center.csv")
	actionsContent, err := os.ReadFile(actionsPath)
	require.NoError(t, err)
	require.Equal(t, strings.Join([]string{
		"cost_center,gross_amount,discount_amount,net_amount,ratio",
		"cc-1,12,2,10,0.666667",
		"(unassigned),6,1,5,0.333333",
		"",
	}, "\n"), string(actionsContent))
}

// TestAggregateCommand_RunByUser は detailed CSV を user 単位に集計できることを検証します。
func TestAggregateCommand_RunByUser(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30_detailed.csv")
	input := strings.Join([]string{
		"date,product,sku,quantity,unit_type,applied_cost_per_quantity,gross_amount,discount_amount,net_amount,username,organization,repository,workflow_path,cost_center_name",
		"2026-04-01,actions,actions_linux,10,minutes,0.006,12,2,10,alice,org-a,repo-a,.github/workflows/ci.yml,cc-1",
		"2026-04-01,actions,actions_linux,5,minutes,0.006,6,1,5,bob,org-a,repo-b,.github/workflows/test.yml,cc-1",
		"2026-04-01,actions,actions_linux,3,minutes,0.006,3,0,3,alice,org-b,repo-c,.github/workflows/test.yml,cc-2",
		"2026-04-01,copilot,copilot_business,1,seats,19,9,0,9,carol,org-c,repo-d,.github/workflows/test.yml,cc-3",
	}, "\n") + "\n"
	err := os.WriteFile(inputPath, []byte(input), outputFilePermission)
	require.NoError(t, err)

	cmd := newRootCmdWithAggregateDependencies(commandDependencies{}, newDefaultAggregateDependencies())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"aggregate", "--input", inputPath, "--group-by", "user", "--output-dir", tmpDir})

	err = cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	actionsPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", "actions_by_user.csv")
	copilotPath := filepath.Join(tmpDir, "GitHub_Usage_jp-ricoh_2026-04-01_to_2026-04-30", "copilot_by_user.csv")
	actionsContent, err := os.ReadFile(actionsPath)
	require.NoError(t, err)
	copilotContent, err := os.ReadFile(copilotPath)
	require.NoError(t, err)

	require.Equal(t, strings.Join([]string{
		"user,gross_amount,discount_amount,net_amount,ratio",
		"alice,15,2,13,0.722222",
		"bob,6,1,5,0.277778",
		"",
	}, "\n"), string(actionsContent))
	require.Equal(t, strings.Join([]string{
		"user,gross_amount,discount_amount,net_amount,ratio",
		"carol,9,0,9,1",
		"",
	}, "\n"), string(copilotContent))
}
