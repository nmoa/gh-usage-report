package main

import (
	"encoding/csv"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func readCSVRecords(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}
	data = bytes.TrimPrefix(data, utf8BOM)

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read CSV: %v", err)
	}

	return records
}

func TestGenerateCSV(t *testing.T) {
	orgReport := OrganizationReport{
		UsageByOrg: []OrgUsage{
			{Organization: "org-a", Usage: Usage{GrossAmount: 100.0, DiscountAmount: 10.0, NetAmount: 90.0}},
			{Organization: "org-b", Usage: Usage{GrossAmount: 50.0, DiscountAmount: 5.0, NetAmount: 45.0}},
		},
		SumUsage: Usage{GrossAmount: 150.0, DiscountAmount: 15.0, NetAmount: 135.0},
		UsageItems: []UsageItem{
			{
				Date:             "2024-01-15",
				Product:          "actions",
				SKU:              "actions_linux",
				Quantity:         100,
				UnitType:         "minutes",
				PricePerUnit:     0.008,
				GrossAmount:      0.80,
				DiscountAmount:   0.08,
				NetAmount:        0.72,
				OrganizationName: "org-a",
				RepositoryName:   "org-a/repo-1",
			},
		},
	}

	tmpDir := t.TempDir()
	filePrefix := "GitHub_Usage_test"

	err := GenerateCSV(tmpDir, filePrefix, orgReport)
	if err != nil {
		t.Fatalf("GenerateCSV returned error: %v", err)
	}

	t.Run("summary CSV", func(t *testing.T) {
		summaryPath := filepath.Join(tmpDir, filePrefix+"_summary.csv")
		records := readCSVRecords(t, summaryPath)

		// Header + 2 orgs + total = 4 rows
		if len(records) != 4 {
			t.Errorf("Expected 4 rows, got %d", len(records))
		}

		// Check header
		expectedHeader := []string{"Organization", "Gross Amount", "Applied Discount", "Net Amount"}
		for i, h := range expectedHeader {
			if records[0][i] != h {
				t.Errorf("Header[%d]: expected %q, got %q", i, h, records[0][i])
			}
		}

		// Check first org row
		if records[1][0] != "org-a" {
			t.Errorf("Expected org-a, got %s", records[1][0])
		}
		if records[1][1] != "100.00" {
			t.Errorf("Expected 100.00, got %s", records[1][1])
		}

		// Check total row
		if records[3][0] != "Total" {
			t.Errorf("Expected Total row, got %s", records[3][0])
		}
		if records[3][3] != "135.00" {
			t.Errorf("Expected total net 135.00, got %s", records[3][3])
		}
	})

	t.Run("details CSV", func(t *testing.T) {
		detailsPath := filepath.Join(tmpDir, filePrefix+"_details.csv")
		records := readCSVRecords(t, detailsPath)

		// Header + 1 item = 2 rows
		if len(records) != 2 {
			t.Errorf("Expected 2 rows, got %d", len(records))
		}

		// Check header
		expectedHeader := []string{"Date", "Product", "SKU", "Quantity", "Unit Type", "Price Per Unit", "Gross Amount", "Discount Amount", "Net Amount", "Organization Name", "Repository Name"}
		for i, h := range expectedHeader {
			if records[0][i] != h {
				t.Errorf("Header[%d]: expected %q, got %q", i, h, records[0][i])
			}
		}

		// Check data row
		if records[1][0] != "2024-01-15" {
			t.Errorf("Expected date 2024-01-15, got %s", records[1][0])
		}
		if records[1][1] != "actions" {
			t.Errorf("Expected product actions, got %s", records[1][1])
		}
		if records[1][9] != "org-a" {
			t.Errorf("Expected org org-a, got %s", records[1][9])
		}
	})
}

func TestGenerateCSV_DeletedOrganization(t *testing.T) {
	orgReport := OrganizationReport{
		UsageByOrg: []OrgUsage{
			{Organization: "", Usage: Usage{GrossAmount: 10.0, DiscountAmount: 1.0, NetAmount: 9.0}},
		},
		SumUsage:   Usage{GrossAmount: 10.0, DiscountAmount: 1.0, NetAmount: 9.0},
		UsageItems: []UsageItem{},
	}

	tmpDir := t.TempDir()
	err := GenerateCSV(tmpDir, "test", orgReport)
	if err != nil {
		t.Fatalf("GenerateCSV returned error: %v", err)
	}

	records := readCSVRecords(t, filepath.Join(tmpDir, "test_summary.csv"))

	if records[1][0] != "[DELETED ORGANIZATION(S)]" {
		t.Errorf("Expected [DELETED ORGANIZATION(S)], got %s", records[1][0])
	}
}

func TestGenerateCSV_WritesUTF8BOM(t *testing.T) {
	orgReport := OrganizationReport{}

	tmpDir := t.TempDir()
	if err := GenerateCSV(tmpDir, "test", orgReport); err != nil {
		t.Fatalf("GenerateCSV returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "test_summary.csv"))
	if err != nil {
		t.Fatalf("Failed to read summary CSV: %v", err)
	}

	if !bytes.HasPrefix(data, utf8BOM) {
		t.Fatalf("Expected UTF-8 BOM at start of CSV file")
	}
}
