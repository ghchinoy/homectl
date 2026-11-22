package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/roborock"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/spf13/cobra"
)

const defaultRoborockControlURL = "http://127.0.0.1:8889"

var roborockCmd = &cobra.Command{
	Use:     "roborock",
	GroupID: "devices",
	Short:   "Run local Roborock server, onboard vacuums, monitor telemetry, and control fleet",
	Long:    `Native Go local Roborock cloud emulator (HTTPS + embedded Mochi MQTT TLS broker + embedded Pion TURN relay) for Q5 Max+ (a125), Saros 10R (a144), and Saros 20 (a288) vacuums.`,
}

func roborockStorageDir() string {
	return filepath.Join(config.ConfigDir(), "roborock")
}

func roborockControlBaseURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("control-url"); strings.TrimSpace(v) != "" {
		return strings.TrimRight(strings.TrimSpace(v), "/")
	}
	if v := strings.TrimSpace(os.Getenv("HOMECTL_ROBOROCK_CONTROL_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	cfg := config.LoadConfig()
	if cfg.Roborock != nil && strings.TrimSpace(cfg.Roborock.ControlAPIURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.Roborock.ControlAPIURL), "/")
	}
	return defaultRoborockControlURL
}

// buildRoborockControlMux builds the local loopback/LAN REST + SSE control API exposed by `homectl roborock serve`.
func buildRoborockControlMux(stack *roborock.Stack, stackTokenR, countryDomain string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/roborock/devices", func(w http.ResponseWriter, r *http.Request) {
		devs := stack.Telemetry.Snapshot(stack.Creds, stack.Keys)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count":   len(devs),
			"devices": devs,
		})
	})

	mux.HandleFunc("/api/roborock/telemetry", func(w http.ResponseWriter, r *http.Request) {
		did := strings.TrimSpace(r.URL.Query().Get("id"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 100
		}
		events := stack.Telemetry.RecentEvents(did, limit)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count":  len(events),
			"events": events,
		})
	})

	mux.HandleFunc("/api/roborock/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		filterID := strings.TrimSpace(r.URL.Query().Get("id"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch, cancel := stack.Telemetry.Subscribe()
		defer cancel()

		// Replay last 10 events on connect so monitors have immediate context.
		for _, ev := range stack.Telemetry.RecentEvents(filterID, 10) {
			raw, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		}
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case ev, open := <-ch:
				if !open {
					return
				}
				if filterID != "" && ev.DID != filterID && ev.DUID != filterID {
					continue
				}
				raw, _ := json.Marshal(ev)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/api/roborock/status", func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("id"))
		if target == "" {
			http.Error(w, "id query parameter is required", http.StatusBadRequest)
			return
		}
		rpcCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		statusRes, err := stack.Controller.GetStatus(rpcCtx, target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		consRes, _ := stack.Controller.GetConsumables(rpcCtx, target)
		out := map[string]any{
			"id":     target,
			"status": statusRes.Result,
		}
		if consRes != nil {
			out["consumables"] = consRes.Result
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/api/roborock/rooms", func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("id"))
		if target == "" {
			http.Error(w, "id query parameter is required", http.StatusBadRequest)
			return
		}
		rpcCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		res, err := stack.Controller.GetRoomMapping(rpcCtx, target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": target, "rooms": res.Result})
	})

	mux.HandleFunc("/api/roborock/camera", func(w http.ResponseWriter, r *http.Request) {
		target := strings.TrimSpace(r.URL.Query().Get("id"))
		var infos []roborock.CameraInfo
		for _, dev := range stack.Creds.ListDevices() {
			if target != "" && !strings.EqualFold(dev.DID, target) && !strings.EqualFold(dev.DUID, target) &&
				!strings.EqualFold(dev.Name, target) && !strings.HasSuffix(strings.ToLower(dev.Model), "."+strings.ToLower(target)) {
				continue
			}
			infos = append(infos, roborock.BuildCameraInfo(dev, stack.TURN, stackTokenR, countryDomain))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"count": len(infos), "cameras": infos})
	})

	mux.HandleFunc("/api/roborock/control", requireLocalOrigin(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID        string `json:"id"`
			Action    string `json:"action"`
			Rooms     []int  `json:"rooms,omitempty"`
			Repeat    int    `json:"repeat,omitempty"`
			FanPower  int    `json:"fan_power,omitempty"`
			WaterMode int    `json:"water_mode,omitempty"`
			DryRun    bool   `json:"dry_run,omitempty"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Action) == "" {
			http.Error(w, "id and action are required", http.StatusBadRequest)
			return
		}
		if req.DryRun {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"dry_run": true, "id": req.ID, "action": req.Action})
			return
		}
		rpcCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var (
			res *roborock.RPCResponse
			err error
		)
		switch strings.ToLower(strings.TrimSpace(req.Action)) {
		case "start":
			res, err = stack.Controller.StartClean(rpcCtx, req.ID)
		case "pause":
			res, err = stack.Controller.PauseClean(rpcCtx, req.ID)
		case "stop":
			res, err = stack.Controller.StopClean(rpcCtx, req.ID)
		case "dock":
			res, err = stack.Controller.ReturnToDock(rpcCtx, req.ID)
		case "find":
			res, err = stack.Controller.FindMe(rpcCtx, req.ID)
		case "clean_rooms":
			if len(req.Rooms) == 0 {
				http.Error(w, "rooms array is required for clean_rooms", http.StatusBadRequest)
				return
			}
			res, err = stack.Controller.CleanRooms(rpcCtx, req.ID, req.Rooms, req.Repeat)
		case "fan_power":
			res, err = stack.Controller.SetFanPower(rpcCtx, req.ID, req.FanPower)
		case "water_mode":
			res, err = stack.Controller.SetWaterMode(rpcCtx, req.ID, req.WaterMode)
		case "request_map":
			res, err = stack.Controller.RequestMap(rpcCtx, req.ID)
		default:
			http.Error(w, fmt.Sprintf("unsupported action %q", req.Action), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": req.ID, "action": req.Action, "result": res.Result})
	}))

	return mux
}

var roborockServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local Roborock HTTPS + embedded MQTT TLS + optional TURN server",
	Example: `  # Run on port 443 with hot-reloaded wildcard cert and embedded TURN
  homectl roborock serve --fqdn api-r.example.com --https-port 443 --cert fullchain.pem --key privkey.pem --turn-mode embedded --turn-host r.example.com --turn-user rrturn --turn-pass secret`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCfg := config.LoadConfig()
		rrCfg := config.RoborockConfig{
			BindHost:    "0.0.0.0",
			HTTPSPort:   443,
			MQTTTLSPort: 8881,
			Region:      "us",
			Timezone:    "America/Denver",
		}
		if appCfg.Roborock != nil {
			rrCfg = *appCfg.Roborock
			if rrCfg.BindHost == "" {
				rrCfg.BindHost = "0.0.0.0"
			}
			if rrCfg.HTTPSPort <= 0 {
				rrCfg.HTTPSPort = 443
			}
			if rrCfg.MQTTTLSPort <= 0 {
				rrCfg.MQTTTLSPort = 8881
			}
			if rrCfg.Region == "" {
				rrCfg.Region = "us"
			}
		}

		if v, _ := cmd.Flags().GetString("fqdn"); v != "" {
			rrCfg.StackFQDN = v
		}
		if v, _ := cmd.Flags().GetString("bind"); v != "" {
			rrCfg.BindHost = v
		}
		if cmd.Flags().Changed("https-port") {
			rrCfg.HTTPSPort, _ = cmd.Flags().GetInt("https-port")
		}
		if cmd.Flags().Changed("advertised-port") {
			rrCfg.AdvertisedPort, _ = cmd.Flags().GetInt("advertised-port")
		}
		if cmd.Flags().Changed("mqtt-port") {
			rrCfg.MQTTTLSPort, _ = cmd.Flags().GetInt("mqtt-port")
		}
		if v, _ := cmd.Flags().GetString("region"); v != "" {
			rrCfg.Region = v
		}
		if v, _ := cmd.Flags().GetString("timezone"); v != "" {
			rrCfg.Timezone = v
		}
		if cmd.Flags().Changed("legacy-tls") {
			rrCfg.AllowLegacyTLS, _ = cmd.Flags().GetBool("legacy-tls")
		}
		if cmd.Flags().Changed("trace") {
			rrCfg.TraceJSONL, _ = cmd.Flags().GetBool("trace")
		}
		if v, _ := cmd.Flags().GetString("camera-domain"); v != "" {
			rrCfg.CameraDomain = v
		}

		certsMap := make(map[string]roborock.CertPair)
		for host, pair := range rrCfg.CertsBySNI {
			certsMap[host] = roborock.CertPair{CertFile: pair.CertFile, KeyFile: pair.KeyFile}
		}
		certFile, _ := cmd.Flags().GetString("cert")
		keyFile, _ := cmd.Flags().GetString("key")
		if certFile != "" && keyFile != "" {
			certsMap["*"] = roborock.CertPair{CertFile: certFile, KeyFile: keyFile}
		}
		sniSpecs, _ := cmd.Flags().GetStringSlice("sni-cert")
		for _, spec := range sniSpecs {
			parts := strings.Split(spec, "=")
			if len(parts) == 2 {
				files := strings.Split(parts[1], ":")
				if len(files) == 2 {
					certsMap[parts[0]] = roborock.CertPair{CertFile: files[0], KeyFile: files[1]}
				}
			}
		}

		noTLS, _ := cmd.Flags().GetBool("no-tls")
		var certMgr *roborock.MultiSNICertManager
		if !noTLS {
			var err error
			certMgr, err = roborock.NewMultiSNICertManager(certsMap)
			if err != nil {
				return fmt.Errorf("TLS setup failed (pass --cert/--key or configure roborock.certs_by_sni in config.json): %w", err)
			}
		}

		if rrCfg.StackFQDN == "" {
			return fmt.Errorf("--fqdn (or roborock.stack_fqdn in config.json) is required")
		}

		turnCfg := roborock.TURNConfig{
			Mode:     roborock.TURNMode(rrCfg.TURN.Mode),
			BindHost: rrCfg.TURN.BindHost,
			Host:     rrCfg.TURN.Host,
			PublicIP: rrCfg.TURN.PublicIP,
			Port:     rrCfg.TURN.Port,
			Username: rrCfg.TURN.Username,
			Password: rrCfg.TURN.Password,
			Realm:    rrCfg.TURN.Realm,
		}
		if v, _ := cmd.Flags().GetString("turn-mode"); v != "" {
			turnCfg.Mode = roborock.TURNMode(v)
		}
		if v, _ := cmd.Flags().GetString("turn-host"); v != "" {
			turnCfg.Host = v
		}
		if cmd.Flags().Changed("turn-port") {
			turnCfg.Port, _ = cmd.Flags().GetInt("turn-port")
		}
		if v, _ := cmd.Flags().GetString("turn-user"); v != "" {
			turnCfg.Username = v
		}
		if v, _ := cmd.Flags().GetString("turn-pass"); v != "" {
			turnCfg.Password = v
		}

		stdLog := core.NewStdLogger(log.New(os.Stdout, "", log.LstdFlags))
		stack, err := roborock.NewStack(roborock.StackConfig{
			Storage:       core.NewDirStorage(roborockStorageDir()),
			Logger:        stdLog,
			KeyRecoverCmd: rrCfg.KeyRecoverCmd,
			Certs:         certMgr,
			HTTP: roborock.HTTPServerConfig{
				BindHost:           rrCfg.BindHost,
				Port:               rrCfg.HTTPSPort,
				AdvertisedHost:     rrCfg.StackFQDN,
				AdvertisedPort:     rrCfg.AdvertisedPort,
				AdvertisedMQTT:     rrCfg.StackFQDN,
				AdvertisedMQTTPort: rrCfg.MQTTTLSPort,
				Region:             rrCfg.Region,
				Timezone:           rrCfg.Timezone,
				AllowLegacyTLS:     rrCfg.AllowLegacyTLS,
				DisableTLS:         noTLS,
			},
			MQTT: roborock.MQTTBrokerConfig{
				BindHost:              rrCfg.BindHost,
				Port:                  rrCfg.MQTTTLSPort,
				AllowLegacyTLS:        rrCfg.AllowLegacyTLS,
				DisableTLS:            noTLS,
				NewConnectionsEnabled: true,
			},
			TURN:                  turnCfg,
			TraceJSONL:            rrCfg.TraceJSONL,
			NewConnectionsEnabled: true,
		})
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := stack.Start(ctx); err != nil {
			return err
		}
		defer func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = stack.Close(shutCtx)
		}()

		// Expose local control API for homectl CLI, Web UI, and mcp-roborock.
		controlBind, _ := cmd.Flags().GetString("control-bind")
		tokenR, _ := roborock.SanitizeStackServer(rrCfg.StackFQDN, rrCfg.HTTPSPort)
		camDomain := rrCfg.CameraDomain
		if camDomain != "" && !strings.HasSuffix(camDomain, "/") {
			camDomain += "/"
		}
		ctrlMux := buildRoborockControlMux(stack, tokenR, camDomain)
		ctrlSrv := &http.Server{
			Addr:              controlBind,
			Handler:           ctrlMux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		go func() {
			stdLog.Printf("roborock: local control/SSE API listening on http://%s", controlBind)
			if err := ctrlSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				stdLog.Printf("roborock: control API error: %v", err)
			}
		}()
		defer func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = ctrlSrv.Shutdown(shutCtx)
		}()

		<-ctx.Done()
		stdLog.Println("roborock: shutting down local server...")
		return nil
	},
}

var roborockOnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Run TLS/DNS/camera preflight or send Wi-Fi AP onboarding packet to a Roborock vacuum",
	Example: `  # Verify DNS, TLS certs, MQTT listener, and 15-char camera bootstrap before touching a vacuum
  homectl roborock onboard --server api-r.example.com --camera-domain r.example.com --model a144 --preflight-only

  # Send Wi-Fi credentials while connected to the vacuum's roborock-vacuum-* hotspot
  homectl roborock onboard --server api-r.example.com --ssid "MyWiFi" --password "Secret" --timezone America/Denver --camera-domain r.example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverArg, _ := cmd.Flags().GetString("server")
		if serverArg == "" {
			if cfg := config.LoadConfig(); cfg.Roborock != nil {
				serverArg = cfg.Roborock.StackFQDN
			}
		}
		if serverArg == "" {
			return fmt.Errorf("--server is required (e.g. api-r.example.com)")
		}
		httpsPort, _ := cmd.Flags().GetInt("https-port")
		model, _ := cmd.Flags().GetString("model")
		camDomain, _ := cmd.Flags().GetString("camera-domain")
		preflightOnly, _ := cmd.Flags().GetBool("preflight-only")
		insecure, _ := cmd.Flags().GetBool("insecure")

		if preflightOnly {
			res, err := roborock.Preflight(cmd.Context(), serverArg, model, camDomain, httpsPort, insecure)
			if err != nil {
				return err
			}
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			fmt.Printf("API Base: %s\nToken.r:  %s\nMQTT:     %s\n", res.APIBaseURL, res.TokenR, res.MQTTAddr)
			for _, c := range res.Checks {
				fmt.Printf("  [OK]   %s\n", c)
			}
			for _, e := range res.Errors {
				fmt.Printf("  [FAIL] %s\n", e)
			}
			if !res.OK {
				return fmt.Errorf("preflight failed with %d error(s)", len(res.Errors))
			}
			return nil
		}

		ssid, _ := cmd.Flags().GetString("ssid")
		wifiPass, _ := cmd.Flags().GetString("password")
		tz, _ := cmd.Flags().GetString("timezone")
		cst, _ := cmd.Flags().GetString("cst")
		countryDomain, _ := cmd.Flags().GetString("country-domain")
		if camDomain != "" {
			countryDomain = strings.TrimRight(camDomain, "/") + "/"
		}
		tokenR, err := roborock.SanitizeStackServer(serverArg, httpsPort)
		if err != nil {
			return err
		}
		apHost, _ := cmd.Flags().GetString("ap-host")
		apPort, _ := cmd.Flags().GetInt("ap-port")

		if isDryRun(cmd) {
			out := map[string]any{
				"dry_run":        true,
				"ap":             fmt.Sprintf("%s:%d", apHost, apPort),
				"ssid":           ssid,
				"token_r":        tokenR,
				"timezone":       tz,
				"country_domain": countryDomain,
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		if err := roborock.SendCfgWiFi(cmd.Context(), roborock.CfgWiFiOptions{
			APHost:        apHost,
			APPort:        apPort,
			SSID:          ssid,
			Password:      wifiPass,
			TokenR:        tokenR,
			Timezone:      tz,
			CST:           cst,
			CountryDomain: countryDomain,
		}); err != nil {
			return err
		}
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "token_r": tokenR})
		}
		fmt.Printf("Onboarding packet sent (token.r=%s). Reconnect to your LAN Wi-Fi and watch 'homectl roborock list' / 'homectl roborock monitor'.\n", tokenR)
		return nil
	},
}

var roborockListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Roborock vacuums with key-recovery state, connection status, and live telemetry",
	RunE: func(cmd *cobra.Command, args []string) error {
		var resp struct {
			Count   int                        `json:"count"`
			Devices []roborock.VacuumTelemetry `json:"devices"`
		}
		base := roborockControlBaseURL(cmd)
		if err := getRoborockControlJSON(cmd.Context(), base+"/api/roborock/devices", &resp); err != nil {
			// Fallback to offline XDG store snapshot if daemon isn't running.
			storage := core.NewDirStorage(roborockStorageDir())
			ks, _ := roborock.NewKeyStore(storage, nil, nil)
			cs, _ := roborock.NewCredentialStore(storage)
			hub := roborock.NewTelemetryHub(10, "")
			resp.Devices = hub.Snapshot(cs, ks)
			resp.Count = len(resp.Devices)
		}
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(resp)
		}
		if len(resp.Devices) == 0 {
			fmt.Println("No Roborock vacuums registered yet. Run 'homectl roborock serve' and 'homectl roborock onboard'.")
			return nil
		}
		fmt.Printf("%-20s %-18s %-10s %-10s %-8s %-12s %s\n", "NAME", "MODEL", "DID", "KEY", "ONLINE", "BATTERY", "STATE")
		fmt.Println(strings.Repeat("-", 96))
		for _, d := range resp.Devices {
			bat := "—"
			if d.Battery != nil {
				bat = fmt.Sprintf("%d%%", *d.Battery)
			}
			state := d.StateName
			if state == "" {
				state = "—"
			}
			keyState := d.KeyRecoveryState
			if keyState == "" {
				keyState = "missing"
			}
			fmt.Printf("%-20s %-18s %-10s %-10s %-8v %-12s %s\n", d.Name, d.Model, d.DID, keyState, d.Connected, bat, state)
		}
		return nil
	},
}

var roborockStatusCmd = &cobra.Command{
	Use:   "status <device>",
	Short: "Query live RPC status and consumables from a connected vacuum",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base := roborockControlBaseURL(cmd)
		u := fmt.Sprintf("%s/api/roborock/status?id=%s", base, url.QueryEscape(args[0]))
		var out map[string]any
		if err := getRoborockControlJSON(cmd.Context(), u, &out); err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

var roborockMonitorCmd = &cobra.Command{
	Use:   "monitor [device]",
	Short: "Stream live decoded MQTT telemetry, RPC traffic, and onboarding events",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base := roborockControlBaseURL(cmd)
		u := base + "/api/roborock/events"
		if len(args) == 1 {
			u += "?id=" + url.QueryEscape(args[0])
		}
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return fmt.Errorf("connect to roborock monitor stream (%s): %w", u, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return fmt.Errorf("monitor stream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		jsonOut := isJSON(cmd)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			raw := strings.TrimPrefix(line, "data: ")
			if jsonOut {
				fmt.Println(raw)
				continue
			}
			var ev roborock.TelemetryEvent
			if json.Unmarshal([]byte(raw), &ev) == nil {
				fmt.Printf("[%s] %-14s did=%-10s %s\n", ev.Timestamp, ev.Kind, ev.DID, ev.Summary)
			}
		}
		return scanner.Err()
	},
}

func makeRoborockActionCmd(use, short, action string) *cobra.Command {
	c := &cobra.Command{
		Use:   use + " <device>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{
				"id":      args[0],
				"action":  action,
				"dry_run": isDryRun(cmd),
			}
			if action == "clean_rooms" || action == "start" {
				if rooms, _ := cmd.Flags().GetIntSlice("rooms"); len(rooms) > 0 {
					payload["action"] = "clean_rooms"
					payload["rooms"] = rooms
					repeat, _ := cmd.Flags().GetInt("repeat")
					payload["repeat"] = repeat
				}
			}
			var out map[string]any
			if err := postRoborockControlJSON(cmd.Context(), roborockControlBaseURL(cmd)+"/api/roborock/control", payload, &out); err != nil {
				return err
			}
			if isJSON(cmd) {
				return json.NewEncoder(os.Stdout).Encode(out)
			}
			fmt.Printf("OK: %s -> %s\n", payload["action"], args[0])
			return nil
		},
	}
	if action == "start" {
		c.Flags().IntSlice("rooms", nil, "Optional segment/room IDs for targeted room cleaning")
		c.Flags().Int("repeat", 1, "Number of cleaning passes")
	}
	return c
}

var roborockCameraCmd = &cobra.Command{
	Use:   "camera [device]",
	Short: "Inspect WebRTC TURN relay and 15-char camera bootstrap configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		u := roborockControlBaseURL(cmd) + "/api/roborock/camera"
		if len(args) == 1 {
			u += "?id=" + url.QueryEscape(args[0])
		}
		var out map[string]any
		if err := getRoborockControlJSON(cmd.Context(), u, &out); err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	},
}

var roborockKeysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Inspect recovered RSA device public keys and captured onboarding samples",
	RunE: func(cmd *cobra.Command, args []string) error {
		storage := core.NewDirStorage(roborockStorageDir())
		ks, err := roborock.NewKeyStore(storage, nil, nil)
		if err != nil {
			return err
		}
		snap := ks.RecordSnapshot()
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"count": len(snap), "devices": snap})
		}
		if len(snap) == 0 {
			fmt.Println("No device keys or samples stored yet.")
			return nil
		}
		for did, rec := range snap {
			state := "missing"
			if rec.Recovery != nil && rec.Recovery.State != "" {
				state = rec.Recovery.State
			}
			fmt.Printf("DID=%-14s model=%-22s state=%-10s query_samples=%d header_samples=%d has_key=%v\n",
				did, rec.Model, state, len(rec.Samples), len(rec.HeaderSamples), rec.ModulusHex != "")
		}
		return nil
	},
}

var roborockImportCmd = &cobra.Command{
	Use:   "import-state <upstream-data-dir>",
	Short: "Import device_keys.json / runtime_credentials.json / web_api_inventory.json from Python local_roborock_server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		storage := core.NewDirStorage(roborockStorageDir())
		ks, err := roborock.NewKeyStore(storage, nil, nil)
		if err != nil {
			return err
		}
		cs, err := roborock.NewCredentialStore(storage)
		if err != nil {
			return err
		}
		kCnt, dCnt, err := roborock.ImportUpstreamDir(args[0], ks, cs)
		if err != nil {
			return err
		}
		if isJSON(cmd) {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"imported_keys": kCnt, "imported_devices": dCnt})
		}
		fmt.Printf("Imported %d RSA device key(s) and %d device record(s) into %s\n", kCnt, dCnt, roborockStorageDir())
		return nil
	},
}

func getRoborockControlJSON(ctx context.Context, rawURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}

func postRoborockControlJSON(ctx context.Context, rawURL string, payload any, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}

func init() {
	rootCmd.AddCommand(roborockCmd)
	roborockCmd.PersistentFlags().String("control-url", "", "Local Roborock control API URL (default http://127.0.0.1:8889)")

	roborockServeCmd.Flags().String("fqdn", "", "Stack FQDN served to vacuums (e.g. api-r.example.com)")
	roborockServeCmd.Flags().String("bind", "", "Bind IP address (default 0.0.0.0)")
	roborockServeCmd.Flags().Int("https-port", 443, "HTTPS bootstrap listener port")
	roborockServeCmd.Flags().Int("advertised-port", 0, "Advertised HTTPS port when behind a reverse proxy")
	roborockServeCmd.Flags().Int("mqtt-port", 8881, "Embedded MQTT TLS listener port")
	roborockServeCmd.Flags().String("cert", "", "Default TLS fullchain PEM file (hot-reloaded on change)")
	roborockServeCmd.Flags().String("key", "", "Default TLS private key PEM file (hot-reloaded on change)")
	roborockServeCmd.Flags().StringSlice("sni-cert", nil, "Per-SNI certificate mapping host=cert.pem:key.pem (repeatable)")
	roborockServeCmd.Flags().Bool("legacy-tls", false, "Allow TLS 1.0+ for older Roborock firmware")
	roborockServeCmd.Flags().Bool("no-tls", false, "Disable TLS termination (use only behind external TLS proxy)")
	roborockServeCmd.Flags().String("region", "", "Region code (us, eu, cn, ru)")
	roborockServeCmd.Flags().String("timezone", "", "IANA timezone (e.g. America/Denver)")
	roborockServeCmd.Flags().String("camera-domain", "", "Short camera bootstrap domain (<=14 chars) for Saros 10R/20")
	roborockServeCmd.Flags().String("turn-mode", "", "WebRTC TURN mode: disabled | embedded | external")
	roborockServeCmd.Flags().String("turn-host", "", "TURN/STUN reachable LAN hostname or IP")
	roborockServeCmd.Flags().Int("turn-port", 3478, "TURN/STUN UDP port")
	roborockServeCmd.Flags().String("turn-user", "", "TURN long-term username")
	roborockServeCmd.Flags().String("turn-pass", "", "TURN long-term password")
	roborockServeCmd.Flags().Bool("trace", false, "Persist decoded telemetry events to telemetry_trace.jsonl")
	roborockServeCmd.Flags().String("control-bind", "127.0.0.1:8889", "Bind address for local control/SSE API")

	roborockOnboardCmd.Flags().String("server", "", "Local server hostname (e.g. api-r.example.com)")
	roborockOnboardCmd.Flags().Int("https-port", 443, "HTTPS port if omitted from --server")
	roborockOnboardCmd.Flags().String("ssid", "", "2.4 GHz home Wi-Fi SSID")
	roborockOnboardCmd.Flags().String("password", "", "Home Wi-Fi password")
	roborockOnboardCmd.Flags().String("timezone", "America/Denver", "IANA timezone")
	roborockOnboardCmd.Flags().String("cst", "", "POSIX timezone string (auto-derived from --timezone if empty)")
	roborockOnboardCmd.Flags().String("country-domain", "", "Regional country domain (auto-derived if empty)")
	roborockOnboardCmd.Flags().String("camera-domain", "", "Short camera bootstrap domain (<=14 chars) for Saros 10R")
	roborockOnboardCmd.Flags().String("model", "", "Optional vacuum model code (e.g. a125, a144, a288)")
	roborockOnboardCmd.Flags().Bool("preflight-only", false, "Run DNS/TLS/MQTT/camera preflight checks without sending cfgwifi")
	roborockOnboardCmd.Flags().Bool("insecure", false, "Skip TLS verification during admin preflight check")
	roborockOnboardCmd.Flags().String("ap-host", roborock.DefaultCfgWiFiHost, "Vacuum hotspot gateway IP")
	roborockOnboardCmd.Flags().Int("ap-port", roborock.DefaultCfgWiFiPort, "Vacuum hotspot UDP onboarding port")

	roborockCmd.AddCommand(
		roborockServeCmd,
		roborockOnboardCmd,
		roborockListCmd,
		roborockStatusCmd,
		roborockMonitorCmd,
		makeRoborockActionCmd("clean", "Start whole-house or room cleaning", "start"),
		makeRoborockActionCmd("pause", "Pause active cleaning", "pause"),
		makeRoborockActionCmd("stop", "Stop active cleaning", "stop"),
		makeRoborockActionCmd("dock", "Send vacuum back to dock", "dock"),
		makeRoborockActionCmd("find", "Ring vacuum locator chime", "find"),
		roborockCameraCmd,
		roborockKeysCmd,
		roborockImportCmd,
	)
}
