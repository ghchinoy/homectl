package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set dimming levels for Lutron zones",
	Long:  `Adjusts the brightness level (0-100) for an individual Lutron zone or all controllable zones on the bridge.`,
}

var setLevelCmd = &cobra.Command{
	Use:   "level [zone_href] [0-100]",
	Short: "Set the dimming level of a specific Lutron zone",
	Long:  `Sets the dimming level (0-100) for a single Lutron zone identified by its LEAP HREF (e.g. /zone/1). Supports --dry-run and --json.`,
	Example: `  # Set zone /zone/1 to 50% brightness
  homectl lutron set level /zone/1 50

  # Preview setting /zone/1 to 0% as JSON without mutating state
  homectl lutron set level /zone/1 0 --dry-run --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		zoneHref := args[0]
		level, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return fmt.Errorf("invalid level %q: %w", args[1], err)
		}
		if level < 0 || level > 100 {
			return fmt.Errorf("level must be between 0 and 100, got %.0f", level)
		}

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "set level",
					Planned: map[string]any{"zone_href": zoneHref, "level": level},
					Message: fmt.Sprintf("[DRY-RUN] Would set %s to %.0f%%", zoneHref, level),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would set %s to %.0f%% (no changes made)\n", zoneHref, level)
			return nil
		}

		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		if !isJSON(cmd) {
			fmt.Printf("Setting %s to %.0f%%...\n", zoneHref, level)
		}
		if err := client.SetLevel(zoneHref, level); err != nil {
			return fmt.Errorf("set level: %w", err)
		}
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status":    "ok",
				"action":    "set_level",
				"zone_href": zoneHref,
				"level":     level,
			})
		}
		fmt.Println("Success!")
		return nil
	},
}

var setAllCmd = &cobra.Command{
	Use:   "all [0-100]",
	Short: "Set the dimming level for all Lutron lights",
	Long:  `Sets the dimming level (0-100) across all controllable Lutron lighting zones on the bridge. Supports --dry-run and --json.`,
	Example: `  # Turn off all Lutron lights
  homectl lutron set all 0

  # Simulate setting all lights to 75% in JSON mode
  homectl lutron set all 75 --dry-run --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		level, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return fmt.Errorf("invalid level %q: %w", args[0], err)
		}
		if level < 0 || level > 100 {
			return fmt.Errorf("level must be between 0 and 100, got %.0f", level)
		}

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "set all",
					Planned: map[string]any{"level": level},
					Message: fmt.Sprintf("[DRY-RUN] Would set all lights to %.0f%%", level),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would set all lights to %.0f%% (no changes made)\n", level)
			return nil
		}

		client, err := getClient(cmd)
		if err != nil {
			return err
		}
		defer client.Close()

		if !isJSON(cmd) {
			fmt.Printf("Setting all lights to %.0f%%...\n", level)
		}
		if err := client.SetAllLevels(level); err != nil {
			return fmt.Errorf("set all levels: %w", err)
		}
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"status": "ok",
				"action": "set_all",
				"level":  level,
			})
		}
		fmt.Println("Success!")
		return nil
	},
}

func init() {
	lutronCmd.AddCommand(setCmd)
	setCmd.AddCommand(setLevelCmd)
	setCmd.AddCommand(setAllCmd)
}
