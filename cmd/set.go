
package cmd

import (
	"fmt"
	"log"
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
	Run: func(cmd *cobra.Command, args []string) {
		zoneHref := args[0]
		level, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			log.Fatalf("Invalid level: %v", err)
		}

		client := getClient(cmd)
		defer client.Close()

		fmt.Printf("Setting %s to %.0f%%...\n", zoneHref, level)
		err = client.SetLevel(zoneHref, level)
		if err != nil {
			log.Fatalf("Failed to set level: %v", err)
		}
		fmt.Println("Success!")
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
	setCmd.AddCommand(setLevelCmd)
}
