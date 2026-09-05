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
	Short: "Set levels for devices",
}

var setLevelCmd = &cobra.Command{
	Use:   "level [zone_href] [0-100]",
	Short: "Set the dimming level of a zone",
	Args:  cobra.ExactArgs(2),
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

		fmt.Printf("Setting %s to %.0f%%...\n", zoneHref, level)
		if err := client.SetLevel(zoneHref, level); err != nil {
			return fmt.Errorf("set level: %w", err)
		}
		fmt.Println("Success!")
		return nil
	},
}

var setAllCmd = &cobra.Command{
	Use:   "all [0-100]",
	Short: "Set the level for all lights",
	Args:  cobra.ExactArgs(1),
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

		fmt.Printf("Setting all lights to %.0f%%...\n", level)
		if err := client.SetAllLevels(level); err != nil {
			return fmt.Errorf("set all levels: %w", err)
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
