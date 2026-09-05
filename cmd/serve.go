package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/pkg/camera"
	"github.com/ghchinoy/homectl/pkg/cast"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the homectl API server",
	Long:  `Starts an HTTP server that provides an API for controlling Lutron and Sonos devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")

		client, err := getClient(cmd)
		if err != nil {
			return fmt.Errorf("lutron client: %w", err)
		}
		defer client.Close()

		http.HandleFunc("/api/lutron/devices", func(w http.ResponseWriter, r *http.Request) {
			devices, err := client.GetDevices()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			nicknames := config.LoadNicknames()
			for i, d := range devices {
				if len(d.LocalZones) > 0 {
					if nick, ok := nicknames[d.LocalZones[0].Href]; ok {
						devices[i].Nickname = nick
					}
				}
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

		http.HandleFunc("/api/debug/lutron", func(w http.ResponseWriter, r *http.Request) {
			statuses, err := client.GetAllZoneStatuses()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statuses)
		})

		http.HandleFunc("/api/lutron/set", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Href  string  `json:"href"`
				Level float64 `json:"level"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := client.SetLevel(req.Href, req.Level); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		http.HandleFunc("/api/lutron/all", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Level float64 `json:"level"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := client.SetAllLevels(req.Level); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		http.HandleFunc("/api/sonos/devices", func(w http.ResponseWriter, r *http.Request) {
			devices, err := sonos.LoadCache()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Add nicknames to Sonos devices
			nicknames := config.LoadNicknames()
			type NicknamedSonos struct {
				sonos.Device
				Nickname string `json:"Nickname,omitempty"`
			}
			var results []NicknamedSonos
			for _, d := range devices {
				nick := ""
				if n, ok := nicknames[d.IP]; ok {
					nick = n
				}
				results = append(results, NicknamedSonos{
					Device:   d,
					Nickname: nick,
				})
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		})

		http.HandleFunc("/api/sonos/status", func(w http.ResponseWriter, r *http.Request) {
			devices, _ := sonos.LoadCache()
			results := make(map[string]interface{})
			for _, d := range devices {
				client := sonos.NewClient(d.IP)
				transport, _ := client.GetTransportInfo()
				pos, _ := client.GetPositionInfo()
				meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
				vol, _ := client.GetVolume()

				results[d.IP] = map[string]interface{}{
					"status":    transport.CurrentTransportState,
					"volume":    vol,
					"title":     meta.Title,
					"artist":    meta.Artist,
					"album":     meta.Album,
					"album_art": meta.AlbumArtURI,
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		})

		http.HandleFunc("/api/sonos/control", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP     string `json:"ip"`
				Action string `json:"action"`
				Volume int    `json:"volume"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(req.IP)
			var err error
			switch req.Action {
			case "play":
				err = client.Play()
			case "pause":
				err = client.Pause()
			case "next":
				err = client.Next()
			case "prev":
				err = client.Previous()
			case "volume":
				err = client.SetVolume(req.Volume)
			default:
				http.Error(w, "Unknown action", http.StatusBadRequest)
				return
			}

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		http.HandleFunc("/api/security/cameras", func(w http.ResponseWriter, r *http.Request) {
			manager := discovery.NewManager()
			manager.AddProvider(&camera.DiscoveryProvider{})
			devices := manager.DiscoverAll(2 * time.Second)

			nicknames := config.LoadNicknames()
			var results []map[string]string
			for _, d := range devices {
				name := d.Name
				if nick, ok := nicknames[d.IP]; ok {
					name = nick
				}
				results = append(results, map[string]string{
					"name": name,
					"ip":   d.IP,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		})

		http.HandleFunc("/api/security/stream", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				http.Error(w, "ip is required", http.StatusBadRequest)
				return
			}

			cfg := config.LoadConfig()
			rtspURL := fmt.Sprintf("rtsp://%s:554", ip)
			if cfg.CameraAuth != "" {
				rtspURL = fmt.Sprintf("rtsp://%s@%s:554", cfg.CameraAuth, ip)
			}

			// Use context to kill ffmpeg when the browser disconnects
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()

			// ffmpeg -rtsp_transport tcp -i rtsp://... -f mpjpeg -q:v 3 -vcodec mjpeg pipe:1
			cmd := exec.CommandContext(ctx, "ffmpeg",
				"-rtsp_transport", "tcp",
				"-i", rtspURL,
				"-f", "mpjpeg",
				"-q:v", "3",
				"-vcodec", "mjpeg",
				"pipe:1",
			)

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Capture stderr for debugging
			cmd.Stderr = os.Stderr

			if err := cmd.Start(); err != nil {
				fmt.Printf("FFmpeg Start Error: %v\n", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=ffserver")
			io.Copy(w, stdout)
			cmd.Wait()
		})

		http.HandleFunc("/api/sonos/art", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			path := html.UnescapeString(r.URL.Query().Get("path"))
			if ip == "" || path == "" {
				http.Error(w, "ip and path are required", http.StatusBadRequest)
				return
			}

			parsedIP := net.ParseIP(ip)
			if parsedIP == nil || parsedIP.To4() == nil {
				http.Error(w, "invalid ip address", http.StatusBadRequest)
				return
			}

			targetURL := path
			if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				targetURL = fmt.Sprintf("http://%s:1400%s", ip, path)
			} else {
				parsed, err := url.Parse(path)
				if err != nil {
					http.Error(w, "invalid art url", http.StatusBadRequest)
					return
				}
				hostOnly := parsed.Hostname()
				if hostOnly == "localhost" || hostOnly == "127.0.0.1" || strings.HasPrefix(hostOnly, "169.254.") {
					http.Error(w, "access to internal host is blocked", http.StatusForbidden)
					return
				}
				targetIP := net.ParseIP(hostOnly)
				if targetIP != nil && (targetIP.IsLoopback() || targetIP.IsLinkLocalUnicast()) {
					http.Error(w, "access to internal host is blocked", http.StatusForbidden)
					return
				}
				if targetIP != nil && targetIP.String() != parsedIP.String() {
					http.Error(w, "art url host does not match speaker", http.StatusForbidden)
					return
				}
			}

			artClient := &http.Client{Timeout: 5 * time.Second}
			resp, err := artClient.Get(targetURL)
			if err != nil {
				fmt.Printf("Art Proxy Error: %v\n", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Art Proxy Remote Status: %d\n", resp.StatusCode)
			}

			for k, v := range resp.Header {
				for _, val := range v {
					w.Header().Add(k, val)
				}
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
		})

		http.HandleFunc("/api/cast/devices", func(w http.ResponseWriter, r *http.Request) {
			manager := discovery.NewManager()
			manager.AddProvider(&cast.DiscoveryProvider{})
			devices := manager.DiscoverAll(2 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devices)
		})

		http.HandleFunc("/api/cast/status", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				http.Error(w, "ip is required", http.StatusBadRequest)
				return
			}
			status, err := cast.GetStatus(ip)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(status)
		})

		http.HandleFunc("/api/cast/control", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP     string  `json:"ip"`
				Action string  `json:"action"`
				Volume float64 `json:"volume"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Add control logic to pkg/cast/client.go and call it here
			if err := cast.Control(req.IP, req.Action, req.Volume); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		uiDir, _ := cmd.Flags().GetString("ui")
		http.Handle("/", http.FileServer(http.Dir(uiDir)))

		fmt.Printf("Starting homectl API server on :%d (serving UI from %s)\n", port, uiDir)
		return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("ui", "./ui/dist", "Directory to serve the UI from")
}
