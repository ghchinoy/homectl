package cmd

import (
	"context"
	"fmt"
	"log"
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
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("host")
		token, _ := cmd.Flags().GetString("token")

		if addr == "" || token == "" {
			log.Fatal("Host and Token are required")
		}

		client := qolsys.NewClient(addr, token)
		client.OnEvent = func(msg map[string]interface{}) {
			fmt.Printf("EVENT: %v\n", msg)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Printf("Connecting to %s...\n", addr)
		if err := client.Connect(ctx); err != nil {
			log.Fatalf("Connection failed: %v", err)
		}
		defer client.Close()
		fmt.Println("Connected! Listening for events (Ctrl+C to stop)...")

		// Keep alive/Initial info request
		client.Send(context.Background(), "INFO", nil)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		errChan := make(chan error, 1)
		go func() {
			errChan <- client.ReadLoop(context.Background())
		}()

		select {
		case <-sigChan:
			fmt.Println("\nStopping...")
		case err := <-errChan:
			log.Fatalf("Read error: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(qolsysCmd)
	qolsysCmd.AddCommand(qolsysMonitorCmd)

	qolsysCmd.PersistentFlags().String("host", "", "IQ Panel IP address")
	qolsysCmd.PersistentFlags().String("token", "", "6-digit Access Token")
}
