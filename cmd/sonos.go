package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/config"
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

// SonosFavoritesOutput represents the structured JSON output for favorites.
type SonosFavoritesOutput struct {
	Count     int              `json:"count"`
	Favorites []sonos.Favorite `json:"favorites"`
}

// SonosServicesOutput represents the structured JSON output for music services.
type SonosServicesOutput struct {
	Count    int                  `json:"count"`
	Default  *sonos.MusicService  `json:"default,omitempty"`
	Services []sonos.MusicService `json:"services"`
}

func resolveSpeakerIP(arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	devices, err := sonos.LoadCache()
	if err == nil && len(devices) > 0 {
		return devices[0].IP, nil
	}
	discovered, err := sonos.Discover(2 * time.Second)
	if err == nil && len(discovered) > 0 {
		return discovered[0].IP, nil
	}
	return "", fmt.Errorf("no Sonos speaker IP provided and none discovered")
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

var seekSonosCmd = &cobra.Command{
	Use:   "seek [ip]",
	Short: "Seek to a track number or time offset on a Sonos speaker",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		track, _ := cmd.Flags().GetInt("track")
		timeStr, _ := cmd.Flags().GetString("time")

		if track == 0 && timeStr == "" {
			return fmt.Errorf("must specify either --track <number> or --time <[H:]MM:SS>")
		}
		if track != 0 && timeStr != "" {
			return fmt.Errorf("cannot specify both --track and --time")
		}
		if track < 0 {
			return fmt.Errorf("track number must be >= 1")
		}

		if isDryRun(cmd) {
			planned := map[string]any{"ip": ip}
			msg := ""
			if track > 0 {
				planned["track"] = track
				msg = fmt.Sprintf("[DRY-RUN] Would seek to track %d on %s", track, ip)
			} else {
				planned["time"] = timeStr
				msg = fmt.Sprintf("[DRY-RUN] Would seek to time %s on %s", timeStr, ip)
			}

			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "sonos seek",
					Planned: planned,
					Message: msg,
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: %s (no changes made)\n", msg)
			return nil
		}

		client := sonos.NewClient(ip)
		if track > 0 {
			if err := client.SeekTrack(track); err != nil {
				return fmt.Errorf("seek track: %w", err)
			}
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"status": "ok",
					"action": "seek_track",
					"ip":     ip,
					"track":  track,
				})
			}
			fmt.Printf("Successfully jumped to track %d on %s.\n", track, ip)
		} else {
			if err := client.SeekTime(timeStr); err != nil {
				return fmt.Errorf("seek time: %w", err)
			}
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"status": "ok",
					"action": "seek_time",
					"ip":     ip,
					"time":   timeStr,
				})
			}
			fmt.Printf("Successfully sought to %s on %s.\n", timeStr, ip)
		}

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

var favoritesSonosCmd = &cobra.Command{
	Use:   "favorites [ip]",
	Short: "List pinned favorites from a Sonos speaker",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var rawIP string
		if len(args) > 0 {
			rawIP = args[0]
		}
		ip, err := resolveSpeakerIP(rawIP)
		if err != nil {
			return err
		}

		client := sonos.NewClient(ip)
		favs, err := client.BrowseFavorites()
		if err != nil {
			return fmt.Errorf("list favorites: %w", err)
		}

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(SonosFavoritesOutput{
				Count:     len(favs),
				Favorites: favs,
			})
		}

		fmt.Printf("Sonos Favorites on %s (%d found):\n\n", ip, len(favs))
		fmt.Printf("%-12s %-30s %-20s %-15s\n", "ID", "TITLE", "TYPE", "DESCRIPTION")
		fmt.Println(strings.Repeat("-", 80))
		for _, f := range favs {
			typ := f.Type
			if lastSlash := strings.LastIndex(typ, "."); lastSlash != -1 {
				typ = typ[lastSlash+1:]
			}
			fmt.Printf("%-12s %-30s %-20s %-15s\n", f.ID, f.Title, typ, f.Description)
		}
		return nil
	},
}

var playFavoriteSonosCmd = &cobra.Command{
	Use:   "play-favorite [ip] [favorite-id]",
	Short: "Play a pinned favorite by ID on a Sonos speaker",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		favID := args[1]

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "sonos play-favorite",
					Planned: map[string]any{"ip": ip, "favorite_id": favID},
					Message: fmt.Sprintf("[DRY-RUN] Would play favorite %q on %s", favID, ip),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would play favorite %q on %s (no changes made)\n", favID, ip)
			return nil
		}

		client := sonos.NewClient(ip)
		fmt.Printf("Playing favorite %q on %s...\n", favID, ip)
		if err := client.PlayFavorite(favID); err != nil {
			return fmt.Errorf("play favorite: %w", err)
		}
		fmt.Println("Success!")
		return nil
	},
}

var playStreamSonosCmd = &cobra.Command{
	Use:   "play-stream [ip] [url]",
	Short: "Play an HTTP/HTTPS audio stream on a Sonos speaker",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		streamURL := args[1]

		u, err := url.Parse(strings.TrimSpace(streamURL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid stream URL %q: scheme must be http or https", streamURL)
		}

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			title = u.Host
			if title == "" {
				title = "Audio Stream"
			}
		}

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "sonos play-stream",
					Planned: map[string]any{"ip": ip, "url": streamURL, "title": title},
					Message: fmt.Sprintf("[DRY-RUN] Would play stream %q (%s) on %s", title, streamURL, ip),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would play stream %q (%s) on %s (no changes made)\n", title, streamURL, ip)
			return nil
		}

		client := sonos.NewClient(ip)
		fmt.Printf("Starting stream %q (%s) on %s...\n", title, streamURL, ip)
		if err := client.PlayStream(streamURL, title); err != nil {
			return fmt.Errorf("play stream: %w", err)
		}
		fmt.Println("Success!")
		return nil
	},
}

var queueAddSonosCmd = &cobra.Command{
	Use:   "queue-add [ip] [uri]",
	Short: "Add an audio track/stream URI to the Sonos queue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := args[0]
		uri := args[1]
		asNext, _ := cmd.Flags().GetBool("next")

		if isDryRun(cmd) {
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(DryRunResult{
					DryRun:  true,
					Command: "sonos queue-add",
					Planned: map[string]any{"ip": ip, "uri": uri, "next": asNext},
					Message: fmt.Sprintf("[DRY-RUN] Would add URI %q to queue on %s (next=%v)", uri, ip, asNext),
				})
			}
			fmt.Printf("[DRY-RUN] Simulating: Would add URI %q to queue on %s (next=%v, no changes made)\n", uri, ip, asNext)
			return nil
		}

		client := sonos.NewClient(ip)
		pos, err := client.AddURIToQueue(uri, "", asNext)
		if err != nil {
			return fmt.Errorf("queue add: %w", err)
		}
		fmt.Printf("Enqueued at track position %d on %s\n", pos, ip)
		return nil
	},
}

var servicesSonosCmd = &cobra.Command{
	Use:   "services [ip]",
	Short: "List available music streaming services on a Sonos speaker",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var rawIP string
		if len(args) > 0 {
			rawIP = args[0]
		}
		ip, err := resolveSpeakerIP(rawIP)
		if err != nil {
			return err
		}

		client := sonos.NewClient(ip)
		services, err := client.ListMusicServices()
		if err != nil {
			return fmt.Errorf("list services: %w", err)
		}

		cfg := config.LoadConfig()
		defaultService, hasDefault := sonos.ResolveDefaultService(services, cfg.SonosDefaultService)

		if isJSON(cmd) {
			var defPtr *sonos.MusicService
			if hasDefault {
				defPtr = &defaultService
			}
			return json.NewEncoder(os.Stdout).Encode(SonosServicesOutput{
				Count:    len(services),
				Default:  defPtr,
				Services: services,
			})
		}

		fmt.Printf("Sonos Music Services on %s (%d available):\n", ip, len(services))
		if hasDefault {
			fmt.Printf("Default Service: %s (ID: %s)\n\n", defaultService.Name, defaultService.ID)
		} else {
			fmt.Println()
		}
		fmt.Printf("%-6s %-30s %-8s %-10s\n", "ID", "NAME", "DEFAULT", "VERSION")
		fmt.Println(strings.Repeat("-", 60))
		for _, s := range services {
			isDef := ""
			if s.IsDefault {
				isDef = "*"
			}
			fmt.Printf("%-6s %-30s %-8s %-10s\n", s.ID, s.Name, isDef, s.Version)
		}
		return nil
	},
}

var queueSonosCmd = &cobra.Command{
	Use:   "queue [ip]",
	Short: "View items in the Sonos playback queue",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var rawIP string
		if len(args) > 0 {
			rawIP = args[0]
		}
		ip, err := resolveSpeakerIP(rawIP)
		if err != nil {
			return err
		}

		start, _ := cmd.Flags().GetInt("start")
		count, _ := cmd.Flags().GetInt("count")

		client := sonos.NewClient(ip)
		qRes, err := client.GetQueue(start, count)
		if err != nil {
			return fmt.Errorf("get queue: %w", err)
		}

		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(qRes)
		}

		fmt.Printf("Sonos Playback Queue on %s (showing %d of %d tracks, start: %d):\n\n",
			ip, qRes.Returned, qRes.TotalMatches, qRes.StartIndex)
		if len(qRes.Items) == 0 {
			fmt.Println("Queue is empty.")
			return nil
		}

		fmt.Printf("%-5s %-30s %-25s %-25s %-8s\n", "POS", "TITLE", "ARTIST", "ALBUM", "DURATION")
		fmt.Println(strings.Repeat("-", 98))
		for _, item := range qRes.Items {
			title := item.Title
			if len(title) > 28 {
				title = title[:25] + "..."
			}
			artist := item.Artist
			if len(artist) > 23 {
				artist = artist[:20] + "..."
			}
			album := item.Album
			if len(album) > 23 {
				album = album[:20] + "..."
			}
			fmt.Printf("%-5d %-30s %-25s %-25s %-8s\n",
				item.Position, title, artist, album, item.Duration)
		}
		return nil
	},
}

func init() {
	playStreamSonosCmd.Flags().String("title", "", "Descriptive title for the stream (defaults to URL host)")
	queueAddSonosCmd.Flags().Bool("next", false, "Insert track as next in queue instead of appending to end")
	queueSonosCmd.Flags().Int("start", 0, "0-based starting index in the queue (default 0)")
	queueSonosCmd.Flags().Int("count", 20, "Number of tracks to return (default 20)")
	seekSonosCmd.Flags().Int("track", 0, "1-based track number in the queue to jump to")
	seekSonosCmd.Flags().String("time", "", "Time offset to seek to in [H:]MM:SS format (e.g. '1:30' or '0:02:15')")

	rootCmd.AddCommand(sonosCmd)
	sonosCmd.AddCommand(listSonosCmd)
	sonosCmd.AddCommand(playSonosCmd)
	sonosCmd.AddCommand(pauseSonosCmd)
	sonosCmd.AddCommand(stopSonosCmd)
	sonosCmd.AddCommand(nextSonosCmd)
	sonosCmd.AddCommand(prevSonosCmd)
	sonosCmd.AddCommand(seekSonosCmd)
	sonosCmd.AddCommand(nowPlayingCmd)
	sonosCmd.AddCommand(sonosDetailsCmd)
	sonosCmd.AddCommand(setSonosVolumeCmd)
	sonosCmd.AddCommand(favoritesSonosCmd)
	sonosCmd.AddCommand(playFavoriteSonosCmd)
	sonosCmd.AddCommand(playStreamSonosCmd)
	sonosCmd.AddCommand(queueAddSonosCmd)
	sonosCmd.AddCommand(queueSonosCmd)
	sonosCmd.AddCommand(servicesSonosCmd)
}
