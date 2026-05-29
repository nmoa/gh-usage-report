package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

const (
	defaultReportPath    = "./reports"
	defaultTimeout       = 300
	defaultPollInterval  = 5 * time.Second
	reportTypeDetailed   = "detailed"
	reportTypeSummarized = "summarized"
	reportTypeBoth       = "both"
	outputFilePermission = 0o644
)

// ReportClient は Usage Reports API を呼び出す抽象です。
type ReportClient interface {
	CreateReport(ctx context.Context, enterprise string, req CreateReportRequest) (*ReportExport, error)
	GetReport(ctx context.Context, enterprise string, reportID string) (*ReportExport, error)
}

// CSVDownloader は CSV ダウンロード処理を抽象化します。
type CSVDownloader interface {
	Download(ctx context.Context, url string) ([]byte, error)
}

// commandDependencies は副作用を持つ依存をまとめます。
type commandDependencies struct {
	newRESTClient func(authToken string, logOutput io.Writer) (*api.RESTClient, error)
	downloader    CSVDownloader
	pollInterval  time.Duration
	mkdirAll      func(path string, perm os.FileMode) error
	writeFile     func(name string, data []byte, perm os.FileMode) error
}

// commandOptions は CLI から受け取る実行オプションです。
type commandOptions struct {
	githubToken  string
	enterprise   string
	reportType   string
	reportPath   string
	timeout      time.Duration
	billingCycle *BillingCycle
}

// newDefaultDependencies は本番実行用の依存を構築します。
func newDefaultDependencies() commandDependencies {
	return commandDependencies{
		newRESTClient: func(authToken string, logOutput io.Writer) (*api.RESTClient, error) {
			return api.NewRESTClient(api.ClientOptions{
				AuthToken: authToken,
				Log:       logOutput,
			})
		},
		downloader:   NewHTTPDownloader(httpDefaultClient),
		pollInterval: defaultPollInterval,
		mkdirAll:     os.MkdirAll,
		writeFile:    os.WriteFile,
	}
}

// newRootCmd は CLI エントリーポイントを構築します。
func newRootCmd(deps commandDependencies) *cobra.Command {
	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())

	cmd := &cobra.Command{
		Use:          "gh billing-report",
		Short:        "Download billing report CSV files from the Usage Reports API",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(cmd, deps)
		},
	}

	cmd.Flags().String("github-token", "", "GitHub token. Uses GITHUB_TOKEN when omitted")
	cmd.Flags().String("enterprise", "", "Enterprise slug")
	cmd.Flags().Int("year", currentYear, "Target year")
	cmd.Flags().Int("month", currentMonth, "Target month")
	cmd.Flags().Int("billing-cycle", 1, "Billing cycle start day")
	cmd.Flags().String("report-path", defaultReportPath, "Output directory for CSV files")
	cmd.Flags().String("report-type", reportTypeBoth, "Report type: detailed, summarized, both")
	cmd.Flags().Int("timeout", defaultTimeout, "Polling timeout in seconds")
	_ = cmd.MarkFlagRequired("enterprise")

	return cmd
}

// runRootCommand は CLI オプションを解釈して実行フローを開始します。
func runRootCommand(cmd *cobra.Command, deps commandDependencies) error {
	options, err := readCommandOptions(cmd)
	if err != nil {
		return err
	}

	logger := log.New(cmd.ErrOrStderr(), "", 0)
	logger.Printf("Reporting range: %s", options.billingCycle.GetDateRangeAsString())

	restClient, err := deps.newRESTClient(options.githubToken, io.Discard)
	if err != nil {
		return fmt.Errorf("failed to initialize API client: %w", err)
	}

	return run(cmd.Context(), deps, logger, NewOctokit(restClient), options)
}

// readCommandOptions は CLI フラグから実行オプションを組み立てます。
func readCommandOptions(cmd *cobra.Command) (commandOptions, error) {
	githubToken, err := cmd.Flags().GetString("github-token")
	if err != nil {
		return commandOptions{}, err
	}
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
	}

	enterprise, err := cmd.Flags().GetString("enterprise")
	if err != nil {
		return commandOptions{}, err
	}
	reportType, err := cmd.Flags().GetString("report-type")
	if err != nil {
		return commandOptions{}, err
	}
	reportPath, err := cmd.Flags().GetString("report-path")
	if err != nil {
		return commandOptions{}, err
	}
	timeoutSeconds, err := cmd.Flags().GetInt("timeout")
	if err != nil {
		return commandOptions{}, err
	}
	if timeoutSeconds <= 0 {
		return commandOptions{}, fmt.Errorf("--timeout must be 1 or greater")
	}

	year, err := cmd.Flags().GetInt("year")
	if err != nil {
		return commandOptions{}, err
	}
	month, err := cmd.Flags().GetInt("month")
	if err != nil {
		return commandOptions{}, err
	}
	billingCycleDay, err := cmd.Flags().GetInt("billing-cycle")
	if err != nil {
		return commandOptions{}, err
	}

	return commandOptions{
		githubToken: githubToken,
		enterprise:  enterprise,
		reportType:  reportType,
		reportPath:  reportPath,
		timeout:     time.Duration(timeoutSeconds) * time.Second,
		billingCycle: NewBillingCycle(InputCycle{
			Year:         year,
			Month:        month,
			BillingCycle: billingCycleDay,
		}),
	}, nil
}

// run は Usage Reports API を使って CSV を取得し保存します。
func run(ctx context.Context, deps commandDependencies, logger *log.Logger, reportClient ReportClient, options commandOptions) error {
	reportTypes, err := determineReportTypes(options.reportType)
	if err != nil {
		return err
	}

	if err := deps.mkdirAll(options.reportPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, currentReportType := range reportTypes {
		if err := createAndSaveReport(ctx, deps, logger, reportClient, options, currentReportType); err != nil {
			return err
		}
	}

	logger.Printf("Saved reports to %s", options.reportPath)
	return nil
}

// createAndSaveReport は 1 種類のレポートを生成し、完了待ちと保存までを行います。
func createAndSaveReport(ctx context.Context, deps commandDependencies, logger *log.Logger, reportClient ReportClient, options commandOptions, reportType string) error {
	logger.Printf("Generating %s report", reportType)

	report, err := reportClient.CreateReport(ctx, options.enterprise, CreateReportRequest{
		ReportType: reportType,
		StartDate:  options.billingCycle.GetStartDateString(),
		EndDate:    options.billingCycle.GetEndDateString(),
	})
	if err != nil {
		return fmt.Errorf("failed to create %s report. Check Enterprise administration permissions: %w", reportType, err)
	}
	if report.ID == "" {
		return fmt.Errorf("%s report response did not include a report ID", reportType)
	}

	completedReport, err := waitForCompletion(ctx, reportClient, options.enterprise, report.ID, options.timeout, deps.pollInterval, logger)
	if err != nil {
		return fmt.Errorf("failed while waiting for %s report completion: %w", reportType, err)
	}
	if len(completedReport.DownloadURLs) == 0 {
		return fmt.Errorf("%s report completed but download_urls was empty", reportType)
	}

	for index, url := range completedReport.DownloadURLs {
		downloadingMessage := buildDownloadingReportMessage(reportType, index, len(completedReport.DownloadURLs))
		downloadSpinner, useSpinner := newProgressSpinner(logger.Writer(), downloadingMessage)
		if useSpinner {
			downloadSpinner.Start()
		} else {
			logger.Print(downloadingMessage)
		}

		csvData, err := deps.downloader.Download(ctx, url)
		if useSpinner {
			downloadSpinner.Stop()
		}
		if err != nil {
			return fmt.Errorf("failed to download %s report: %w", reportType, err)
		}
		logger.Print(buildDownloadedReportMessage(reportType, index, len(completedReport.DownloadURLs)))

		outputPath := filepath.Join(options.reportPath, buildFilename(options.enterprise, options.billingCycle, reportType, index, len(completedReport.DownloadURLs)))
		if err := deps.writeFile(outputPath, csvData, outputFilePermission); err != nil {
			return fmt.Errorf("failed to write CSV file: %w", err)
		}
		logger.Printf("Saved: %s", outputPath)
	}

	return nil
}

// waitForCompletion はレポートが completed になるまでポーリングします。
func waitForCompletion(ctx context.Context, reportClient ReportClient, enterprise string, reportID string, timeout time.Duration, pollInterval time.Duration, logger *log.Logger) (*ReportExport, error) {
	timeoutContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	progressSpinner, useSpinner := newProgressSpinner(logger.Writer(), buildReportWaitingMessage(0))
	progressStarted := useSpinner
	if progressStarted {
		progressSpinner.Start()
	}
	defer func() {
		if progressStarted {
			progressSpinner.Stop()
		}
	}()

	for {
		report, err := reportClient.GetReport(timeoutContext, enterprise, reportID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch report status: %w", err)
		}

		switch report.Status {
		case "completed":
			if progressStarted {
				progressSpinner.Stop()
				progressStarted = false
			}
			logger.Print(buildReportCompletedMessage(time.Since(start)))
			return report, nil
		case "failed":
			return nil, fmt.Errorf("report generation failed")
		}

		progressMessage := buildReportWaitingMessage(time.Since(start))
		if progressStarted {
			progressSpinner.Suffix = buildSpinnerSuffix(progressMessage)
		} else {
			logger.Print(progressMessage)
		}

		timer := time.NewTimer(pollInterval)
		select {
		case <-timeoutContext.Done():
			timer.Stop()
			return nil, fmt.Errorf("report did not complete within %s", timeout.Round(time.Second))
		case <-timer.C:
		}
	}
}

// newProgressSpinner は進捗表示用の spinner を構築し、利用可否を返します。
func newProgressSpinner(writer io.Writer, message string) (*spinner.Spinner, bool) {
	writerFile, ok := writer.(*os.File)
	if !ok || !isInteractiveWriter(writerFile) {
		return nil, false
	}

	progressSpinner := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriterFile(writerFile))
	progressSpinner.Suffix = buildSpinnerSuffix(message)
	return progressSpinner, true
}

// isInteractiveWriter は spinner を安全に表示できる端末かを判定します。
func isInteractiveWriter(writerFile *os.File) bool {
	writerInfo, err := writerFile.Stat()
	if err != nil {
		return false
	}

	return writerInfo.Mode()&os.ModeCharDevice != 0
}

// buildSpinnerSuffix は spinner に表示する接尾辞を組み立てます。
func buildSpinnerSuffix(message string) string {
	return " " + message
}

// buildReportWaitingMessage はレポート完了待ち中の表示文言を返します。
func buildReportWaitingMessage(elapsed time.Duration) string {
	return fmt.Sprintf("Waiting for report completion... (%s elapsed)", elapsed.Round(time.Second))
}

// buildReportCompletedMessage はレポート完了時の表示文言を返します。
func buildReportCompletedMessage(elapsed time.Duration) string {
	return fmt.Sprintf("Report completed after %s.", elapsed.Round(time.Second))
}

// buildDownloadingReportMessage はダウンロード中の表示文言を返します。
func buildDownloadingReportMessage(reportType string, index int, total int) string {
	return fmt.Sprintf("Downloading %s report file %d/%d...", reportType, index+1, total)
}

// buildDownloadedReportMessage はダウンロード完了時の表示文言を返します。
func buildDownloadedReportMessage(reportType string, index int, total int) string {
	return fmt.Sprintf("Downloaded %s report file %d/%d.", reportType, index+1, total)
}

// determineReportTypes は CLI 入力から実行対象のレポート種別を返します。
func determineReportTypes(reportType string) ([]string, error) {
	switch strings.ToLower(reportType) {
	case reportTypeDetailed:
		return []string{reportTypeDetailed}, nil
	case reportTypeSummarized:
		return []string{reportTypeSummarized}, nil
	case reportTypeBoth:
		return []string{reportTypeDetailed, reportTypeSummarized}, nil
	default:
		return nil, fmt.Errorf("--report-type must be one of: detailed, summarized, both")
	}
}

// buildFilename は保存先の CSV ファイル名を組み立てます。
func buildFilename(enterprise string, billingCycle *BillingCycle, reportType string, index int, total int) string {
	fileName := fmt.Sprintf("GitHub_Usage_%s_%s_%s", enterprise, billingCycle.GetDateRangeAsString(), reportType)
	if total > 1 && index > 0 {
		return fmt.Sprintf("%s_%d.csv", fileName, index+1)
	}
	return fileName + ".csv"
}

// main は CLI を実行します。
func main() {
	cobra.CheckErr(newRootCmd(newDefaultDependencies()).Execute())
}
