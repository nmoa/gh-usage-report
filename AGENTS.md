# AGENTS.md

## Project Overview

`gh-billing-report` is a GitHub CLI extension that generates Excel billing reports for GitHub Enterprise. It fetches usage data from the GitHub Enterprise Billing API and exports it as an `.xlsx` file.

## Build & Test

```bash
# Build
go build ./...

# Run all tests
go test ./...
```

There is no linter configured in this repository.

## Architecture

This is a single-package Go application (`package main`) with the following structure:

| File | Responsibility |
|------|---------------|
| `main.go` | CLI entry point using cobra/viper for flags and configuration |
| `octokit.go` | GitHub REST API client wrapper (uses `github.com/cli/go-gh/v2`) |
| `billing_cycle.go` | Date range calculation for billing cycles |
| `usage_item.go` | Data model for usage items and filtering |
| `organization_report.go` | Aggregation logic (group by org, compute totals) |
| `excel_export.go` | Excel file generation using excelize |

## Key Dependencies

- `github.com/cli/go-gh/v2` — GitHub CLI Go library for API access
- `github.com/spf13/cobra` / `github.com/spf13/viper` — CLI framework
- `github.com/xuri/excelize/v2` — Excel file generation

## Conventions

- All source files are in the root directory (flat structure, single package).
- Tests are in `*_test.go` files alongside the source.
- The API client (`Octokit`) uses the `gh` CLI's built-in authentication via `go-gh`.
- No external HTTP calls are made outside of the GitHub Enterprise Billing API.

## CI/CD

- **Test**: Runs `go test .` on push/PR to `main` (`.github/workflows/test.yml`)
- **Release**: Uses `cli/gh-extension-precompile@v2` on tag push (`.github/workflows/release.yml`)
