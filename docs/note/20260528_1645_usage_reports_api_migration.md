# Usage Reports API への移行実装プラン

## 調査結果サマリ

### 旧実装の問題点

旧実装は `GET /enterprises/{enterprise}/settings/billing/usage?year=YYYY&month=MM` を使用していたが、
このAPIはデフォルトで「コストセンター未割当のアイテムのみ」を返す仕様であった。
そのため、コストセンターが割り当てられた大量のUsageItemが取得漏れになっていた。

### 新方針: Usage Reports API

非同期レポート生成APIを使えば、GitHub UIからダウンロードできるものと同等のCSVが取得できる。
実機検証で以下を確認済み:

- POST後30-60秒でレポートが `completed` になる
- `download_urls` にはAzure Blob Storage の SAS URL (1時間有効) が返る
- URLからダウンロードするとCSVが取得できる (全コストセンターのデータを含む)

### 実機検証で確認したAPIの挙動

```
POST /enterprises/enterprise-slug/settings/billing/reports
  Body: {"report_type":"summarized","start_date":"2026-04-01","end_date":"2026-04-30"}
  Response (202): {"id":"xxx","status":"processing",...}

GET /enterprises/enterprise-slug/settings/billing/reports/{id}
  (10秒後) Response: {"status":"processing"}
  (40秒後) Response: {"status":"completed","download_urls":["https://mbproduction.blob.core.windows.net/..."]}
```

- LIST API (`GET /enterprises/{enterprise}/settings/billing/reports`) では `download_urls` が含まれない
- 個別取得 (`GET /enterprises/{enterprise}/settings/billing/reports/{report_id}`) で `download_urls` が取得可能

## 実装プラン

### Step 1: 不要ファイルの削除

以下を削除する:

| ファイル | 理由 |
|---|---|
| `excel_export.go` | Excel出力は不要 |
| `organization_report.go` | 独自集計ロジックは不要 (APIが集計済みCSVを返す) |
| `organization_report_test.go` | 上記のテスト |
| `csv_export.go` | 独自CSV生成は不要 (APIがCSVを返す) |
| `csv_export_test.go` | 上記のテスト |
| `usage_item.go` | UsageItem構造体は不要 |

### Step 2: `billing_cycle.go` の修正

既存の `BillingCycle` は日付範囲を算出するのに使えるため維持。
ただし以下を変更:

- `GetRequiredAPIDateRange()` → 削除 (旧API用)
- `IsInDateRange()` → 削除 (UsageItem依存)
- `APIDate` 型 → 削除 (旧API用)
- `ConvertDateToApiDate()` → 削除 (旧API用)
- 新規追加: `GetStartDateString() string` — `"2026-04-01"` 形式を返す
- 新規追加: `GetEndDateString() string` — `"2026-04-30"` 形式を返す

```go
func (bc *BillingCycle) GetStartDateString() string {
    return bc.dateRange.Start.Format(OUTPUT_FORMAT)
}

func (bc *BillingCycle) GetEndDateString() string {
    return bc.dateRange.End.Format(OUTPUT_FORMAT)
}
```

テスト (`billing_cycle_test.go`) は `GetRequiredDateRange` のテストを維持し、
新メソッドのテストを追加。

### Step 3: `octokit.go` の全面書き換え

新しいAPIクライアントに書き換える。

```go
package main

import (
    "fmt"
    "io"
    "net/http"

    "github.com/cli/go-gh/v2/pkg/api"
)

// レポート生成要求と状態取得のレスポンス
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

// レポート生成要求のリクエストボディ
type CreateReportRequest struct {
    ReportType string `json:"report_type"`
    StartDate  string `json:"start_date"`
    EndDate    string `json:"end_date"`
}

// APIクライアント
type Octokit struct {
    client *api.RESTClient
}

// レポート生成を要求する
func (o *Octokit) CreateReport(enterprise string, req CreateReportRequest) (*ReportExport, error) {
    url := fmt.Sprintf("enterprises/%s/settings/billing/reports", enterprise)
    var resp ReportExport
    err := o.client.Post(url, req, &resp)
    if err != nil {
        return nil, err
    }
    return &resp, nil
}

// レポートの状態を取得する
func (o *Octokit) GetReport(enterprise, reportID string) (*ReportExport, error) {
    url := fmt.Sprintf("enterprises/%s/settings/billing/reports/%s", enterprise, reportID)
    var resp ReportExport
    err := o.client.Get(url, &resp)
    if err != nil {
        return nil, err
    }
    return &resp, nil
}

// URLからCSVをダウンロードする (SAS URL なのでgh clientではなくnet/httpを使う)
func DownloadCSV(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("download failed: status %d", resp.StatusCode)
    }

    return io.ReadAll(resp.Body)
}
```

### Step 4: `main.go` の書き換え

CLIオプションの変更とメインフローの書き換え。

変更点:

- `--csv` フラグ削除
- `--report-type` フラグ追加 (デフォルト: `both`)
- `--timeout` フラグ追加 (デフォルト: 300)
- メインフロー: レポート作成 → ポーリング → ダウンロード → 保存

```go
// メインフローの疑似コード
func run(enterprise, reportType, reportPath string, billingCycle *BillingCycle, timeout int) error {
    // 1. レポートタイプの決定
    reportTypes := determineReportTypes(reportType) // ["detailed"], ["summarized"], or ["detailed","summarized"]

    for _, rt := range reportTypes {
        // 2. レポート生成要求
        report, err := octokit.CreateReport(enterprise, CreateReportRequest{
            ReportType: rt,
            StartDate:  billingCycle.GetStartDateString(),
            EndDate:    billingCycle.GetEndDateString(),
        })

        // 3. ポーリング (5秒間隔、timeout秒まで)
        report, err = waitForCompletion(octokit, enterprise, report.ID, timeout)

        // 4. ダウンロード & 保存
        for i, url := range report.DownloadURLs {
            data, err := DownloadCSV(url)
            filename := buildFilename(billingCycle, rt, i, len(report.DownloadURLs))
            os.WriteFile(filepath.Join(reportPath, filename), data, 0644)
        }
    }
}
```

### Step 5: `go.mod` の依存整理

`excelize` は不要になるので削除:

```bash
go mod tidy
```

### Step 6: テスト

#### 単体テスト

- `billing_cycle_test.go`: 既存テスト維持 + `GetStartDateString` / `GetEndDateString` テスト追加
- `octokit_test.go` (新規): `CreateReport`, `GetReport` のHTTPモックテスト
- `main_test.go` (新規): `determineReportTypes`, `buildFilename` 等の純粋関数テスト

#### 結合テスト

- CLIパッケージの関数から、モックHTTPサーバーを使ってE2Eフローを確認

## ファイル構成 (実装後)

```
main.go              — CLI定義・メインフロー
octokit.go           — API呼び出し (createReport, getReport)
download.go          — CSVダウンロード (純粋なHTTP GET)
billing_cycle.go     — 日付範囲算出 (既存維持 + 新メソッド)
billing_cycle_test.go — 日付範囲テスト
octokit_test.go      — APIクライアントテスト (新規)
feature.md           — 機能仕様
```

## 実装順序

1. Step 1: 不要ファイル削除
2. Step 2: `billing_cycle.go` 修正
3. Step 3: `octokit.go` 書き換え
4. Step 4: `main.go` 書き換え
5. Step 5: `go mod tidy`
6. Step 6: テスト追加
7. 動作確認: `go build && gh billing-report --enterprise enterprise-slug --year 2026 --month 4`
