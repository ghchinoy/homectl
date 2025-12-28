
package cmd

import (
	"log"

	"github.com/ghchinoy/control/pkg/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch the interactive Terminal UI",
	Run: func(cmd *cobra.Command, args []string) {
		client := getClient(cmd)
		defer client.Close()

		if err := tui.Start(client); err != nil {
			log.Fatalf("TUI Error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
