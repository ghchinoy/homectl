package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MockClient implements ClientInterface for testing.
type MockClient struct {
	ip       string
	volume   int
	state    string
	title    string
	artist   string
	album    string
	trackURI string
	failPlay bool
	failVol  bool

	// coordinatorIP, when non-empty, marks this mock as a stereo-pair/group
	// follower whose GetCoordinatorIP() redirects to that address.
	coordinatorIP string
	// zoneGroupState is returned verbatim by GetZoneGroupState().
	zoneGroupState sonos.ZoneGroupState
}

func (m *MockClient) GetVolume() (int, error) {
	if m.failVol {
		return 0, errors.New("volume read failure")
	}
	return m.volume, nil
}

func (m *MockClient) SetVolume(v int) error {
	if m.failVol {
		return errors.New("volume write failure")
	}
	m.volume = v
	return nil
}

func (m *MockClient) GetTransportInfo() (sonos.TransportInfo, error) {
	return sonos.TransportInfo{
		CurrentTransportState: m.state,
	}, nil
}

func (m *MockClient) GetPositionInfo() (sonos.PositionInfo, error) {
	return sonos.PositionInfo{
		TrackDuration: "0:04:12",
		RelTime:       "0:01:45",
		TrackMetaData: "<item><title>Mock Song</title></item>",
		TrackURI:      m.trackURI,
	}, nil
}

func (m *MockClient) GetCoordinatorIP() (string, error) {
	if m.coordinatorIP != "" {
		return m.coordinatorIP, nil
	}
	return m.ip, nil
}

func (m *MockClient) GetZoneGroupState() (sonos.ZoneGroupState, error) {
	return m.zoneGroupState, nil
}

func (m *MockClient) ParseTrackMetadata(xmlStr string) (sonos.TrackMetadata, error) {
	return sonos.TrackMetadata{
		Title:  m.title,
		Artist: m.artist,
		Album:  m.album,
	}, nil
}

func (m *MockClient) Play() error {
	if m.failPlay {
		return errors.New("playback failure")
	}
	m.state = "PLAYING"
	return nil
}

func (m *MockClient) Pause() error {
	m.state = "PAUSED_PLAYBACK"
	return nil
}

func (m *MockClient) Stop() error {
	m.state = "STOPPED"
	return nil
}

func (m *MockClient) Next() error {
	m.title = "Next Track"
	return nil
}

func (m *MockClient) Previous() error {
	m.title = "Previous Track"
	return nil
}

func setupTestSession(t *testing.T, mockClient *MockClient) (*mcp.ClientSession, func()) {
	return setupTestSessionWithFactory(t, func(ip string) ClientInterface {
		return mockClient
	})
}

// setupTestSessionWithFactory wires a custom, IP-aware client factory so tests can
// simulate multi-speaker topologies (e.g. stereo-pair follower -> coordinator).
func setupTestSessionWithFactory(t *testing.T, factory ClientFactory) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Office Speaker", IP: "192.168.4.120", RinconID: "RINCON_001", ModelName: "Sonos One"},
			}, nil
		}),
	)

	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatalf("failed to connect client: %v", err)
	}

	cleanup := func() {
		session.Close()
		cancel()
	}

	return session, cleanup
}

func TestListTools(t *testing.T) {
	mock := &MockClient{volume: 25, state: "PLAYING"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"sonos_list_speakers",
		"sonos_get_now_playing",
		"sonos_get_topology",
		"sonos_control",
		"sonos_set_volume",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("missing expected tool: %s", expected)
		}
	}
}

func TestSonosListSpeakersTool(t *testing.T) {
	mock := &MockClient{volume: 20}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_speakers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_speakers failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("expected at least 2 content items, got %d", len(res.Content))
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var listResult ListSpeakersResult
	if err := json.Unmarshal([]byte(textContent.Text), &listResult); err != nil {
		t.Fatalf("failed to unmarshal ListSpeakersResult: %v", err)
	}

	if listResult.Count != 1 || len(listResult.Speakers) != 1 || listResult.Speakers[0].Name != "Office Speaker" {
		t.Errorf("unexpected speakers returned: %+v", listResult)
	}

	// Verify that structuredContent is an object/record (not a bare array) per SEP-2106
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be map[string]any (record/object), got %T: %+v",
				res.StructuredContent, res.StructuredContent)
		}
	}
}

func TestSonosGetNowPlayingTool(t *testing.T) {
	mock := &MockClient{
		volume: 30,
		state:  "PLAYING",
		title:  "Take Five",
		artist: "Dave Brubeck",
		album:  "Time Out",
	}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_get_now_playing",
		Arguments: map[string]any{
			"ip": "192.168.4.120",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_get_now_playing failed: %v", err)
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var nowPlaying NowPlayingResult
	if err := json.Unmarshal([]byte(textContent.Text), &nowPlaying); err != nil {
		t.Fatalf("failed to unmarshal now playing JSON: %v", err)
	}

	if nowPlaying.Title != "Take Five" || nowPlaying.Artist != "Dave Brubeck" || nowPlaying.Volume != 30 {
		t.Errorf("unexpected now-playing result: %+v", nowPlaying)
	}
}

func TestSonosControlTool(t *testing.T) {
	mock := &MockClient{state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Play
	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.4.120",
			"action": "play",
		},
	})
	if err != nil {
		t.Fatalf("CallTool play failed: %v", err)
	}
	if mock.state != "PLAYING" {
		t.Errorf("expected state PLAYING, got %s", mock.state)
	}

	// 2. Pause
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.4.120",
			"action": "pause",
		},
	})
	if err != nil {
		t.Fatalf("CallTool pause failed: %v", err)
	}
	if mock.state != "PAUSED_PLAYBACK" {
		t.Errorf("expected state PAUSED_PLAYBACK, got %s", mock.state)
	}

	// 3. Invalid action
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.4.120",
			"action": "invalid_action",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("expected error or IsError=true for invalid action, got success")
	}
}

func TestSonosSetVolumeTool(t *testing.T) {
	mock := &MockClient{volume: 20}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Absolute volume
	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_set_volume",
		Arguments: map[string]any{
			"ip":     "192.168.4.120",
			"volume": 45,
		},
	})
	if err != nil {
		t.Fatalf("CallTool absolute volume failed: %v", err)
	}
	if mock.volume != 45 {
		t.Errorf("expected volume 45, got %d", mock.volume)
	}

	// 2. Relative delta (+10)
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_set_volume",
		Arguments: map[string]any{
			"ip":    "192.168.4.120",
			"delta": 10,
		},
	})
	if err != nil {
		t.Fatalf("CallTool delta +10 failed: %v", err)
	}
	if mock.volume != 55 {
		t.Errorf("expected volume 55, got %d", mock.volume)
	}

	// 3. Clamping to 100
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_set_volume",
		Arguments: map[string]any{
			"ip":     "192.168.4.120",
			"volume": 150,
		},
	})
	if err != nil {
		t.Fatalf("CallTool clamp to 100 failed: %v", err)
	}
	if mock.volume != 100 {
		t.Errorf("expected volume 100, got %d", mock.volume)
	}
}

// TestSonosGetNowPlayingFollowerRedirect verifies that querying a stereo-pair
// follower (which reports state=PLAYING with an x-rincon: TrackURI and no
// metadata) transparently redirects to the coordinator and reports the
// coordinator's authoritative state. Regression for control-84r.
func TestSonosGetNowPlayingFollowerRedirect(t *testing.T) {
	const followerIP = "192.168.4.98"
	const coordinatorIP = "192.168.4.99"

	follower := &MockClient{
		ip:            followerIP,
		volume:        15,
		state:         "PLAYING", // false-positive transport state of a follower
		trackURI:      "x-rincon:RINCON_000E58CF418201400",
		coordinatorIP: coordinatorIP,
	}
	coordinator := &MockClient{
		ip:     coordinatorIP,
		volume: 22,
		state:  "STOPPED",
		title:  "Poison",
		artist: "Alice Cooper",
	}

	factory := func(ip string) ClientInterface {
		if ip == coordinatorIP {
			return coordinator
		}
		return follower
	}

	session, cleanup := setupTestSessionWithFactory(t, factory)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_get_now_playing",
		Arguments: map[string]any{
			"ip": followerIP,
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_get_now_playing failed: %v", err)
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var np NowPlayingResult
	if err := json.Unmarshal([]byte(textContent.Text), &np); err != nil {
		t.Fatalf("failed to unmarshal now-playing JSON: %v", err)
	}

	if !np.IsFollower {
		t.Errorf("expected IsFollower=true for follower query, got false: %+v", np)
	}
	if np.CoordinatorIP != coordinatorIP {
		t.Errorf("expected CoordinatorIP=%s, got %q", coordinatorIP, np.CoordinatorIP)
	}
	if np.IP != coordinatorIP {
		t.Errorf("expected reported IP to redirect to coordinator %s, got %q", coordinatorIP, np.IP)
	}
	if np.State != "STOPPED" {
		t.Errorf("expected authoritative State=STOPPED from coordinator, got %q", np.State)
	}
	if np.Title != "Poison" || np.Artist != "Alice Cooper" {
		t.Errorf("expected coordinator track 'Poison' by 'Alice Cooper', got %q by %q", np.Title, np.Artist)
	}
}

// TestSonosGetTopologyTool verifies the topology tool surfaces group/stereo-pair
// structure with coordinator identification. Regression for control-rs9.
func TestSonosGetTopologyTool(t *testing.T) {
	mock := &MockClient{
		ip: "192.168.4.99",
		zoneGroupState: sonos.ZoneGroupState{
			Groups: []sonos.ZoneGroup{
				{
					ID:          "RINCON_000E58CF418201400:1",
					Coordinator: "RINCON_000E58CF418201400",
					Members: []sonos.ZoneGroupMember{
						{UUID: "RINCON_000E58CF418201400", RoomName: "Office", Location: "http://192.168.4.99:1400/xml/device_description.xml"},
						{UUID: "RINCON_000E58CF418200400", RoomName: "Office", Location: "http://192.168.4.98:1400/xml/device_description.xml"},
					},
				},
				{
					ID:          "RINCON_ABC:2",
					Coordinator: "RINCON_ABC",
					Members: []sonos.ZoneGroupMember{
						{UUID: "RINCON_ABC", RoomName: "Kitchen", Location: "http://192.168.4.50:1400/xml/device_description.xml"},
					},
				},
			},
		},
	}

	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_get_topology",
		Arguments: map[string]any{
			"ip": "192.168.4.99",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_get_topology failed: %v", err)
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var topo TopologyResult
	if err := json.Unmarshal([]byte(textContent.Text), &topo); err != nil {
		t.Fatalf("failed to unmarshal topology JSON: %v", err)
	}

	if topo.Count != 2 || len(topo.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %+v", topo)
	}

	pair := topo.Groups[0]
	if !pair.IsPair {
		t.Errorf("expected first group to be a stereo pair, got IsPair=false")
	}
	if len(pair.Members) != 2 {
		t.Fatalf("expected 2 members in pair, got %d", len(pair.Members))
	}
	var coordFound bool
	for _, m := range pair.Members {
		if m.UUID == pair.Coordinator {
			if !m.IsCoordinator {
				t.Errorf("coordinator member %s not flagged IsCoordinator", m.UUID)
			}
			if m.IP != "192.168.4.99" {
				t.Errorf("expected coordinator IP 192.168.4.99, got %q", m.IP)
			}
			coordFound = true
		}
	}
	if !coordFound {
		t.Error("coordinator member not present in pair members")
	}

	if topo.Groups[1].IsPair {
		t.Error("expected standalone Kitchen group to not be a pair")
	}

	// StructuredContent must be an object/record per SEP-2106.
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be a record, got %T", res.StructuredContent)
		}
	}
}
