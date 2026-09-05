package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ghchinoy/homectl/pkg/qolsys"
	"github.com/spf13/cobra"
)

var qolsysCmd = &cobra.Command{
	Use:   "qolsys",
	Short: "Control and monitor Qolsys IQ Panel",
}

var qolsysMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Stream events from the IQ Panel",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, _ := cmd.Flags().GetString("host")
		token, _ := cmd.Flags().GetString("token")

		if addr == "" || token == "" {
			return fmt.Errorf("both --host and --token are required")
		}

		client := qolsys.NewClient(addr, token)
		client.OnEvent = func(msg map[string]interface{}) {
			fmt.Printf("EVENT: %v\n", msg)
		}

		connectCtx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		fmt.Printf("Connecting to %s...\n", addr)
		if err := client.Connect(connectCtx); err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer client.Close()
		fmt.Println("Connected! Listening for events (Ctrl+C to stop)...")

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
			fmt.Println("\nStopping...")
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
