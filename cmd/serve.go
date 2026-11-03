package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/camera"
	"github.com/ghchinoy/homectl/pkg/cast"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/spf13/cobra"
)

// isAllowedLocalOrigin verifies that browser-initiated requests originate from
// the same host or a trusted localhost/LAN origin (W3C User Agents § 3.1 P4).
func isAllowedLocalOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		secSite := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))
		return secSite == "" || secSite == "same-origin" || secSite == "same-site" || secSite == "none"
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	hostOnly := u.Hostname()
	return hostOnly == "localhost" || hostOnly == "127.0.0.1" || hostOnly == "::1"
}

func requireLocalOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAllowedLocalOrigin(r) {
			http.Error(w, "cross-origin request blocked", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// resolveArtTargetURL validates speaker IP and album art path/URL, preventing
// SSRF against internal loopback/link-local addresses or arbitrary external domains.
func resolveArtTargetURL(ip, rawPath string) (string, int, error) {
	path := html.UnescapeString(rawPath)
	if ip == "" || path == "" {
		return "", http.StatusBadRequest, fmt.Errorf("ip and path are required")
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil || parsedIP.To4() == nil {
		return "", http.StatusBadRequest, fmt.Errorf("invalid ip address")
	}

	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return fmt.Sprintf("http://%s:1400%s", parsedIP.String(), path), http.StatusOK, nil
	}

	parsed, err := url.Parse(path)
	if err != nil {
		return "", http.StatusBadRequest, fmt.Errorf("invalid art url")
	}
	hostOnly := parsed.Hostname()
	if hostOnly == "localhost" || hostOnly == "127.0.0.1" || strings.HasPrefix(hostOnly, "169.254.") {
		return "", http.StatusForbidden, fmt.Errorf("access to internal host is blocked")
	}
	targetIP := net.ParseIP(hostOnly)
	if targetIP == nil {
		return "", http.StatusForbidden, fmt.Errorf("art url host must be the speaker ip")
	}
	if targetIP.IsLoopback() || targetIP.IsLinkLocalUnicast() || targetIP.IsUnspecified() {
		return "", http.StatusForbidden, fmt.Errorf("access to internal host is blocked")
	}
	if !targetIP.Equal(parsedIP) {
		return "", http.StatusForbidden, fmt.Errorf("art url host does not match speaker")
	}
	return path, http.StatusOK, nil
}

var serveCmd = &cobra.Command{
	Use:     "serve",
	GroupID: "services",
	Short:   "Start the homectl API server",
	Long:    `Starts an HTTP server that provides an API for controlling Lutron and Sonos devices.`,
	Example: `  # Start the server bound to localhost
  homectl serve -H 127.0.0.1

  # Start the server on a custom port with a custom UI directory
  homectl serve --port 9090 --ui /usr/local/share/homectl/ui`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")

		client, err := getClient(cmd)
		if err != nil {
			return fmt.Errorf("lutron client: %w", err)
		}
		defer client.Close()

		mux := http.NewServeMux()

		mux.HandleFunc("/api/lutron/devices", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/lutron/status", func(w http.ResponseWriter, r *http.Request) {
			statuses, err := client.GetAllZoneStatuses()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statuses)
		})

		mux.HandleFunc("/api/debug/lutron", func(w http.ResponseWriter, r *http.Request) {
			statuses, err := client.GetAllZoneStatuses()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statuses)
		})

		mux.HandleFunc("/api/lutron/set", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/lutron/all", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/sonos/devices", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/sonos/status", func(w http.ResponseWriter, r *http.Request) {
			devices, _ := sonos.LoadCache()
			results := make(map[string]any)
			for _, d := range devices {
				client := sonos.NewClient(d.IP)
				transport, _ := client.GetTransportInfo()
				pos, _ := client.GetPositionInfo()
				meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)
				vol, _ := client.GetVolume()

				results[d.IP] = map[string]any{
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

		mux.HandleFunc("/api/sonos/control", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP     string `json:"ip"`
				Action string `json:"action"`
				Volume int    `json:"volume"`
				Track  int    `json:"track"`
				Target string `json:"target"`
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
			case "seek_track":
				err = client.SeekTrack(req.Track)
			case "seek_time":
				err = client.SeekTime(req.Target)
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

		mux.HandleFunc("/api/sonos/favorites", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				devices, _ := sonos.LoadCache()
				if len(devices) > 0 {
					ip = devices[0].IP
				}
			}
			if ip == "" {
				http.Error(w, "ip parameter required (no speakers in cache)", http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(ip)
			favs, err := client.BrowseFavorites()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"count":     len(favs),
				"favorites": favs,
			})
		})

		mux.HandleFunc("/api/sonos/play-favorite", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP         string `json:"ip"`
				FavoriteID string `json:"favorite_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.IP == "" || req.FavoriteID == "" {
				http.Error(w, "ip and favorite_id are required", http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(req.IP)
			if err := client.PlayFavorite(req.FavoriteID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status":      "ok",
				"action":      "play_favorite",
				"ip":          req.IP,
				"favorite_id": req.FavoriteID,
			})
		})

		mux.HandleFunc("/api/sonos/services", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				devices, _ := sonos.LoadCache()
				if len(devices) > 0 {
					ip = devices[0].IP
				}
			}
			if ip == "" {
				http.Error(w, "ip parameter required (no speakers in cache)", http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(ip)
			services, err := client.ListMusicServices()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			cfg := config.LoadConfig()
			defSvc, hasDef := sonos.ResolveDefaultService(services, cfg.SonosDefaultService)
			var defPtr *sonos.MusicService
			if hasDef {
				defPtr = &defSvc
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"count":    len(services),
				"default":  defPtr,
				"services": services,
			})
		})

		mux.HandleFunc("/api/sonos/play-stream", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP    string `json:"ip"`
				URL   string `json:"url"`
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.IP == "" || req.URL == "" {
				http.Error(w, "ip and url are required", http.StatusBadRequest)
				return
			}

			u, err := url.Parse(strings.TrimSpace(req.URL))
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(w, "invalid stream url: scheme must be http or https", http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(req.IP)
			if err := client.PlayStream(req.URL, req.Title); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"action": "play_stream",
				"ip":     req.IP,
				"url":    req.URL,
				"title":  req.Title,
			})
		})

		mux.HandleFunc("/api/sonos/queue-add", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				IP       string `json:"ip"`
				URI      string `json:"uri"`
				Metadata string `json:"metadata"`
				AsNext   bool   `json:"as_next"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.IP == "" || req.URI == "" {
				http.Error(w, "ip and uri are required", http.StatusBadRequest)
				return
			}

			client := sonos.NewClient(req.IP)
			pos, err := client.AddURIToQueue(req.URI, req.Metadata, req.AsNext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status":         "ok",
				"action":         "queue_add",
				"ip":             req.IP,
				"uri":            req.URI,
				"track_position": pos,
				"as_next":        req.AsNext,
			})
		})

		mux.HandleFunc("/api/sonos/queue", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				devices, _ := sonos.LoadCache()
				if len(devices) > 0 {
					ip = devices[0].IP
				}
			}
			if ip == "" {
				http.Error(w, "ip parameter required (no speakers in cache)", http.StatusBadRequest)
				return
			}

			start := 0
			if sStr := r.URL.Query().Get("start"); sStr != "" {
				if s, err := strconv.Atoi(sStr); err == nil && s >= 0 {
					start = s
				}
			}
			count := 20
			if cStr := r.URL.Query().Get("count"); cStr != "" {
				if c, err := strconv.Atoi(cStr); err == nil && c > 0 {
					count = c
				}
			}

			client := sonos.NewClient(ip)
			qRes, err := client.GetQueue(start, count)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(qRes)
		})

		mux.HandleFunc("/api/security/cameras", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/security/stream", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			if ip == "" {
				http.Error(w, "ip is required", http.StatusBadRequest)
				return
			}
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil || parsedIP.To4() == nil {
				http.Error(w, "invalid ip address", http.StatusBadRequest)
				return
			}

			cfg := config.LoadConfig()
			rtspURL := fmt.Sprintf("rtsp://%s:554", parsedIP.String())
			if cfg.CameraAuth != "" {
				rtspURL = fmt.Sprintf("rtsp://%s@%s:554", cfg.CameraAuth, parsedIP.String())
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

		mux.HandleFunc("/api/sonos/art", func(w http.ResponseWriter, r *http.Request) {
			ip := r.URL.Query().Get("ip")
			rawPath := r.URL.Query().Get("path")
			targetURL, status, err := resolveArtTargetURL(ip, rawPath)
			if err != nil {
				http.Error(w, err.Error(), status)
				return
			}

			artClient := &http.Client{Timeout: 5 * time.Second}
			artReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
			if err != nil {
				http.Error(w, "invalid art url", http.StatusBadRequest)
				return
			}
			artReq.Header.Set("User-Agent", "homectl/1.0")
			resp, err := artClient.Do(artReq)
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

		mux.HandleFunc("/api/cast/devices", func(w http.ResponseWriter, r *http.Request) {
			manager := discovery.NewManager()
			manager.AddProvider(&cast.DiscoveryProvider{})
			devices := manager.DiscoverAll(2 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(devices)
		})

		mux.HandleFunc("/api/cast/status", func(w http.ResponseWriter, r *http.Request) {
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

		mux.HandleFunc("/api/cast/control", requireLocalOrigin(func(w http.ResponseWriter, r *http.Request) {
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
		}))

		uiDir, _ := cmd.Flags().GetString("ui")
		resolvedUIDir := filepath.Clean(uiDir)
		if evalDir, err := filepath.EvalSymlinks(resolvedUIDir); err == nil {
			resolvedUIDir = evalDir
		}
		mux.Handle("/", http.FileServer(http.Dir(resolvedUIDir)))

		flagHost, _ := cmd.Flags().GetString("host")
		host := ResolveAPIHost(flagHost)
		addr := net.JoinHostPort(host, strconv.Itoa(port))

		server := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		errCh := make(chan error, 1)
		go func() {
			fmt.Printf("Starting homectl API server on %s (serving UI from %s)\n", addr, resolvedUIDir)
			if host == "0.0.0.0" || host == "" {
				fmt.Println("[SECURITY ADVISORY] API server bound to 0.0.0.0 (all network interfaces) without authentication.")
				fmt.Println("                   To restrict access, bind to loopback (-H 127.0.0.1) or your Tailscale IP (-H 100.x.y.z),")
				fmt.Println("                   or set \"api_host\" in ~/.config/homectl/config.json.")
			} else if IsTailscaleIP(host) {
				fmt.Printf("[SECURITY] Bound to Tailscale interface (%s) for secure off-LAN access.\n", host)
			} else if IsLoopbackHost(host) {
				fmt.Printf("[SECURITY] Bound to local loopback interface (%s). External network access is blocked.\n", host)
			}

			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
			close(errCh)
		}()

		select {
		case <-cmd.Context().Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(cmd.Context()), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case err := <-errCh:
			return err
		}
	},
}

var (
	tailscaleIPv4CIDR = mustParseCIDR("100.64.0.0/10")
	tailscaleIPv6CIDR = mustParseCIDR("fd7a:115c:a1e0::/48")
)

func mustParseCIDR(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR %q: %v", cidr, err))
	}
	return ipnet
}

// IsTailscaleIP checks if an IP string belongs to Tailscale IPv4 (100.64.0.0/10) or IPv6 (fd7a:115c:a1e0::/48) ranges.
func IsTailscaleIP(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	return tailscaleIPv4CIDR.Contains(ip) || tailscaleIPv6CIDR.Contains(ip)
}

// IsLoopbackHost checks if the host string refers to a local loopback interface (localhost, 127.0.0.1, ::1).
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// ResolveAPIHost resolves the host address to bind the API server to according to strict precedence:
// 1. Explicit CLI flag (--host / -H)
// 2. HOMECTL_API_HOST environment variable
// 3. config.json "api_host" setting
// 4. Default fallback: "0.0.0.0"
func ResolveAPIHost(flagHost string) string {
	if trimmed := strings.TrimSpace(flagHost); trimmed != "" {
		return trimmed
	}
	if envHost := strings.TrimSpace(os.Getenv("HOMECTL_API_HOST")); envHost != "" {
		return envHost
	}
	cfg := config.LoadConfig()
	if cfgHost := strings.TrimSpace(cfg.APIHost); cfgHost != "" {
		return cfgHost
	}
	return "0.0.0.0"
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringP("host", "H", "", "Host/IP address to bind the server to (default: 0.0.0.0; supports 127.0.0.1, Tailscale 100.x.y.z)")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("ui", "./ui/dist", "Directory to serve the UI from")
}
