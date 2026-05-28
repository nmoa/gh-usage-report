package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.PersistentFlags().String("github-token", "", "Github token")
	viper.BindPFlag("github-token", rootCmd.PersistentFlags().Lookup("github-token"))
	viper.BindEnv("github-token", "GITHUB_TOKEN")

	rootCmd.PersistentFlags().String("enterprise", "", "Enterprise name")
	rootCmd.MarkPersistentFlagRequired("enterprise")

	currentYear := time.Now().Year()
	rootCmd.PersistentFlags().Int("year", currentYear, "Specify the year, e.g. 2024")

	currentMonth := int(time.Now().Month())
	rootCmd.PersistentFlags().Int("month", currentMonth, "Specify the month, e.g. 1")

	rootCmd.PersistentFlags().Int("billing-cycle", 1, "First day of your billing cycle, e.g. 15")

	rootCmd.PersistentFlags().String("report-path", "./reports", "Path where the report will be generated (path will be generated recursively if it does not exist)")

	rootCmd.PersistentFlags().Bool("csv", false, "Output in CSV format instead of Excel")
}

var rootCmd = &cobra.Command{
	Use:   "gh billing-export",
	Short: "Generate a Billing Report",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		givenYear, _ := cmd.Flags().GetInt("year")
		givenMonth, _ := cmd.Flags().GetInt("month")
		givenBillingCycle, _ := cmd.Flags().GetInt("billing-cycle")
		githubToken, _ := cmd.Flags().GetString("github-token")
		enterprise, _ := cmd.Flags().GetString("enterprise")
		reportPath, _ := cmd.Flags().GetString("report-path")
		csvOutput, _ := cmd.Flags().GetBool("csv")

		inputCycle := InputCycle{
			Year:         givenYear,
			Month:        givenMonth,
			BillingCycle: givenBillingCycle,
		}
		billingCycle := NewBillingCycle(inputCycle)

		log.Printf("Date Range: %s\n", billingCycle.GetDateRangeAsString())

		opts := api.ClientOptions{
			AuthToken: githubToken, // Replace with valid auth token.
			Log:       os.Stdout,
		}

		client, _ := api.NewRESTClient(opts)
		octokit := Octokit{client}

		allUsageItems, err := octokit.getUsageItemsForDates(enterprise, billingCycle.GetRequiredAPIDateRange())

		if err != nil {
			log.Fatal("Error while getting usageItems from API. Make sure you have provided a GITHUB_TOKEN with the scopes 'manage_billing:enterprise' and 'read:enterprise'.\nOriginal error:\n", err)
		}

		// Filter usage Items using the billing_cycle IsInDateRange method
		relevantUsageItems := FilterUsageItems(allUsageItems, billingCycle.IsInDateRange)
		log.Printf("Found %d usageItems of which %d are in given Billing-Cycle\n", len(allUsageItems), len(relevantUsageItems))

		orgReport := NewOrganizationReport(relevantUsageItems)

		// ensure the reportPath exists
		err = os.MkdirAll(reportPath, os.ModePerm)

		filePrefix := fmt.Sprintf("GitHub_Usage_%s", billingCycle.GetDateRangeAsString())

		if csvOutput {
			err = GenerateCSV(reportPath, filePrefix, orgReport)
			if err != nil {
				log.Fatalf("Error while generating CSV files: \n%s\n", err)
			}
		} else {
			excelFileName := filepath.Join(reportPath, filePrefix+".xlsx")
			err = GenerateExcel(excelFileName, orgReport)
			if err != nil {
				log.Fatalf("Error while generating Excel file: \n%s\n", err)
			}
		}

		log.Printf("Report generated at %s\n", reportPath)
	},
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
