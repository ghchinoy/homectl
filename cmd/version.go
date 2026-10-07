package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ghchinoy/homectl/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the homectl version, commit, and build info",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		info := version.Get()
		if jsonFlag {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "homectl %s\n", version.String())
		return nil
	},
}

func init() {
	rootCmd.Version = version.Version
	rootCmd.SetVersionTemplate("homectl {{printf \"v%s\" .Version}}\n")
	versionCmd.Flags().Bool("json", false, "Output version information in JSON format")
	rootCmd.AddCommand(versionCmd)
}
