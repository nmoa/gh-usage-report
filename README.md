# GH Usage Report

`gh usage-report` downloads enterprise usage report CSV files from GitHub's [Usage Reports API](https://docs.github.com/en/enterprise-cloud@latest/rest/billing/usage-reports?apiVersion=2022-11-28) and can re-aggregate downloaded CSVs by organization, cost center, or user, ordered by net amount or name.

This repository is originally forked from [davelosert/gh-billing-report](https://github.com/davelosert/gh-billing-report).

## Features

- Download `detailed`, `summarized`, or `both` usage report CSVs for an enterprise billing cycle.
- Calculate the report date range from `--year`, `--month`, and `--billing-cycle`.
- Poll GitHub's asynchronous export API and download every returned CSV file.
- Re-aggregate an existing usage CSV into product-specific CSVs grouped by `org`, `cost_center`, or `user`, sorted by `net_amount` or `name`.

## Installation

### GitHub CLI extension

```bash
gh extension install nmoa/gh-usage-report
```

### Run from source

```bash
git clone https://github.com/nmoa/gh-usage-report.git
cd gh-usage-report
go test ./...
go run . --help
```

This project requires Go 1.24 or later.

### Dev Container

This repository includes a Dev Container with Go 1.24 and GitHub CLI preinstalled.

```bash
go test ./...
go run . --help
```

## Authentication

The authenticated user must be an enterprise admin or billing manager.

GitHub currently documents these Usage Reports API endpoints for:

- Fine-grained personal access tokens
- GitHub App user access tokens
- GitHub App installation access tokens

For a fine-grained personal access token, grant `Enterprise administration` enterprise permissions with `write` access.

You can pass the token explicitly or use the `GITHUB_TOKEN` environment variable.

```bash
export GITHUB_TOKEN=<your-token>
gh usage-report --enterprise my-enterprise
```

## Download usage reports

Generate usage report CSV files for a billing cycle:

```bash
gh usage-report --enterprise my-enterprise
```

Run from source:

```bash
go run . --enterprise my-enterprise
```

Example with explicit options:

```bash
gh usage-report \
  --enterprise my-enterprise \
  --year 2026 \
  --month 4 \
  --billing-cycle 1 \
  --report-type both \
  --report-path ./reports \
  --timeout 300
```

### Download command options

| Option | Description | Default |
| --- | --- | --- |
| `--github-token` | GitHub token. Uses `GITHUB_TOKEN` when omitted. | none |
| `--enterprise` | Enterprise slug. | required |
| `--year` | Target year. | current year |
| `--month` | Target month. | current month |
| `--billing-cycle` | First day of the billing cycle. | `1` |
| `--report-path` | Output directory for downloaded CSV files. | `./reports` |
| `--report-type` | Report type: `detailed`, `summarized`, or `both`. | `both` |
| `--timeout` | Polling timeout in seconds. | `300` |

### Billing cycle behavior

By default, the report covers a calendar month.

When `--billing-cycle` is set, the tool calculates the range as:

- Start: the billing cycle day in the requested month
- End: the day before the same billing cycle day in the following month

If the billing cycle day does not exist in the requested month, the first day of the next month is used.

| Input | Report period |
| --- | --- |
| `--year 2024 --month 1 --billing-cycle 1` | `2024-01-01` to `2024-01-31` |
| `--year 2024 --month 1 --billing-cycle 15` | `2024-01-15` to `2024-02-14` |
| `--year 2024 --month 2 --billing-cycle 30` | `2024-03-01` to `2024-03-29` |

All cutoff dates are interpreted in UTC.

### Download output

The download command writes one or more CSV files to `--report-path`.

Typical output files:

- `GitHub_Usage_<enterprise>_<start>_to_<end>_detailed.csv`
- `GitHub_Usage_<enterprise>_<start>_to_<end>_summarized.csv`

If GitHub returns multiple download URLs for the same report type, the second and later files receive a numeric suffix:

- `GitHub_Usage_<enterprise>_<start>_to_<end>_detailed_2.csv`

## Aggregate downloaded CSV files

Use the `aggregate` subcommand to create product-specific summary CSVs from an existing usage export.

```bash
gh usage-report aggregate --input reports/GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30_summarized.csv --group-by org
```

Sort by group name instead of net amount:

```bash
gh usage-report aggregate --input reports/GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30_summarized.csv --group-by org --sort-by name
```

Run from source:

```bash
go run . aggregate --input reports/GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30_detailed.csv --group-by user
```

### Aggregate command options

| Option | Description | Default |
| --- | --- | --- |
| `--input` | Input usage CSV file. | required |
| `--group-by` | Grouping key: `org`, `cost_center`, or `user`. | required |
| `--sort-by` | Row ordering: `net_amount` or `name`. | `net_amount` |
| `--output-dir` | Parent directory for aggregate CSV output. | `.` |

### Aggregate behavior

- `summarized` CSV files support `org` and `cost_center` grouping.
- `detailed` CSV files support `org`, `cost_center`, and `user` grouping.
- `--group-by user` fails for `summarized` CSV files because they do not contain a `username` column.
- Rows are sorted by `net_amount` descending by default.
- `--sort-by name` sorts rows by the aggregate key ascending.
- Rows with the same `net_amount` fall back to the aggregate key so output stays deterministic.
- Empty grouping values are emitted as `(unassigned)`.
- `ratio` is the share of each row's `net_amount` within the total `net_amount` of the same product.

For an input file named `GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30_summarized.csv`, the aggregate command writes output into:

```text
<output-dir>/GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30/
```

Each product becomes its own CSV file:

- `actions_by_org.csv`
- `copilot_by_org.csv`
- `ghas_by_org.csv`

Each aggregate CSV contains:

- Group key (`org`, `cost_center`, or `user`)
- `gross_amount`
- `discount_amount`
- `net_amount`
- `ratio`

Example:

```csv
org,gross_amount,discount_amount,net_amount,ratio
org-a,12,2,10,0.666667
org-b,6,1,5,0.333333
```

## Development

Run the test suite:

```bash
go test ./...
```

Try the commands locally:

```bash
go run . --enterprise my-enterprise --year 2026 --month 4
go run . aggregate --input reports/GitHub_Usage_my-enterprise_2026-04-01_to_2026-04-30_detailed.csv --group-by org
```

## Releasing

To release a new version of this extension:

1. Merge your pull requests into `main`. Tag your pull requests with appropriate labels (such as `feature`, `bug`, `documentation`, `dependencies`, `chore`, `refactor`) to enable the automatic release notes generator to categorize changes.
2. Push a new semantic version tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
3. The GitHub Actions release workflow will trigger automatically. It will:
   - Build and cross-compile binaries for all supported platforms.
   - Generate secure Artifact Attestations for the compiled binaries.
   - Create a GitHub Release with automatically categorized release notes based on the configuration in `.github/release.yml`.

## License

This project is licensed under the [MIT License](./LICENSE).
