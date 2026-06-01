// Package main は gh usage-report CLI を提供します。
package main

import (
	"fmt"
	"time"
)

// InputCycle は CLI から受け取る請求サイクル条件です。
type InputCycle struct {
	Year         int
	Month        int
	BillingCycle int
}

// DateRange は請求期間の開始日と終了日を表します。
type DateRange struct {
	Start time.Time
	End   time.Time
}

const OUTPUT_FORMAT = "2006-01-02"

// BillingCycle は解決済みの請求期間を保持します。
type BillingCycle struct {
	dateRange DateRange
}

// NewBillingCycle は CLI 入力から請求期間を構築します。
func NewBillingCycle(inputCycle InputCycle) *BillingCycle {
	dateRange := GetRequiredDateRange(inputCycle)
	return &BillingCycle{dateRange: dateRange}
}

// GetDateRange は請求期間の開始時刻と終了時刻を返します。
func (bc *BillingCycle) GetDateRange() (time.Time, time.Time) {
	return bc.dateRange.Start, bc.dateRange.End
}

// GetStartDateString は API リクエスト向けの開始日文字列を返します。
func (bc *BillingCycle) GetStartDateString() string {
	return bc.dateRange.Start.Format(OUTPUT_FORMAT)
}

// GetEndDateString は API リクエスト向けの終了日文字列を返します。
func (bc *BillingCycle) GetEndDateString() string {
	return bc.dateRange.End.Format(OUTPUT_FORMAT)
}

// GetDateRangeAsString は出力ファイル名向けの期間文字列を返します。
func (bc *BillingCycle) GetDateRangeAsString() string {
	return fmt.Sprintf("%s_to_%s", bc.GetStartDateString(), bc.GetEndDateString())
}

// GetRequiredDateRange は入力条件から請求期間を計算します。
func GetRequiredDateRange(inputCycle InputCycle) DateRange {
	if inputCycle.BillingCycle == 1 {
		startOfMonth := startOfMonth(inputCycle.Year, inputCycle.Month)
		endOfMonth := endOfMonth(inputCycle.Year, inputCycle.Month)
		return DateRange{Start: startOfMonth, End: endOfMonth}
	}

	if !isExists(inputCycle.Year, inputCycle.Month, inputCycle.BillingCycle) {
		start := startOfMonth(inputCycle.Year, inputCycle.Month+1)
		end := endOfDay(inputCycle.Year, inputCycle.Month+1, inputCycle.BillingCycle-1)
		return DateRange{Start: start, End: end}
	}

	start := startOfDay(inputCycle.Year, inputCycle.Month, inputCycle.BillingCycle)
	end := endOfDay(inputCycle.Year, inputCycle.Month+1, inputCycle.BillingCycle-1)
	return DateRange{Start: start, End: end}
}

// endOfDay は指定日の末尾時刻を UTC で返します。
func endOfDay(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 23, 59, 59, 0, time.UTC)
}

// startOfDay は指定日の開始時刻を UTC で返します。
func startOfDay(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// endOfMonth は指定月の末尾時刻を UTC で返します。
func endOfMonth(year int, month int) time.Time {
	firstOfMonth := endOfDay(year, month, 1)
	return firstOfMonth.AddDate(0, 1, -1)
}

// startOfMonth は指定月の開始時刻を UTC で返します。
func startOfMonth(year, month int) time.Time {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}

// isExists は指定した日付がその月に存在するかを返します。
func isExists(year int, month int, day int) bool {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Day() == day
}
