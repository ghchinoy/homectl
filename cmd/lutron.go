package cmd

import (
	"github.com/spf13/cobra"
)

var lutronCmd = &cobra.Command{
	Use:     "lutron",
	GroupID: "devices",
	Short:   "Control Lutron Caseta and RA2 Select lighting devices",
	Long:    `Inspect and control Lutron Caseta and RA2 Select areas, zones, and dimmers over local mTLS LEAP.`,
}

func init() {
	rootCmd.AddCommand(lutronCmd)
}
