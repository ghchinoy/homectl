package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ghchinoy/homectl/pkg/tui"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:     "ui",
	GroupID: "services",
	Short:   "Launch the interactive Terminal UI",
	Long:    `Starts an interactive Bubble Tea terminal dashboard for browsing and controlling Lutron lighting zones, areas, and Sonos speakers. Disabled when HOMECTL_NO_TUI=1 or --json is passed.`,
	Example: `  # Launch the interactive TUI
  homectl ui

  # Connect to a specific Lutron bridge IP in the TUI
  homectl ui --bridge 192.168.1.20`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsNoTUI() || isJSON(cmd) {
			msg := "interactive TUI is disabled when HOMECTL_NO_TUI is set or --json is requested (Hint: use 'homectl lutron list devices --json' or 'homectl sonos list --json')"
			if isJSON(cmd) {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
					"status":   "error",
					"error":    msg,
					"no_tui":   IsNoTUI(),
					"no_color": IsNoColor(),
				})
			}
			return fmt.Errorf("%s", msg)
		}

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
