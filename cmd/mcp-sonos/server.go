package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ClientInterface defines the operations required on a Sonos speaker.
type ClientInterface interface {
	GetVolume() (int, error)
	SetVolume(volume int) error
	GetTransportInfo() (sonos.TransportInfo, error)
	GetPositionInfo() (sonos.PositionInfo, error)
	ParseTrackMetadata(xmlStr string) (sonos.TrackMetadata, error)
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
		Description: "Retrieves the current playback state, track metadata, progress, and volume for a Sonos speaker as compact JSON.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args NowPlayingParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.IP) == "" {
			return nil, nil, fmt.Errorf("ip parameter is required")
		}

		client := cfg.ClientFactory(args.IP)

		transport, _ := client.GetTransportInfo()
		vol, _ := client.GetVolume()
		pos, _ := client.GetPositionInfo()
		meta, _ := client.ParseTrackMetadata(pos.TrackMetaData)

		res := NowPlayingResult{
			IP:            args.IP,
			State:         transport.CurrentTransportState,
			Volume:        vol,
			Title:         meta.Title,
			Artist:        meta.Artist,
			Album:         meta.Album,
			Duration:      pos.TrackDuration,
			Progress:      pos.RelTime,
			StreamContent: meta.StreamContent,
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

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: summary},
				&mcp.TextContent{Text: string(jsonData)},
			},
		}, res, nil
	})

	// Tool 3: sonos_control (Mutating)
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

	// Tool 4: sonos_set_volume (Mutating)
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

	return server
}
