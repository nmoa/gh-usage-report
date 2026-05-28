package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func GenerateCSV(reportPath string, filePrefix string, orgReport OrganizationReport) error {
	err := generateSummaryCSV(filepath.Join(reportPath, filePrefix+"_summary.csv"), orgReport)
	if err != nil {
		return fmt.Errorf("failed to generate summary CSV: %w", err)
	}

	err = generateDetailsCSV(filepath.Join(reportPath, filePrefix+"_details.csv"), orgReport)
	if err != nil {
		return fmt.Errorf("failed to generate details CSV: %w", err)
	}

	return nil
}

func generateSummaryCSV(outputFile string, orgReport OrganizationReport) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(utf8BOM); err != nil {
		return err
	}

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"Organization", "Gross Amount", "Applied Discount", "Net Amount"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, item := range orgReport.UsageByOrg {
		orgName := item.Organization
		if orgName == "" {
			orgName = "[DELETED ORGANIZATION(S)]"
		}
		row := []string{
			orgName,
			fmt.Sprintf("%.2f", item.GrossAmount),
			fmt.Sprintf("%.2f", item.DiscountAmount*-1),
			fmt.Sprintf("%.2f", item.NetAmount),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	// Total row
	totalRow := []string{
		"Total",
		fmt.Sprintf("%.2f", orgReport.SumUsage.GrossAmount),
		fmt.Sprintf("%.2f", orgReport.SumUsage.DiscountAmount),
		fmt.Sprintf("%.2f", orgReport.SumUsage.NetAmount),
	}
	if err := w.Write(totalRow); err != nil {
		return err
	}

	return nil
}

func generateDetailsCSV(outputFile string, orgReport OrganizationReport) error {
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(utf8BOM); err != nil {
		return err
	}

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"Date",
		"Product",
		"SKU",
		"Quantity",
		"Unit Type",
		"Price Per Unit",
		"Gross Amount",
		"Discount Amount",
		"Net Amount",
		"Organization Name",
		"Repository Name",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, item := range orgReport.UsageItems {
		row := []string{
			item.Date,
			item.Product,
			item.SKU,
			fmt.Sprintf("%g", item.Quantity),
			item.UnitType,
			fmt.Sprintf("%.4f", item.PricePerUnit),
			fmt.Sprintf("%.2f", item.GrossAmount),
			fmt.Sprintf("%.2f", item.DiscountAmount),
			fmt.Sprintf("%.2f", item.NetAmount),
			item.OrganizationName,
			item.RepositoryName,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return nil
}
