package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ghchinoy/homectl/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	GroupID: "services",
	Short:   "Print the homectl version, commit, and build info",
	Long:    `Prints the current homectl semantic version, git commit hash, and build timestamp in human-readable or JSON format.`,
	Example: `  # Print version string
  homectl version

  # Output detailed version metadata as JSON
  homectl version --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag := isJSON(cmd)
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
