// Package main provides the gh billing-report CLI and CSV aggregation helpers.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	aggregateGroupingOrg        aggregateGrouping = "org"
	aggregateGroupingCostCenter aggregateGrouping = "cost_center"
	aggregateGroupingUser       aggregateGrouping = "user"
	amountPrecision                               = 9
	ratioPrecision                                = 6
	unassignedGroupValue                          = "(unassigned)"
)

// usageCSVFormat は入力 CSV の種類を表します。
type usageCSVFormat string

const (
	usageCSVFormatDetailed   usageCSVFormat = "detailed"
	usageCSVFormatSummarized usageCSVFormat = "summarized"
)

// aggregateGrouping は集計単位を表します。
type aggregateGrouping string

// aggregateSortBy は集計結果の並び順を表します。
type aggregateSortBy string

const (
	aggregateSortByNetAmount aggregateSortBy = "net_amount"
	aggregateSortByName      aggregateSortBy = "name"
)

// usageRecord は集計に必要な CSV 行データです。
type usageRecord struct {
	Product        string
	GrossAmount    float64
	DiscountAmount float64
	NetAmount      float64
	Username       string
	Organization   string
	CostCenterName string
}

// aggregateRow は 1 つの集計キーに対する集計結果です。
type aggregateRow struct {
	Key            string
	GrossAmount    float64
	DiscountAmount float64
	NetAmount      float64
	Ratio          float64
}

// parseUsageCSV は detailed または summarized の Usage CSV を読み取ります。
func parseUsageCSV(reader io.Reader) ([]usageRecord, usageCSVFormat, error) {
	csvReader := csv.NewReader(stripUTF8BOM(reader))
	csvReader.FieldsPerRecord = -1

	headers, err := csvReader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, "", fmt.Errorf("csv is empty")
		}
		return nil, "", fmt.Errorf("failed to read csv header: %w", err)
	}

	headerIndex := buildCSVHeaderIndex(headers)
	format, err := detectUsageCSVFormat(headerIndex)
	if err != nil {
		return nil, "", err
	}

	if err := validateUsageCSVHeaders(headerIndex); err != nil {
		return nil, "", err
	}

	records := []usageRecord{}
	rowNumber := 1
	for {
		row, err := csvReader.Read()
		if err != nil {
			if err == io.EOF {
				return records, format, nil
			}
			return nil, "", fmt.Errorf("failed to read csv row %d: %w", rowNumber+1, err)
		}
		rowNumber++

		if isBlankCSVRow(row) {
			continue
		}

		record, err := parseUsageCSVRow(row, rowNumber, headerIndex)
		if err != nil {
			return nil, "", err
		}
		records = append(records, record)
	}
}

// stripUTF8BOM は CSV 先頭の UTF-8 BOM を取り除きます。
func stripUTF8BOM(reader io.Reader) io.Reader {
	bufferedReader := bufio.NewReader(reader)
	leadingBytes, err := bufferedReader.Peek(3)
	if err == nil && bytes.Equal(leadingBytes, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = bufferedReader.Discard(3)
	}
	return bufferedReader
}

// buildCSVHeaderIndex はヘッダー名から列位置を引ける辞書を構築します。
func buildCSVHeaderIndex(headers []string) map[string]int {
	headerIndex := make(map[string]int, len(headers))
	for index, header := range headers {
		headerIndex[strings.TrimSpace(header)] = index
	}
	return headerIndex
}

// detectUsageCSVFormat はヘッダーから CSV 種類を判定します。
func detectUsageCSVFormat(headerIndex map[string]int) (usageCSVFormat, error) {
	if _, ok := headerIndex["username"]; ok {
		return usageCSVFormatDetailed, nil
	}

	if _, ok := headerIndex["organization"]; ok {
		return usageCSVFormatSummarized, nil
	}

	return "", fmt.Errorf("csv must include organization column")
}

// validateUsageCSVHeaders は集計に必要なヘッダーの存在を検証します。
func validateUsageCSVHeaders(headerIndex map[string]int) error {
	requiredHeaders := []string{"product", "gross_amount", "discount_amount", "net_amount", "organization", "cost_center_name"}
	for _, header := range requiredHeaders {
		if _, ok := headerIndex[header]; !ok {
			return fmt.Errorf("csv must include %s column", header)
		}
	}

	return nil
}

// isBlankCSVRow は空行を判定します。
func isBlankCSVRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

// parseUsageCSVRow は 1 行分の CSV レコードを集計用構造体に変換します。
func parseUsageCSVRow(row []string, rowNumber int, headerIndex map[string]int) (usageRecord, error) {
	product, err := readCSVField(row, rowNumber, headerIndex, "product")
	if err != nil {
		return usageRecord{}, err
	}
	grossAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "gross_amount")
	if err != nil {
		return usageRecord{}, err
	}
	discountAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "discount_amount")
	if err != nil {
		return usageRecord{}, err
	}
	netAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "net_amount")
	if err != nil {
		return usageRecord{}, err
	}
	username := readCSVOptionalField(row, headerIndex, "username")
	organization, err := readCSVField(row, rowNumber, headerIndex, "organization")
	if err != nil {
		return usageRecord{}, err
	}
	costCenterName, err := readCSVField(row, rowNumber, headerIndex, "cost_center_name")
	if err != nil {
		return usageRecord{}, err
	}

	return usageRecord{
		Product:        strings.TrimSpace(product),
		GrossAmount:    grossAmount,
		DiscountAmount: discountAmount,
		NetAmount:      netAmount,
		Username:       strings.TrimSpace(username),
		Organization:   strings.TrimSpace(organization),
		CostCenterName: strings.TrimSpace(costCenterName),
	}, nil
}

// readCSVField は必須列の値を返します。
func readCSVField(row []string, rowNumber int, headerIndex map[string]int, header string) (string, error) {
	index, ok := headerIndex[header]
	if !ok {
		return "", fmt.Errorf("csv must include %s column", header)
	}
	if index >= len(row) {
		return "", fmt.Errorf("row %d is missing %s column", rowNumber, header)
	}
	return row[index], nil
}

// readCSVOptionalField は任意列の値を返します。
func readCSVOptionalField(row []string, headerIndex map[string]int, header string) string {
	index, ok := headerIndex[header]
	if !ok || index >= len(row) {
		return ""
	}
	return row[index]
}

// parseCSVFloatField は数値列を float64 に変換します。
func parseCSVFloatField(row []string, rowNumber int, headerIndex map[string]int, header string) (float64, error) {
	value, err := readCSVField(row, rowNumber, headerIndex, header)
	if err != nil {
		return 0, err
	}

	parsedValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("row %d has invalid %s value %q: %w", rowNumber, header, value, err)
	}

	return parsedValue, nil
}

// parseAggregateGrouping は CLI 入力の集計単位を正規化します。
func parseAggregateGrouping(value string) (aggregateGrouping, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case string(aggregateGroupingOrg), "organization":
		return aggregateGroupingOrg, nil
	case string(aggregateGroupingCostCenter), "costcenter":
		return aggregateGroupingCostCenter, nil
	case string(aggregateGroupingUser), "username":
		return aggregateGroupingUser, nil
	default:
		return "", fmt.Errorf("--group-by must be one of: org, cost_center, user")
	}
}

// parseAggregateSortBy は CLI 入力の並び順を正規化します。
func parseAggregateSortBy(value string) (aggregateSortBy, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case string(aggregateSortByNetAmount), "netamount":
		return aggregateSortByNetAmount, nil
	case string(aggregateSortByName):
		return aggregateSortByName, nil
	default:
		return "", fmt.Errorf("--sort-by must be one of: net_amount, name")
	}
}

// aggregateUsageRecords は product ごとに指定単位の集計結果を既定の並び順で返します。
func aggregateUsageRecords(records []usageRecord, format usageCSVFormat, grouping aggregateGrouping) (map[string][]aggregateRow, error) {
	return aggregateUsageRecordsWithSort(records, format, grouping, aggregateSortByNetAmount)
}

// aggregateUsageRecordsWithSort は product ごとに指定単位と並び順の集計結果を返します。
func aggregateUsageRecordsWithSort(records []usageRecord, format usageCSVFormat, grouping aggregateGrouping, sortBy aggregateSortBy) (map[string][]aggregateRow, error) {
	if grouping == aggregateGroupingUser && format != usageCSVFormatDetailed {
		return nil, fmt.Errorf("--group-by user requires a detailed CSV with username column")
	}
	if sortBy != aggregateSortByNetAmount && sortBy != aggregateSortByName {
		return nil, fmt.Errorf("--sort-by must be one of: net_amount, name")
	}

	type aggregateTotals struct {
		GrossAmount    float64
		DiscountAmount float64
		NetAmount      float64
	}

	productGroupTotals := map[string]map[string]aggregateTotals{}
	productNetTotals := map[string]float64{}

	for _, record := range records {
		groupKey := groupValueForRecord(record, grouping)
		if _, ok := productGroupTotals[record.Product]; !ok {
			productGroupTotals[record.Product] = map[string]aggregateTotals{}
		}

		totals := productGroupTotals[record.Product][groupKey]
		totals.GrossAmount += record.GrossAmount
		totals.DiscountAmount += record.DiscountAmount
		totals.NetAmount += record.NetAmount
		productGroupTotals[record.Product][groupKey] = totals
		productNetTotals[record.Product] += record.NetAmount
	}

	result := make(map[string][]aggregateRow, len(productGroupTotals))
	for product, groupTotals := range productGroupTotals {
		rows := make([]aggregateRow, 0, len(groupTotals))
		totalNetAmount := productNetTotals[product]
		for key, totals := range groupTotals {
			ratio := 0.0
			if totalNetAmount != 0 {
				ratio = totals.NetAmount / totalNetAmount
			}

			rows = append(rows, aggregateRow{
				Key:            key,
				GrossAmount:    totals.GrossAmount,
				DiscountAmount: totals.DiscountAmount,
				NetAmount:      totals.NetAmount,
				Ratio:          ratio,
			})
		}

		sortAggregateRows(rows, sortBy)

		result[product] = rows
	}

	return result, nil
}

// sortAggregateRows は指定の並び順で集計結果を並べ替えます。
func sortAggregateRows(rows []aggregateRow, sortBy aggregateSortBy) {
	sort.Slice(rows, func(left int, right int) bool {
		return aggregateRowsLess(rows[left], rows[right], sortBy)
	})
}

// aggregateRowsLess は 2 行の並び順を比較します。
func aggregateRowsLess(left aggregateRow, right aggregateRow, sortBy aggregateSortBy) bool {
	if sortBy == aggregateSortByName {
		return aggregateRowsLessByName(left, right)
	}
	return aggregateRowsLessByNetAmount(left, right)
}

// aggregateRowsLessByNetAmount は net_amount 降順の並び順を比較します。
func aggregateRowsLessByNetAmount(left aggregateRow, right aggregateRow) bool {
	if left.NetAmount != right.NetAmount {
		return left.NetAmount > right.NetAmount
	}
	return left.Key < right.Key
}

// aggregateRowsLessByName は集計キー昇順の並び順を比較します。
func aggregateRowsLessByName(left aggregateRow, right aggregateRow) bool {
	if left.Key != right.Key {
		return left.Key < right.Key
	}
	return left.NetAmount > right.NetAmount
}

// groupValueForRecord は対象レコードから集計キーを取り出します。
func groupValueForRecord(record usageRecord, grouping aggregateGrouping) string {
	switch grouping {
	case aggregateGroupingOrg:
		return normalizeAggregateKey(record.Organization)
	case aggregateGroupingCostCenter:
		return normalizeAggregateKey(record.CostCenterName)
	case aggregateGroupingUser:
		return normalizeAggregateKey(record.Username)
	default:
		return unassignedGroupValue
	}
}

// normalizeAggregateKey は空のキーを集計用の表示値に置き換えます。
func normalizeAggregateKey(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return unassignedGroupValue
	}
	return trimmedValue
}

// formatAggregatedCSV は集計結果を CSV 形式のバイト列に変換します。
func formatAggregatedCSV(rows []aggregateRow, grouping aggregateGrouping) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)

	if err := writer.Write([]string{aggregateGroupingHeader(grouping), "gross_amount", "discount_amount", "net_amount", "ratio"}); err != nil {
		return nil, fmt.Errorf("failed to write csv header: %w", err)
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Key,
			formatAggregateNumber(row.GrossAmount, amountPrecision),
			formatAggregateNumber(row.DiscountAmount, amountPrecision),
			formatAggregateNumber(row.NetAmount, amountPrecision),
			formatAggregateNumber(row.Ratio, ratioPrecision),
		}); err != nil {
			return nil, fmt.Errorf("failed to write csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush csv writer: %w", err)
	}

	return buffer.Bytes(), nil
}

// aggregateGroupingHeader は出力 CSV の先頭列名を返します。
func aggregateGroupingHeader(grouping aggregateGrouping) string {
	switch grouping {
	case aggregateGroupingOrg:
		return "org"
	case aggregateGroupingCostCenter:
		return "cost_center"
	case aggregateGroupingUser:
		return "user"
	default:
		return "group"
	}
}

// formatAggregateNumber は見やすい桁数に丸めて文字列化します。
func formatAggregateNumber(value float64, precision int) string {
	factor := math.Pow10(precision)
	roundedValue := math.Round(value*factor) / factor
	formattedValue := strconv.FormatFloat(roundedValue, 'f', precision, 64)
	formattedValue = strings.TrimRight(formattedValue, "0")
	formattedValue = strings.TrimRight(formattedValue, ".")
	if formattedValue == "" || formattedValue == "-0" {
		return "0"
	}
	return formattedValue
}

// buildAggregateOutputDirName は入力ファイル名から出力ディレクトリ名を組み立てます。
func buildAggregateOutputDirName(inputPath string) string {
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	baseName = strings.TrimSuffix(baseName, "_detailed")
	baseName = strings.TrimSuffix(baseName, "_summarized")
	return baseName
}

// buildAggregateOutputFileName は product と集計単位から出力ファイル名を組み立てます。
func buildAggregateOutputFileName(product string, grouping aggregateGrouping) string {
	return fmt.Sprintf("%s_by_%s.csv", strings.TrimSpace(product), grouping)
}
