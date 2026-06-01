package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBillingCycle_GetDateRange は請求期間の境界を検証します。
func TestBillingCycle_GetDateRange(t *testing.T) {
	tests := []struct {
		name          string
		input         InputCycle
		expectedStart string
		expectedEnd   string
	}{
		{
			name:          "if billing cycle is 1, returns the start and end of the given month",
			input:         InputCycle{Year: 2022, Month: 5, BillingCycle: 1},
			expectedStart: "2022-05-01T00:00:00Z",
			expectedEnd:   "2022-05-31T23:59:59Z",
		},
		{
			name:          "if billing cycle is > 1, creates a date range from the given month and the end of the previous day of the next month",
			input:         InputCycle{Year: 2022, Month: 5, BillingCycle: 2},
			expectedStart: "2022-05-02T00:00:00Z",
			expectedEnd:   "2022-06-01T23:59:59Z",
		},
		{
			name:          "handles billing dates for months that do not contain the billing cycle date by having the first of the next month as start date",
			input:         InputCycle{Year: 2024, Month: 2, BillingCycle: 29},
			expectedStart: "2024-02-29T00:00:00Z",
			expectedEnd:   "2024-03-28T23:59:59Z",
		},
		{
			name:          "handles a billing date of 30 correct in February",
			input:         InputCycle{Year: 2024, Month: 2, BillingCycle: 30},
			expectedStart: "2024-03-01T00:00:00Z",
			expectedEnd:   "2024-03-29T23:59:59Z",
		},
		{
			name:          "handles leap years correctly when billing date is not 29",
			input:         InputCycle{Year: 2024, Month: 2, BillingCycle: 3},
			expectedStart: "2024-02-03T00:00:00Z",
			expectedEnd:   "2024-03-02T23:59:59Z",
		},
		{
			name:          "handles leap years correctly when billing date is 29",
			input:         InputCycle{Year: 2024, Month: 2, BillingCycle: 29},
			expectedStart: "2024-02-29T00:00:00Z",
			expectedEnd:   "2024-03-28T23:59:59Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBillingCycle(tt.input)
			resultStart, resultEnd := bc.GetDateRange()

			expectedStart, err := time.Parse(time.RFC3339, tt.expectedStart)
			require.NoError(t, err)
			expectedEnd, err := time.Parse(time.RFC3339, tt.expectedEnd)
			require.NoError(t, err)

			require.Equal(t, expectedStart, resultStart)
			require.Equal(t, expectedEnd, resultEnd)
		})
	}
}

// TestBillingCycle_GetStartDateString は API 用開始日文字列を検証します。
func TestBillingCycle_GetStartDateString(t *testing.T) {
	tests := []struct {
		name     string
		input    InputCycle
		expected string
	}{
		{
			name:     "if billing cycle is 1, returns the first day of the given month",
			input:    InputCycle{Year: 2022, Month: 5, BillingCycle: 1},
			expected: "2022-05-01",
		},
		{
			name:     "if billing cycle is greater than 1, returns the billing cycle start day",
			input:    InputCycle{Year: 2022, Month: 5, BillingCycle: 2},
			expected: "2022-05-02",
		},
		{
			name:     "if billing cycle day does not exist, returns the first day of the next month",
			input:    InputCycle{Year: 2024, Month: 2, BillingCycle: 31},
			expected: "2024-03-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBillingCycle(tt.input)
			result := bc.GetStartDateString()
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestBillingCycle_GetEndDateString は API 用終了日文字列を検証します。
func TestBillingCycle_GetEndDateString(t *testing.T) {
	tests := []struct {
		name     string
		input    InputCycle
		expected string
	}{
		{
			name:     "if billing cycle is 1, returns the last day of the given month",
			input:    InputCycle{Year: 2022, Month: 5, BillingCycle: 1},
			expected: "2022-05-31",
		},
		{
			name:     "if billing cycle is greater than 1, returns the day before the next cycle",
			input:    InputCycle{Year: 2022, Month: 5, BillingCycle: 2},
			expected: "2022-06-01",
		},
		{
			name:     "if billing cycle day does not exist, returns the last day before that cycle in the next month",
			input:    InputCycle{Year: 2024, Month: 2, BillingCycle: 31},
			expected: "2024-03-30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBillingCycle(tt.input)
			result := bc.GetEndDateString()
			require.Equal(t, tt.expected, result)
		})
	}
}
