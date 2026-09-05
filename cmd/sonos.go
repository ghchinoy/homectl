package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/spf13/cobra"
)

var sonosCmd = &cobra.Command{
	Use:   "sonos",
	Short: "Control Sonos speakers",
}

var listSonosCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered Sonos speakers",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			_ = sonos.SaveCache(newSpeakers)
		} else if err != nil {
			fmt.Printf("Discovery error: %v\n", err)
		}

		// Reload merged cache for display
		speakers, _ = sonos.LoadCache()

		if len(speakers) == 0 {
			fmt.Println("No Sonos speakers found.")
			return nil
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
		return nil
	},
}

var playSonosCmd = &cobra.Command{
	Use:   "play [ip]",
	Short: "Start playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		if err := client.Play(); err != nil {
			return fmt.Errorf("play: %w", err)
		}
		fmt.Println("Playing...")
		return nil
	},
}

var pauseSonosCmd = &cobra.Command{
	Use:   "pause [ip]",
	Short: "Pause playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		if err := client.Pause(); err != nil {
			return fmt.Errorf("pause: %w", err)
		}
		fmt.Println("Paused.")
		return nil
	},
}

var stopSonosCmd = &cobra.Command{
	Use:   "stop [ip]",
	Short: "Stop playback on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		if err := client.Stop(); err != nil {
			return fmt.Errorf("stop: %w", err)
		}
		fmt.Println("Stopped.")
		return nil
	},
}

var nextSonosCmd = &cobra.Command{
	Use:   "next [ip]",
	Short: "Skip to the next track on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		if err := client.Next(); err != nil {
			return fmt.Errorf("next: %w", err)
		}
		fmt.Println("Skipped to next.")
		return nil
	},
}

var prevSonosCmd = &cobra.Command{
	Use:   "prev [ip]",
	Short: "Skip to the previous track on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		if err := client.Previous(); err != nil {
			return fmt.Errorf("prev: %w", err)
		}
		fmt.Println("Skipped to previous.")
		return nil
	},
}

var nowPlayingCmd = &cobra.Command{
	Use:   "now-playing [ip]",
	Short: "Show what is currently playing on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := sonos.NewClient(args[0])
		pos, err := client.GetPositionInfo()
		if err != nil {
			return fmt.Errorf("get position info: %w", err)
		}

		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)

		fmt.Printf("Title:    %s\n", meta.Title)
		fmt.Printf("Artist:   %s\n", meta.Artist)
		fmt.Printf("Album:    %s\n", meta.Album)
		fmt.Printf("Duration: %s\n", pos.TrackDuration)
		fmt.Printf("Progress: %s\n", pos.RelTime)
		fmt.Printf("URI:      %s\n", pos.TrackURI)
		return nil
	},
}

var sonosDetailsCmd = &cobra.Command{
	Use:   "details [ip]",
	Short: "Show detailed status and metadata for a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
		return nil
	},
}

var setSonosVolumeCmd = &cobra.Command{
	Use:   "volume [ip] [0-100]",
	Short: "Set volume for a Sonos speaker",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		vol, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid volume %q: %w", args[1], err)
		}

		client := sonos.NewClient(ip)
		fmt.Printf("Setting volume for %s to %d%%...\n", ip, vol)
		if err := client.SetVolume(vol); err != nil {
			return fmt.Errorf("set volume: %w", err)
		}
		fmt.Println("Success!")
		return nil
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
