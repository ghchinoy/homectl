package cmd

import (
	"github.com/spf13/cobra"
)

var lutronCmd = &cobra.Command{
	Use:   "lutron",
	Short: "Control Lutron Caseta and RA2 Select devices",
}

func init() {
	rootCmd.AddCommand(lutronCmd)
}
