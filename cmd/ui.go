package cmd

import (
	"fmt"

	"github.com/ghchinoy/homectl/pkg/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch the interactive Terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		if err := tui.Start(client); err != nil {
			return fmt.Errorf("tui error: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
