package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClientInterface defines the operations required on a Sonos speaker.
type ClientInterface interface {
	GetVolume() (int, error)
	SetVolume(volume int) error
	GetTransportInfo() (sonos.TransportInfo, error)
	GetPositionInfo() (sonos.PositionInfo, error)
	ParseTrackMetadata(xmlStr string) (sonos.TrackMetadata, error)
	GetCoordinatorIP() (string, error)
	GetZoneGroupState() (sonos.ZoneGroupState, error)
	BrowseFavorites() ([]sonos.Favorite, error)
	PlayFavorite(idOrTitle string) error
	PlayStream(streamURL, title string) error
	AddURIToQueue(uri, metadata string, asNext bool) (int, error)
	ListMusicServices() ([]sonos.MusicService, error)
	Play() error
	Pause() error
	Stop() error
	Next() error
	Previous() error
}

// ClientFactory creates a ClientInterface for the specified IP.
type ClientFactory func(ip string) ClientInterface

// CacheLoader loads cached Sonos devices.
type CacheLoader func() ([]sonos.Device, error)

// CacheSaver saves discovered Sonos devices to cache.
type CacheSaver func(devices []sonos.Device) error

// Discoverer runs network discovery for Sonos devices.
type Discoverer func(timeout time.Duration) ([]sonos.Device, error)

// ServerConfig configures the Sonos MCP server.
type ServerConfig struct {
	ClientFactory ClientFactory
	CacheLoader   CacheLoader
	CacheSaver    CacheSaver
	Discoverer    Discoverer
}

// ServerOption configures ServerConfig.
type ServerOption func(*ServerConfig)

// WithClientFactory overrides the client factory for testing.
func WithClientFactory(f ClientFactory) ServerOption {
	return func(c *ServerConfig) {
		if f != nil {
			c.ClientFactory = f
		}
	}
}

// WithCacheLoader overrides cache loading.
func WithCacheLoader(l CacheLoader) ServerOption {
	return func(c *ServerConfig) {
		if l != nil {
			c.CacheLoader = l
		}
	}
}

// WithCacheSaver overrides cache saving.
func WithCacheSaver(s CacheSaver) ServerOption {
	return func(c *ServerConfig) {
		if s != nil {
			c.CacheSaver = s
		}
	}
}

// WithDiscoverer overrides discovery.
func WithDiscoverer(d Discoverer) ServerOption {
	return func(c *ServerConfig) {
		if d != nil {
			c.Discoverer = d
		}
	}
}

func defaultServerConfig() ServerConfig {
	return ServerConfig{
		ClientFactory: func(ip string) ClientInterface {
			return sonos.NewClient(ip)
		},
		CacheLoader: sonos.LoadCache,
		CacheSaver:  sonos.SaveCache,
		Discoverer:  sonos.Discover,
	}
}

// --- Parameter Structs ---

// ListSpeakersParams defines parameters for sonos_list_speakers.
type ListSpeakersParams struct {
	Refresh bool `json:"refresh,omitempty" jsonschema:"Whether to actively scan the network via mDNS instead of using cached devices"`
}

// NowPlayingParams defines parameters for sonos_get_now_playing.
type NowPlayingParams struct {
	IP string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
}

// ControlParams defines parameters for sonos_control.
type ControlParams struct {
	IP     string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
	Action string `json:"action" jsonschema:"Playback action: 'play', 'pause', 'stop', 'next', 'previous' (required)"`
}

// SetVolumeParams defines parameters for sonos_set_volume.
type SetVolumeParams struct {
	IP     string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
	Volume int    `json:"volume,omitempty" jsonschema:"Target volume level between 0 and 100"`
	Delta  int    `json:"delta,omitempty" jsonschema:"Relative volume change (e.g. +5 or -10). Applied if volume is 0 or omitted."`
}

// ListFavoritesParams defines parameters for sonos_list_favorites.
type ListFavoritesParams struct {
	IP string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
}

// PlayFavoriteParams defines parameters for sonos_play_favorite.
type PlayFavoriteParams struct {
	IP         string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
	FavoriteID string `json:"favorite_id" jsonschema:"ID or title of the favorite item to play (required)"`
}

// PlayStreamParams defines parameters for sonos_play_stream.
type PlayStreamParams struct {
	IP    string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
	URL   string `json:"url" jsonschema:"HTTP or HTTPS audio stream URL (required)"`
	Title string `json:"title,omitempty" jsonschema:"Optional display title for the stream (defaults to URL host)"`
}

// AddToQueueParams defines parameters for sonos_add_to_queue.
type AddToQueueParams struct {
	IP     string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
	URI    string `json:"uri" jsonschema:"Audio URI to enqueue (required)"`
	AsNext bool   `json:"as_next,omitempty" jsonschema:"If true, inserts track as next to play instead of appending to end of queue"`
}

// ListServicesParams defines parameters for sonos_list_services.
type ListServicesParams struct {
	IP string `json:"ip" jsonschema:"IP address of the Sonos speaker (required)"`
}

// ListFavoritesResult represents the structured, object-wrapped output of sonos_list_favorites.
type ListFavoritesResult struct {
	Count     int              `json:"count"`
	Favorites []sonos.Favorite `json:"favorites"`
}

// ListServicesResult represents the structured, object-wrapped output of sonos_list_services.
type ListServicesResult struct {
	Count    int                  `json:"count"`
	Default  *sonos.MusicService  `json:"default,omitempty"`
	Services []sonos.MusicService `json:"services"`
}

// ListSpeakersResult represents the structured, object-wrapped output of sonos_list_speakers.
type ListSpeakersResult struct {
	Count    int            `json:"count"`
	Speakers []sonos.Device `json:"speakers"`
}

// NowPlayingResult is the compact, token-optimized track status payload.
type NowPlayingResult struct {
	IP            string `json:"ip"`
	State         string `json:"state"`
	Volume        int    `json:"volume"`
	Title         string `json:"title,omitempty"`
	Artist        string `json:"artist,omitempty"`
	Album         string `json:"album,omitempty"`
	Duration      string `json:"duration,omitempty"`
	Progress      string `json:"progress,omitempty"`
	StreamContent string `json:"stream_content,omitempty"`
	// TrackURI is the raw transport URI. For a stereo-pair/group follower this is
	// an "x-rincon:<coordinator-rincon>" reference rather than real media.
	TrackURI string `json:"track_uri,omitempty"`
	// IsFollower is true when the queried speaker was a stereo-pair/group follower
	// whose transport merely mirrors the coordinator; in that case the reported
	// fields are resolved from CoordinatorIP.
	IsFollower bool `json:"is_follower,omitempty"`
	// CoordinatorIP is the IP of the group coordinator whose playback state is
	// authoritative. Populated (and playback re-queried from it) when IsFollower.
	CoordinatorIP string `json:"coordinator_ip,omitempty"`
}

// TopologyMember describes a single speaker within a zone group.
type TopologyMember struct {
	UUID          string `json:"uuid"`
	RoomName      string `json:"room_name"`
	IP            string `json:"ip,omitempty"`
	IsCoordinator bool   `json:"is_coordinator"`
	Invisible     bool   `json:"invisible,omitempty"`
}

// TopologyGroup describes a zone group (a standalone speaker, a stereo pair, or a
// multi-room group), identifying its coordinator and members.
type TopologyGroup struct {
	ID          string           `json:"id"`
	Coordinator string           `json:"coordinator_uuid"`
	IsPair      bool             `json:"is_pair"`
	Members     []TopologyMember `json:"members"`
}

// TopologyResult is the object-wrapped output of sonos_get_topology.
type TopologyResult struct {
	Count  int             `json:"count"`
	Groups []TopologyGroup `json:"groups"`
}

// locationToIP extracts the host portion from a Sonos device Location URL
// (e.g. "http://192.168.4.99:1400/xml/device_description.xml" -> "192.168.4.99").
func locationToIP(loc string) string {
	loc = strings.TrimPrefix(loc, "http://")
	loc = strings.TrimPrefix(loc, "https://")
	if idx := strings.Index(loc, ":"); idx != -1 {
		return loc[:idx]
	}
	if idx := strings.Index(loc, "/"); idx != -1 {
		return loc[:idx]
	}
	return loc
}

// CreateMCPServer constructs and registers all Sonos tools on an MCP server.
func CreateMCPServer(opts ...ServerOption) *mcp.Server {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "homectl-sonos",
		Version: "1.0.0",
	}, nil)

	// Tool 1: sonos_list_speakers (Read-Only)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_list_speakers",
		Description: "Lists all discovered Sonos speakers on the network, returning IP, room name, Rincon ID, and model.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListSpeakersParams) (*mcp.CallToolResult, any, error) {
		var speakers []sonos.Device
		var err error

		if !args.Refresh {
			speakers, _ = cfg.CacheLoader()
		}

		if len(speakers) == 0 || args.Refresh {
			speakers, err = cfg.Discoverer(3 * time.Second)
			if err != nil {
				return nil, nil, fmt.Errorf("discovery failed: %w", err)
			}
			if len(speakers) > 0 {
				_ = cfg.CacheSaver(speakers)
			}
		}

		res := ListSpeakersResult{
			Count:    len(speakers),
			Speakers: speakers,
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode speakers: %w", err)
		}

		summaryText := fmt.Sprintf("Found %d Sonos speaker(s)", len(speakers))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summaryText},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, res, nil
	})

	// Tool 2: sonos_get_now_playing (Read-Only, compact JSON)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_get_now_playing",
		Description: "Retrieves the current playback state, track metadata, progress, and volume for a Sonos speaker as compact JSON. Automatically resolves stereo-pair/group followers to their coordinator so playback state is authoritative (see is_follower/coordinator_ip in the result).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args NowPlayingParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}

		targetIP := args.IP
		client := cfg.ClientFactory(targetIP)

		pos, _ := client.GetPositionInfo()

		// Stereo-pair / group followers report their transport as PLAYING with an
		// "x-rincon:<coordinator-rincon>" TrackURI and no real metadata. Redirect
		// to the coordinator so we report authoritative playback state instead of a
		// false-positive "PLAYING with no track". See control-84r.
		isFollower := false
		coordinatorIP := ""
		if strings.HasPrefix(pos.TrackURI, "x-rincon:") {
			if coord, err := client.GetCoordinatorIP(); err == nil && coord != "" && coord != targetIP {
				isFollower = true
				coordinatorIP = coord
				targetIP = coord
				client = cfg.ClientFactory(targetIP)
				pos, _ = client.GetPositionInfo()
			}
		}

		transport, _ := client.GetTransportInfo()
		vol, _ := client.GetVolume()
		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)

		res := NowPlayingResult{
			IP:            targetIP,
			State:         transport.CurrentTransportState,
			Volume:        vol,
			Title:         meta.Title,
			Artist:        meta.Artist,
			Album:         meta.Album,
			Duration:      pos.TrackDuration,
			Progress:      pos.RelTime,
			StreamContent: meta.StreamContent,
			TrackURI:      pos.TrackURI,
			IsFollower:    isFollower,
			CoordinatorIP: coordinatorIP,
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode now-playing data: %w", err)
		}

		summary := fmt.Sprintf("[%s] %s - %s (%s)", res.State, res.Artist, res.Title, res.Progress)
		if res.Title == "" && res.StreamContent != "" {
			summary = fmt.Sprintf("[%s] Stream: %s", res.State, res.StreamContent)
		} else if res.Title == "" {
			summary = fmt.Sprintf("[%s] No track loaded", res.State)
		}
		if res.IsFollower {
			summary = fmt.Sprintf("%s (resolved from stereo-pair/group coordinator %s)", summary, res.CoordinatorIP)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summary},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, res, nil
	})

	// Tool 3: sonos_get_topology (Read-Only) — exposes group/stereo-pair structure
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_get_topology",
		Description: "Returns the Sonos zone-group topology (standalone speakers, stereo pairs, and multi-room groups), identifying the coordinator and members of each group. Use this to understand which speaker is authoritative before controlling playback.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args NowPlayingParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required (any speaker on the household)")
		}

		client := cfg.ClientFactory(args.IP)
		state, err := client.GetZoneGroupState()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read zone group state from %s: %w", args.IP, err)
		}

		result := TopologyResult{}
		for _, g := range state.Groups {
			group := TopologyGroup{
				ID:          g.ID,
				Coordinator: g.Coordinator,
			}
			for _, m := range g.Members {
				group.Members = append(group.Members, TopologyMember{
					UUID:          m.UUID,
					RoomName:      m.RoomName,
					IP:            locationToIP(m.Location),
					IsCoordinator: m.UUID == g.Coordinator,
					Invisible:     m.Invisible,
				})
			}
			// A stereo pair presents as a single group of exactly two bonded members.
			group.IsPair = len(group.Members) == 2
			result.Groups = append(result.Groups, group)
		}
		result.Count = len(result.Groups)

		jsonData, err := json.Marshal(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode topology: %w", err)
		}

		summary := fmt.Sprintf("Found %d Sonos zone group(s)", result.Count)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summary},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, result, nil
	})

	// Tool 4: sonos_control (Mutating)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_control",
		Description: "Controls playback on a Sonos speaker: 'play', 'pause', 'stop', 'next', 'previous'.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ControlParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}
		action := strings.TrimSpace(strings.ToLower(args.Action))
		if action == "" {
			return nil, nil, fmt.Errorf("action parameter is required ('play', 'pause', 'stop', 'next', 'previous')")
		}

		client := cfg.ClientFactory(args.IP)
		var err error

		switch action {
		case "play":
			err = client.Play()
		case "pause":
			err = client.Pause()
		case "stop":
			err = client.Stop()
		case "next":
			err = client.Next()
		case "prev", "previous":
			err = client.Previous()
		default:
			return nil, nil, fmt.Errorf("unknown action %q (supported: 'play', 'pause', 'stop', 'next', 'previous')", action)
		}

		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute %s on %s: %w", action, args.IP, err)
		}

		msg := fmt.Sprintf("Successfully executed '%s' on Sonos speaker at %s", action, args.IP)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, map[string]string{"status": "ok", "action": action, "ip": args.IP}, nil
	})

	// Tool 5: sonos_set_volume (Mutating)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_set_volume",
		Description: "Sets the volume of a Sonos speaker (0-100) or applies a relative delta (e.g. +5, -10).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SetVolumeParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}

		client := cfg.ClientFactory(args.IP)
		targetVol := args.Volume

		prevVol, _ := client.GetVolume()

		if args.Delta != 0 || (args.Volume == 0 && args.Delta != 0) {
			targetVol = prevVol + args.Delta
		}

		if targetVol < 0 {
			targetVol = 0
		} else if targetVol > 100 {
			targetVol = 100
		}

		if err := client.SetVolume(targetVol); err != nil {
			return nil, nil, fmt.Errorf("failed to set volume on %s: %w", args.IP, err)
		}

		msg := fmt.Sprintf("Adjusted volume on %s: %d%% -> %d%%", args.IP, prevVol, targetVol)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, map[string]any{"status": "ok", "ip": args.IP, "previous_volume": prevVol, "volume": targetVol}, nil
	})

	// Tool 6: sonos_list_favorites (Read-Only)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_list_favorites",
		Description: "Lists all pinned Sonos favorites (playlists, albums, radio stations) on the speaker with ID, title, type, and description.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListFavoritesParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}

		client := cfg.ClientFactory(args.IP)
		favs, err := client.BrowseFavorites()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to browse favorites on %s: %w", args.IP, err)
		}

		res := ListFavoritesResult{
			Count:     len(favs),
			Favorites: favs,
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode favorites: %w", err)
		}

		summary := fmt.Sprintf("Found %d Sonos favorite(s) on %s", len(favs), args.IP)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summary},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, res, nil
	})

	// Tool 7: sonos_play_favorite (Mutating)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_play_favorite",
		Description: "Starts playback of a pinned Sonos favorite item by its ID or title on the specified speaker (with automatic coordinator resolution).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args PlayFavoriteParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}
		if strings.TrimSpace(args.FavoriteID) == "" {
			return nil, nil, fmt.Errorf("favorite_id parameter is required")
		}

		client := cfg.ClientFactory(args.IP)
		if err := client.PlayFavorite(args.FavoriteID); err != nil {
			return nil, nil, fmt.Errorf("failed to play favorite on %s: %w", args.IP, err)
		}

		msg := fmt.Sprintf("Successfully initiated playback of favorite %q on %s", args.FavoriteID, args.IP)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, map[string]string{"status": "ok", "action": "play_favorite", "ip": args.IP, "favorite_id": args.FavoriteID}, nil
	})

	// Tool 8: sonos_play_stream (Mutating)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_play_stream",
		Description: "Plays an arbitrary HTTP or HTTPS audio stream (e.g. internet radio or audio podcast) on the specified speaker with automatic coordinator resolution.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args PlayStreamParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}
		if strings.TrimSpace(args.URL) == "" {
			return nil, nil, fmt.Errorf("url parameter is required")
		}

		u, err := url.Parse(strings.TrimSpace(args.URL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, nil, fmt.Errorf("invalid stream URL %q: scheme must be http or https", args.URL)
		}

		title := args.Title
		if title == "" {
			title = u.Host
			if title == "" {
				title = "Audio Stream"
			}
		}

		client := cfg.ClientFactory(args.IP)
		if err := client.PlayStream(args.URL, title); err != nil {
			return nil, nil, fmt.Errorf("failed to play stream on %s: %w", args.IP, err)
		}

		msg := fmt.Sprintf("Successfully started stream %q (%s) on %s", title, args.URL, args.IP)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, map[string]string{"status": "ok", "action": "play_stream", "ip": args.IP, "url": args.URL, "title": title}, nil
	})

	// Tool 9: sonos_add_to_queue (Mutating)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_add_to_queue",
		Description: "Adds an audio URI to the Sonos playback queue (optionally as next track) on the group coordinator.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args AddToQueueParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}
		if strings.TrimSpace(args.URI) == "" {
			return nil, nil, fmt.Errorf("uri parameter is required")
		}

		client := cfg.ClientFactory(args.IP)
		pos, err := client.AddURIToQueue(args.URI, "", args.AsNext)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add URI to queue on %s: %w", args.IP, err)
		}

		msg := fmt.Sprintf("Successfully added URI to queue at track position %d on %s", pos, args.IP)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, map[string]any{"status": "ok", "action": "add_to_queue", "ip": args.IP, "uri": args.URI, "track_position": pos, "as_next": args.AsNext}, nil
	})

	// Tool 10: sonos_list_services (Read-Only)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sonos_list_services",
		Description: "Lists all registered music streaming services (Spotify, Apple Music, YouTube Music, etc.) available on the Sonos speaker catalog, noting the configured default.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListServicesParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}

		client := cfg.ClientFactory(args.IP)
		services, err := client.ListMusicServices()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list music services on %s: %w", args.IP, err)
		}

		appCfg := config.LoadConfig()
		defSvc, hasDef := sonos.ResolveDefaultService(services, appCfg.SonosDefaultService)
		var defPtr *sonos.MusicService
		if hasDef {
			defPtr = &defSvc
		}

		res := ListServicesResult{
			Count:    len(services),
			Default:  defPtr,
			Services: services,
		}

		jsonData, err := json.Marshal(res)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode music services: %w", err)
		}

		summary := fmt.Sprintf("Found %d music service(s) on %s", len(services), args.IP)
		if hasDef {
			summary += fmt.Sprintf(" (default: %s)", defSvc.Name)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summary},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, res, nil
	})

	return server
}
