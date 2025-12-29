package cmd

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/ghchinoy/homectl/pkg/sonos"
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
		// Load from cache first
		speakers, _ := sonos.LoadCache()
		if len(speakers) > 0 {
			fmt.Printf("Loaded %d speakers from cache. Refreshing...\n", len(speakers))
		} else {
			fmt.Println("No cache found. Discovering Sonos speakers...")
		}

		// Perform discovery and save
		newSpeakers, err := sonos.Discover(5 * time.Second)
		if err == nil && len(newSpeakers) > 0 {
			sonos.SaveCache(newSpeakers)
		} else if err != nil {
			fmt.Printf("Discovery error: %v\n", err)
		}

		// Reload merged cache for display
		speakers, _ = sonos.LoadCache()

		if len(speakers) == 0 {
			fmt.Println("No Sonos speakers found.")
			return
		}

		fmt.Printf("%-20s %-15s %-10s %-15s %-30s\n", "NAME", "IP", "VOLUME", "STATUS", "NOW PLAYING")
		fmt.Println("---------------------------------------------------------------------------------------------------------")
		for _, s := range speakers {
			client := sonos.NewClient(s.IP)
			vol, err := client.GetVolume()
			volStr := strconv.Itoa(vol) + "%"
			if err != nil {
				volStr = "Error"
			}
			
			status := "-"
			track := "-"
			info, err := client.GetTransportInfo()
			if err == nil {
				status = info.CurrentTransportState
				pos, _ := client.GetPositionInfo()
				meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
				if meta.Title != "" {
					track = meta.Title
				}
			}

			fmt.Printf("%-20s %-15s %-10s %-15s %-30s\n", s.Name, s.IP, volStr, status, track)
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

var nextSonosCmd = &cobra.Command{
	Use:   "next [ip]",
	Short: "Skip to the next track on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		if err := client.Next(); err != nil {
			log.Fatalf("Failed to skip next: %v", err)
		}
		fmt.Println("Skipped to next.")
	},
}

var prevSonosCmd = &cobra.Command{
	Use:   "prev [ip]",
	Short: "Skip to the previous track on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		if err := client.Previous(); err != nil {
			log.Fatalf("Failed to skip previous: %v", err)
		}
		fmt.Println("Skipped to previous.")
	},
}

var nowPlayingCmd = &cobra.Command{
	Use:   "now-playing [ip]",
	Short: "Show what is currently playing on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		pos, err := client.GetPositionInfo()
		if err != nil {
			log.Fatalf("Failed to get position info: %v", err)
		}
		
		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
		
		fmt.Printf("Title:    %s\n", meta.Title)
		fmt.Printf("Artist:   %s\n", meta.Artist)
		fmt.Printf("Album:    %s\n", meta.Album)
		fmt.Printf("Duration: %s\n", pos.TrackDuration)
		fmt.Printf("Progress: %s\n", pos.RelTime)
		fmt.Printf("URI:      %s\n", pos.TrackURI)
	},
}

var sonosDetailsCmd = &cobra.Command{
	Use:   "details [ip]",
	Short: "Show detailed status and metadata for a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := sonos.NewClient(args[0])
		
		transport, _ := client.GetTransportInfo()
		vol, _ := client.GetVolume()
		pos, _ := client.GetPositionInfo()
		media, _ := client.GetMediaInfo()
		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
		nextMeta, _ := client.ParseTrackMetadata(media.NextURIMetaData)
		name, rincon, modelName, modelNum, _ := sonos.GetDeviceName(args[0])

		fmt.Printf("Name:     %s\n", name)
		fmt.Printf("IP:       %s\n", args[0])
		fmt.Printf("Model:    %s (%s)\n", modelName, modelNum)
		fmt.Printf("ID:       %s\n", rincon)
		fmt.Printf("Status:   %s\n", transport.CurrentTransportState)
		fmt.Printf("Volume:   %d%%\n", vol)
		fmt.Printf("Queue:    %d tracks\n", media.NrTracks)
		fmt.Println("---------------------------------")
		fmt.Printf("Track:    %s\n", meta.Title)
		fmt.Printf("Artist:   %s\n", meta.Artist)
		fmt.Printf("Album:    %s\n", meta.Album)
		if meta.StreamContent != "" {
			fmt.Printf("Stream:   %s\n", meta.StreamContent)
		}
		if meta.AudioFormat != "" {
			fmt.Printf("Format:   %s\n", meta.AudioFormat)
		}
		fmt.Printf("Duration: %s (%s)\n", pos.TrackDuration, pos.RelTime)
		if nextMeta.Title != "" {
			fmt.Printf("Next:     %s by %s\n", nextMeta.Title, nextMeta.Artist)
		}
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
	sonosCmd.AddCommand(nextSonosCmd)
	sonosCmd.AddCommand(prevSonosCmd)
	sonosCmd.AddCommand(nowPlayingCmd)
	sonosCmd.AddCommand(sonosDetailsCmd)
	sonosCmd.AddCommand(setSonosVolumeCmd)
}
