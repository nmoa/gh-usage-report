package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// aggregateCommandDependencies は aggregate サブコマンドの副作用をまとめます。
type aggregateCommandDependencies struct {
	openFile  func(name string) (io.ReadCloser, error)
	mkdirAll  func(path string, perm os.FileMode) error
	writeFile func(name string, data []byte, perm os.FileMode) error
}

// aggregateCommandOptions は aggregate サブコマンドの実行オプションです。
type aggregateCommandOptions struct {
	inputPath string
	outputDir string
	grouping  aggregateGrouping
	sortBy    aggregateSortBy
}

// newDefaultAggregateDependencies は aggregate サブコマンドの本番依存を構築します。
func newDefaultAggregateDependencies() aggregateCommandDependencies {
	return aggregateCommandDependencies{
		openFile:  func(name string) (io.ReadCloser, error) { return os.Open(name) },
		mkdirAll:  os.MkdirAll,
		writeFile: os.WriteFile,
	}
}

// newAggregateCmd は Usage CSV を再集計するサブコマンドを構築します。
func newAggregateCmd(deps aggregateCommandDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "aggregate",
		Short:        "Aggregate a downloaded usage CSV by org, cost center, or user",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAggregateCommand(cmd, deps)
		},
	}

	cmd.Flags().String("input", "", "Input usage CSV file")
	cmd.Flags().String("group-by", "", "Grouping: org, cost_center, or user")
	cmd.Flags().String("sort-by", string(aggregateSortByNetAmount), "Row ordering: net_amount or name")
	cmd.Flags().String("output-dir", ".", "Parent directory for aggregated CSV files")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("group-by")

	return cmd
}

// runAggregateCommand は集計サブコマンドの実行フローを制御します。
func runAggregateCommand(cmd *cobra.Command, deps aggregateCommandDependencies) error {
	options, err := readAggregateCommandOptions(cmd)
	if err != nil {
		return err
	}

	logger := log.New(cmd.ErrOrStderr(), "", 0)
	logger.Printf("Reading usage CSV: %s", options.inputPath)

	inputFile, err := deps.openFile(options.inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input CSV: %w", err)
	}
	defer inputFile.Close()

	records, format, err := parseUsageCSV(inputFile)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("input CSV did not contain any data rows")
	}

	aggregatedRows, err := aggregateUsageRecordsWithSort(records, format, options.grouping, options.sortBy)
	if err != nil {
		return err
	}

	outputDir := filepath.Join(options.outputDir, buildAggregateOutputDirName(options.inputPath))
	if err := deps.mkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create aggregate output directory: %w", err)
	}

	products := make([]string, 0, len(aggregatedRows))
	for product := range aggregatedRows {
		products = append(products, product)
	}
	sort.Strings(products)

	for _, product := range products {
		content, err := formatAggregatedCSV(aggregatedRows[product], options.grouping)
		if err != nil {
			return err
		}

		outputPath := filepath.Join(outputDir, buildAggregateOutputFileName(product, options.grouping))
		if err := deps.writeFile(outputPath, content, outputFilePermission); err != nil {
			return fmt.Errorf("failed to write aggregate CSV: %w", err)
		}
		logger.Printf("Saved: %s", outputPath)
	}

	logger.Printf("Saved aggregated CSVs to %s", outputDir)
	return nil
}

// readAggregateCommandOptions はフラグから集計オプションを組み立てます。
func readAggregateCommandOptions(cmd *cobra.Command) (aggregateCommandOptions, error) {
	inputPath, err := cmd.Flags().GetString("input")
	if err != nil {
		return aggregateCommandOptions{}, err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return aggregateCommandOptions{}, err
	}
	groupByValue, err := cmd.Flags().GetString("group-by")
	if err != nil {
		return aggregateCommandOptions{}, err
	}
	grouping, err := parseAggregateGrouping(groupByValue)
	if err != nil {
		return aggregateCommandOptions{}, err
	}
	sortByValue, err := cmd.Flags().GetString("sort-by")
	if err != nil {
		return aggregateCommandOptions{}, err
	}
	sortBy, err := parseAggregateSortBy(sortByValue)
	if err != nil {
		return aggregateCommandOptions{}, err
	}

	return aggregateCommandOptions{
		inputPath: inputPath,
		outputDir: outputDir,
		grouping:  grouping,
		sortBy:    sortBy,
	}, nil
}
