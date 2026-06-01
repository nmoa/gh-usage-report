// Package aggregate は CSV パース・集計・フォーマット機能を提供します。
package aggregate

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
	// GroupingOrg は org 単位の集計を表します。
	GroupingOrg Grouping = "org"
	// GroupingCostCenter は cost_center 単位の集計を表します。
	GroupingCostCenter Grouping = "cost_center"
	// GroupingUser は user 単位の集計を表します。
	GroupingUser Grouping = "user"

	// SortByNetAmount は net_amount 降順の並び順です。
	SortByNetAmount SortBy = "net_amount"
	// SortByName は集計キー昇順の並び順です。
	SortByName SortBy = "name"

	// CSVFormatDetailed は detailed CSV を表します。
	CSVFormatDetailed CSVFormat = "detailed"
	// CSVFormatSummarized は summarized CSV を表します。
	CSVFormatSummarized CSVFormat = "summarized"

	amountPrecision      = 9
	ratioPrecision       = 6
	unassignedGroupValue = "(unassigned)"
)

// CSVFormat は入力 CSV の種類を表します。
type CSVFormat string

// Grouping は集計単位を表します。
type Grouping string

// SortBy は集計結果の並び順を表します。
type SortBy string

// UsageRecord は集計に必要な CSV 行データです。
type UsageRecord struct {
	Product        string
	GrossAmount    float64
	DiscountAmount float64
	NetAmount      float64
	Username       string
	Organization   string
	CostCenterName string
}

// Row は 1 つの集計キーに対する集計結果です。
type Row struct {
	Key            string
	GrossAmount    float64
	DiscountAmount float64
	NetAmount      float64
	Ratio          float64
}

// ParseUsageCSV は detailed または summarized の Usage CSV を読み取ります。
func ParseUsageCSV(reader io.Reader) ([]UsageRecord, CSVFormat, error) {
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
	format, err := detectCSVFormat(headerIndex)
	if err != nil {
		return nil, "", err
	}

	if err := validateCSVHeaders(headerIndex); err != nil {
		return nil, "", err
	}

	records := []UsageRecord{}
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

		record, err := parseCSVRow(row, rowNumber, headerIndex)
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

// detectCSVFormat はヘッダーから CSV 種類を判定します。
func detectCSVFormat(headerIndex map[string]int) (CSVFormat, error) {
	if _, ok := headerIndex["username"]; ok {
		return CSVFormatDetailed, nil
	}
	if _, ok := headerIndex["organization"]; ok {
		return CSVFormatSummarized, nil
	}
	return "", fmt.Errorf("csv must include organization column")
}

// validateCSVHeaders は集計に必要なヘッダーの存在を検証します。
func validateCSVHeaders(headerIndex map[string]int) error {
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

// parseCSVRow は 1 行分の CSV レコードを集計用構造体に変換します。
func parseCSVRow(row []string, rowNumber int, headerIndex map[string]int) (UsageRecord, error) {
	product, err := readCSVField(row, rowNumber, headerIndex, "product")
	if err != nil {
		return UsageRecord{}, err
	}
	grossAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "gross_amount")
	if err != nil {
		return UsageRecord{}, err
	}
	discountAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "discount_amount")
	if err != nil {
		return UsageRecord{}, err
	}
	netAmount, err := parseCSVFloatField(row, rowNumber, headerIndex, "net_amount")
	if err != nil {
		return UsageRecord{}, err
	}
	username := readCSVOptionalField(row, headerIndex, "username")
	organization, err := readCSVField(row, rowNumber, headerIndex, "organization")
	if err != nil {
		return UsageRecord{}, err
	}
	costCenterName, err := readCSVField(row, rowNumber, headerIndex, "cost_center_name")
	if err != nil {
		return UsageRecord{}, err
	}

	return UsageRecord{
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

// ParseGrouping は CLI 入力の集計単位を正規化します。
func ParseGrouping(value string) (Grouping, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case string(GroupingOrg), "organization":
		return GroupingOrg, nil
	case string(GroupingCostCenter), "costcenter":
		return GroupingCostCenter, nil
	case string(GroupingUser), "username":
		return GroupingUser, nil
	default:
		return "", fmt.Errorf("--group-by must be one of: org, cost_center, user")
	}
}

// ParseSortBy は CLI 入力の並び順を正規化します。
func ParseSortBy(value string) (SortBy, error) {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case string(SortByNetAmount), "netamount":
		return SortByNetAmount, nil
	case string(SortByName):
		return SortByName, nil
	default:
		return "", fmt.Errorf("--sort-by must be one of: net_amount, name")
	}
}

// UsageRecords は product ごとに指定単位の集計結果を既定の並び順で返します。
func UsageRecords(records []UsageRecord, format CSVFormat, grouping Grouping) (map[string][]Row, error) {
	return UsageRecordsWithSort(records, format, grouping, SortByNetAmount)
}

// UsageRecordsWithSort は product ごとに指定単位と並び順の集計結果を返します。
func UsageRecordsWithSort(records []UsageRecord, format CSVFormat, grouping Grouping, sortBy SortBy) (map[string][]Row, error) {
	if grouping == GroupingUser && format != CSVFormatDetailed {
		return nil, fmt.Errorf("--group-by user requires a detailed CSV with username column")
	}
	if sortBy != SortByNetAmount && sortBy != SortByName {
		return nil, fmt.Errorf("--sort-by must be one of: net_amount, name")
	}

	type totals struct {
		GrossAmount    float64
		DiscountAmount float64
		NetAmount      float64
	}

	productGroupTotals := map[string]map[string]totals{}
	productNetTotals := map[string]float64{}

	for _, record := range records {
		groupKey := groupValueForRecord(record, grouping)
		if _, ok := productGroupTotals[record.Product]; !ok {
			productGroupTotals[record.Product] = map[string]totals{}
		}
		t := productGroupTotals[record.Product][groupKey]
		t.GrossAmount += record.GrossAmount
		t.DiscountAmount += record.DiscountAmount
		t.NetAmount += record.NetAmount
		productGroupTotals[record.Product][groupKey] = t
		productNetTotals[record.Product] += record.NetAmount
	}

	result := make(map[string][]Row, len(productGroupTotals))
	for product, groupTotals := range productGroupTotals {
		rows := make([]Row, 0, len(groupTotals))
		totalNetAmount := productNetTotals[product]
		for key, t := range groupTotals {
			ratio := 0.0
			if totalNetAmount != 0 {
				ratio = t.NetAmount / totalNetAmount
			}
			rows = append(rows, Row{
				Key:            key,
				GrossAmount:    t.GrossAmount,
				DiscountAmount: t.DiscountAmount,
				NetAmount:      t.NetAmount,
				Ratio:          ratio,
			})
		}
		sortRows(rows, sortBy)
		result[product] = rows
	}

	return result, nil
}

// sortRows は指定の並び順で集計結果を並べ替えます。
func sortRows(rows []Row, sortBy SortBy) {
	sort.Slice(rows, func(left int, right int) bool {
		return rowsLess(rows[left], rows[right], sortBy)
	})
}

// rowsLess は 2 行の並び順を比較します。
func rowsLess(left Row, right Row, sortBy SortBy) bool {
	if sortBy == SortByName {
		return rowsLessByName(left, right)
	}
	return rowsLessByNetAmount(left, right)
}

// rowsLessByNetAmount は net_amount 降順の並び順を比較します。
func rowsLessByNetAmount(left Row, right Row) bool {
	if left.NetAmount != right.NetAmount {
		return left.NetAmount > right.NetAmount
	}
	return left.Key < right.Key
}

// rowsLessByName は集計キー昇順の並び順を比較します。
func rowsLessByName(left Row, right Row) bool {
	if left.Key != right.Key {
		return left.Key < right.Key
	}
	return left.NetAmount > right.NetAmount
}

// groupValueForRecord は対象レコードから集計キーを取り出します。
func groupValueForRecord(record UsageRecord, grouping Grouping) string {
	switch grouping {
	case GroupingOrg:
		return normalizeKey(record.Organization)
	case GroupingCostCenter:
		return normalizeKey(record.CostCenterName)
	case GroupingUser:
		return normalizeKey(record.Username)
	default:
		return unassignedGroupValue
	}
}

// normalizeKey は空のキーを集計用の表示値に置き換えます。
func normalizeKey(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return unassignedGroupValue
	}
	return trimmedValue
}

// FormatCSV は集計結果を CSV 形式のバイト列に変換します。
func FormatCSV(rows []Row, grouping Grouping) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)

	if err := writer.Write([]string{groupingHeader(grouping), "gross_amount", "discount_amount", "net_amount", "ratio"}); err != nil {
		return nil, fmt.Errorf("failed to write csv header: %w", err)
	}

	for _, row := range rows {
		if err := writer.Write([]string{
			row.Key,
			formatNumber(row.GrossAmount, amountPrecision),
			formatNumber(row.DiscountAmount, amountPrecision),
			formatNumber(row.NetAmount, amountPrecision),
			formatNumber(row.Ratio, ratioPrecision),
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

// groupingHeader は出力 CSV の先頭列名を返します。
func groupingHeader(grouping Grouping) string {
	switch grouping {
	case GroupingOrg:
		return "org"
	case GroupingCostCenter:
		return "cost_center"
	case GroupingUser:
		return "user"
	default:
		return "group"
	}
}

// formatNumber は見やすい桁数に丸めて文字列化します。
func formatNumber(value float64, precision int) string {
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

// BuildOutputDirName は入力ファイル名から出力ディレクトリ名を組み立てます。
func BuildOutputDirName(inputPath string) string {
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	baseName = strings.TrimSuffix(baseName, "_detailed")
	baseName = strings.TrimSuffix(baseName, "_summarized")
	return baseName
}

// BuildOutputFileName は product と集計単位から出力ファイル名を組み立てます。
func BuildOutputFileName(product string, grouping Grouping) string {
	return fmt.Sprintf("%s_by_%s.csv", strings.TrimSpace(product), grouping)
}
