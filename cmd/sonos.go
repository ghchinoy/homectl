package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/spf13/cobra"
)

// SonosSpeakerSummary represents a speaker status item for structured JSON output.
type SonosSpeakerSummary struct {
	Name       string `json:"name"`
	IP         string `json:"ip"`
	RinconID   string `json:"rincon_id,omitempty"`
	Model      string `json:"model,omitempty"`
	Volume     int    `json:"volume"`
	Status     string `json:"status"`
	NowPlaying string `json:"now_playing,omitempty"`
}

// SonosNowPlayingOutput represents detailed now-playing track information for JSON output.
type SonosNowPlayingOutput struct {
	IP            string              `json:"ip"`
	Status        string              `json:"status"`
	Volume        int                 `json:"volume"`
	Duration      string              `json:"duration,omitempty"`
	Progress      string              `json:"progress,omitempty"`
	URI           string              `json:"uri,omitempty"`
	Track         sonos.TrackMetadata `json:"track"`
	StreamContent string              `json:"stream_content,omitempty"`
}

// SonosDetailsOutput represents full speaker details for JSON output.
type SonosDetailsOutput struct {
	Name        string              `json:"name"`
	IP          string              `json:"ip"`
	ModelName   string              `json:"model_name"`
	ModelNumber string              `json:"model_number"`
	ID          string              `json:"id"`
	Status      string              `json:"status"`
	Volume      int                 `json:"volume"`
	QueueCount  int                 `json:"queue_count"`
	Track       sonos.TrackMetadata `json:"track"`
	NextTrack   sonos.TrackMetadata `json:"next_track,omitempty"`
	Duration    string              `json:"duration,omitempty"`
	Progress    string              `json:"progress,omitempty"`
}

var sonosCmd = &cobra.Command{
	Use:   "sonos",
	Short: "Control Sonos speakers",
}

var listSonosCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered Sonos speakers",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut := isJSON(cmd)

		// Load from cache first
		speakers, _ := sonos.LoadCache()
		if len(speakers) > 0 {
			if !jsonOut {
				fmt.Printf("Loaded %d speakers from cache. Refreshing...\n", len(speakers))
			}
		} else {
			if !jsonOut {
				fmt.Println("No cache found. Discovering Sonos speakers...")
			}
		}

		// Perform discovery and save
		newSpeakers, err := sonos.Discover(5 * time.Second)
		if err == nil && len(newSpeakers) > 0 {
			_ = sonos.SaveCache(newSpeakers)
		} else if err != nil && !jsonOut {
			fmt.Fprintf(os.Stderr, "Discovery error: %v\n", err)
		}

		// Reload merged cache for display
		speakers, _ = sonos.LoadCache()

		if jsonOut {
			var summaries []SonosSpeakerSummary
			for _, s := range speakers {
				client := sonos.NewClient(s.IP)
				vol, _ := client.GetVolume()
				status := "-"
				track := ""
				info, err := client.GetTransportInfo()
				if err == nil {
					status = info.CurrentTransportState
					pos, _ := client.GetPositionInfo()
					meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
					if meta.Title != "" {
						track = meta.Title
					} else if meta.StreamContent != "" {
						track = meta.StreamContent
					}
				}
				summaries = append(summaries, SonosSpeakerSummary{
					Name:       s.Name,
					IP:         s.IP,
					RinconID:   s.RinconID,
					Model:      fmt.Sprintf("%s (%s)", s.ModelName, s.ModelNumber),
					Volume:     vol,
					Status:     status,
					NowPlaying: track,
				})
			}
			return json.NewEncoder(os.Stdout).Encode(summaries)
		}

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
		transport, _ := client.GetTransportInfo()
		vol, _ := client.GetVolume()
		pos, err := client.GetPositionInfo()
		if err != nil {
			return fmt.Errorf("get position info: %w", err)
		}

		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(SonosNowPlayingOutput{
				IP:            args[0],
				Status:        transport.CurrentTransportState,
				Volume:        vol,
				Duration:      pos.TrackDuration,
				Progress:      pos.RelTime,
				URI:           pos.TrackURI,
				Track:         meta,
				StreamContent: meta.StreamContent,
			})
		}

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

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(SonosDetailsOutput{
				Name:        name,
				IP:          args[0],
				ModelName:   modelName,
				ModelNumber: modelNum,
				ID:          rincon,
				Status:      transport.CurrentTransportState,
				Volume:      vol,
				QueueCount:  media.NrTracks,
				Track:       meta,
				NextTrack:   nextMeta,
				Duration:    pos.TrackDuration,
				Progress:    pos.RelTime,
			})
		}

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

		if vol < 0 || vol > 100 {
			return fmt.Errorf("volume must be between 0 and 100, got %d", vol)
		}

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "sonos volume",
					Planned: map[string]any{"ip": ip, "volume": vol},
					Message: fmt.Sprintf("[DRY-RUN] Would set volume for %s to %d%%", ip, vol),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would set volume for %s to %d%% (no changes made)\n", ip, vol)
			return nil
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
