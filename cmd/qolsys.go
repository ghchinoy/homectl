package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/qolsys"
	"github.com/spf13/cobra"
)

var qolsysCmd = &cobra.Command{
	Use:     "qolsys",
	GroupID: "devices",
	Short:   "Control and monitor Qolsys IQ Panel security systems",
	Long:    `Connects to a Qolsys IQ Panel over local TLS port 12345 using a 6-digit C4 integration token to monitor real-time zone and partition events.`,
}

var qolsysMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Stream real-time security events from the Qolsys IQ Panel",
	Long:  `Establishes a persistent TLS connection to the Qolsys IQ Panel and streams partition and sensor events until interrupted. Resolves credentials from flags, HOMECTL_QOLSYS_HOST / HOMECTL_QOLSYS_TOKEN env vars, or config.json.`,
	Example: `  # Stream events using environment variables
  HOMECTL_QOLSYS_HOST=192.168.1.60 HOMECTL_QOLSYS_TOKEN=123456 homectl qolsys monitor

  # Stream events as newline-delimited JSON
  homectl qolsys monitor --host 192.168.1.60 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("host")
		if addr == "" {
			addr = os.Getenv("HOMECTL_QOLSYS_HOST")
		}
		token, _ := cmd.Flags().GetString("token")
		if token == "" {
			token = os.Getenv("HOMECTL_QOLSYS_TOKEN")
		}
		if addr == "" || token == "" {
			cfg := config.LoadConfig()
			if addr == "" {
				addr = cfg.QolsysHost
			}
			if token == "" {
				token = cfg.QolsysToken
			}
		}

		if addr == "" || token == "" {
			return fmt.Errorf("both host and token are required (Hint: set HOMECTL_QOLSYS_HOST and HOMECTL_QOLSYS_TOKEN or pass --host and --token)")
		}

		jsonOut := isJSON(cmd)
		client := qolsys.NewClient(addr, token)
		client.OnEvent = func(msg map[string]any) {
			if jsonOut {
				_ = json.NewEncoder(os.Stdout).Encode(msg)
				return
			}
			fmt.Printf("EVENT: %v\n", msg)
		}

		connectCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		if !jsonOut {
			fmt.Printf("Connecting to %s...\n", addr)
		}
		if err := client.Connect(connectCtx); err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer client.Close()
		if !jsonOut {
			fmt.Println("Connected! Listening for events (Ctrl+C to stop)...")
		}

		// Keep alive/Initial info request
		_ = client.Send(cmd.Context(), "INFO", nil)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		errChan := make(chan error, 1)
		go func() {
			errChan <- client.ReadLoop(cmd.Context())
		}()

		select {
		case <-sigChan:
			if !jsonOut {
				fmt.Println("\nStopping...")
			}
			return nil
		case err := <-errChan:
			return fmt.Errorf("read error: %w", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(qolsysCmd)
	qolsysCmd.AddCommand(qolsysMonitorCmd)

	qolsysCmd.PersistentFlags().String("host", "", "IQ Panel IP address")
	qolsysCmd.PersistentFlags().String("token", "", "6-digit Access Token")
}
