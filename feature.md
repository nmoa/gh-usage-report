# gh-billing-report 機能仕様

## 概要

GitHub Enterprise の Usage Reports API を利用して、billing の usage report (CSV) を取得・保存する GitHub CLI Extension。

## 満たすべきフィーチャー

- Usage Reports API を利用して enterprise の billing report CSV を取得できること
- 対象期間を `--year` `--month` `--billing-cycle` から計算できること
- `detailed` `summarized` `both` の各レポート種別を扱えること
- 完了待ちのポーリングと CSV ダウンロード進捗を CLI で表示できること
- `.devcontainer/devcontainer.json` により Go 1.24 と GitHub CLI を含む開発環境をコンテナで起動できること
- Dev Container 作成時に `go mod download` を実行し、依存取得済みの状態で作業を開始できること

## 使用する API

- `POST /enterprises/{enterprise}/settings/billing/reports` — レポート生成要求
- `GET /enterprises/{enterprise}/settings/billing/reports/{report_id}` — レポート状態・ダウンロードURL取得

参考: <https://docs.github.com/en/enterprise-cloud@latest/rest/billing/usage-reports?apiVersion=2022-11-28>

## コマンド

```
gh billing-report --enterprise <slug> [options]
```

## CLIオプション

| オプション | 説明 | デフォルト |
|---|---|---|
| `--enterprise` | Enterprise slug (必須) | - |
| `--github-token` | GitHub トークン (環境変数 `GITHUB_TOKEN` でも可) | - |
| `--year` | 対象年 | 現在の年 |
| `--month` | 対象月 | 現在の月 |
| `--billing-cycle` | 請求サイクル開始日 | 1 |
| `--report-path` | CSV出力先ディレクトリ | `./reports` |
| `--report-type` | レポート種別 (`detailed`, `summarized`, `both`) | `both` |
| `--timeout` | ポーリングタイムアウト (秒) | 300 |

## 処理フロー

1. `--year`, `--month`, `--billing-cycle` から対象期間 (start_date, end_date) を算出する
2. `--report-type` に応じて、`POST /enterprises/{enterprise}/settings/billing/reports` でレポート生成を要求する
   - `both` の場合は `detailed` と `summarized` の2つを要求する
3. 5秒間隔で `GET /enterprises/{enterprise}/settings/billing/reports/{report_id}` をポーリングし、`status` が `completed` になるのを待つ
   - ポーリング中は `briandowns/spinner` の No.11 で進捗を表示する (例: `Waiting for report completion... (15s elapsed)`)
   - 端末上では spinner のメッセージを更新し続け、通常実行では HTTP デバッグログを表示しない
   - `--timeout` を超えてもcompletedにならない場合はエラー終了する
4. `completed` になったら `download_urls` から CSV をダウンロードする
   - ダウンロード中も `briandowns/spinner` の No.11 で進捗を表示する (例: `Downloading detailed report file 1/2...`)
   - spinner を使えない出力先では plain log へフォールバックする
   - `download_urls` は配列で複数ファイルが含まれる可能性がある。全てダウンロードする
5. ダウンロードした CSV を `--report-path` に保存する

## 出力ファイル名

```
GitHub_Usage_{enterprise-slug}_{start_date}_to_{end_date}_detailed.csv
GitHub_Usage_{enterprise-slug}_{start_date}_to_{end_date}_summarized.csv
```

例: `GitHub_Usage_enterprise-slug_2026-04-01_to_2026-04-30_detailed.csv`

`download_urls` に複数ファイルがある場合は、2つ目以降に連番を付与する:

```
GitHub_Usage_enterprise-slug_2026-04-01_to_2026-04-30_detailed_2.csv
```

## 必要な権限

トークンに以下の権限が必要:

- Fine-grained PAT: "Enterprise administration" enterprise permissions (write)

## エラーハンドリング

- API 認証エラー (401/403): トークンの権限不足を示すメッセージを表示して終了
- レポート生成エラー (400): リクエストパラメータのエラーメッセージを表示して終了
- タイムアウト: `--timeout` 秒以内にレポートが完了しなかった旨のメッセージを表示して終了
- ダウンロードエラー: リトライせずエラーメッセージを表示して終了

## 削除対象 (不要になる機能)

- Excel出力機能 (`excel_export.go`)
- Organization集計ロジック (`organization_report.go`)
- 独自CSV生成ロジック (`csv_export.go`)
- UsageItem構造体・フィルタ (`usage_item.go`)
- 旧 billing usage API 呼び出し (`octokit.go` の既存コード)
