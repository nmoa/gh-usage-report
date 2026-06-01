package main

import (
	"github.com/nmoa/gh-usage-report/internal/cli"
	"github.com/spf13/cobra"
)

// main は CLI を実行します。
func main() {
	cobra.CheckErr(cli.NewRootCmd().Execute())
}
