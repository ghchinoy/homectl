package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ghchinoy/homectl/pkg/sonos"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the homectl API server",
	Long:  `Starts an HTTP server that provides an API for controlling Lutron and Sonos devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		
		client := getClient(cmd)
		defer client.Close()

		http.HandleFunc("/api/lutron/devices", func(w http.ResponseWriter, r *http.Request) {
			devices, err := client.GetDevices()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devices)
		})

		http.HandleFunc("/api/lutron/status", func(w http.ResponseWriter, r *http.Request) {
			statuses, err := client.GetAllZoneStatuses()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statuses)
		})

		http.HandleFunc("/api/sonos/devices", func(w http.ResponseWriter, r *http.Request) {
			devices, err := sonos.LoadCache()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devices)
		})

		fmt.Printf("Starting homectl API server on :%d\n", port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
}
