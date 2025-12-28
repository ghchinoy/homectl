package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/ghchinoy/control/pkg/sonos"
	"github.com/spf13/cobra"
)

var sonosCmd = &cobra.Command{
	Use:   "sonos",
	Short: "Control Sonos speakers",
}

var listSonosCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered Sonos speakers",
	Run: func(cmd *cobra.Command, args []string) {
		// Hardcoded for now based on discovery, in future use mDNS
		speakers := []struct {
			Name string
			IP   string
		}{
			{"TV Room", "192.168.4.100"},
			{"Move 2", "192.168.4.120"},
			{"Whole House", "192.168.4.101"},
			{"Office", "192.168.4.99"},
		}

		fmt.Printf("%-20s %-15s %-10s %-10s\n", "NAME", "IP", "VOLUME", "STATUS")
		fmt.Println("------------------------------------------------------------")
		for _, s := range speakers {
			client := sonos.NewClient(s.IP)
			vol, err := client.GetVolume()
			volStr := strconv.Itoa(vol) + "%"
			if err != nil {
				volStr = "Error"
			}
			
			status := "-"
			info, err := client.GetTransportInfo()
			if err == nil {
				status = info.CurrentTransportState
			}

			fmt.Printf("%-20s %-15s %-10s %-10s\n", s.Name, s.IP, volStr, status)
		}
	},
}

var playSonosCmd = &cobra.Command{
	Use:   "play [ip]",
	Short: "Start playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		if err := client.Play(); err != nil {
			log.Fatalf("Failed to play: %v", err)
		}
		fmt.Println("Playing...")
	},
}

var pauseSonosCmd = &cobra.Command{
	Use:   "pause [ip]",
	Short: "Pause playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		if err := client.Pause(); err != nil {
			log.Fatalf("Failed to pause: %v", err)
		}
		fmt.Println("Paused.")
	},
}

var stopSonosCmd = &cobra.Command{
	Use:   "stop [ip]",
	Short: "Stop playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		if err := client.Stop(); err != nil {
			log.Fatalf("Failed to stop: %v", err)
		}
		fmt.Println("Stopped.")
	},
}

var nowPlayingCmd = &cobra.Command{
	Use:   "now-playing [ip]",
	Short: "Show what is currently playing on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		info, err := client.GetPositionInfo()
		if err != nil {
			log.Fatalf("Failed to get position info: %v", err)
		}
		
		fmt.Printf("Track: %d\n", info.Track)
		fmt.Printf("Duration: %s\n", info.TrackDuration)
		fmt.Printf("RelTime: %s\n", info.RelTime)
		fmt.Printf("URI: %s\n", info.TrackURI)
		// Metadata is XML encoded, skipping pretty printing for now
		fmt.Printf("Metadata: %s\n", info.TrackMetaData)
	},
}

var setSonosVolumeCmd = &cobra.Command{
	Use:   "volume [ip] [0-100]",
	Short: "Set volume for a Sonos speaker",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ip := args[0]
		vol, _ := strconv.Atoi(args[1])

		client := sonos.NewClient(ip)
		fmt.Printf("Setting volume for %s to %d%%...\n", ip, vol)
		if err := client.SetVolume(vol); err != nil {
			log.Fatalf("Failed to set volume: %v", err)
		}
		fmt.Println("Success!")
	},
}

func init() {
	rootCmd.AddCommand(sonosCmd)
	sonosCmd.AddCommand(listSonosCmd)
	sonosCmd.AddCommand(playSonosCmd)
	sonosCmd.AddCommand(pauseSonosCmd)
	sonosCmd.AddCommand(stopSonosCmd)
	sonosCmd.AddCommand(nowPlayingCmd)
	sonosCmd.AddCommand(setSonosVolumeCmd)
}
