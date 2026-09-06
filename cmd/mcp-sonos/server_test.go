package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	lastEnqueuedMetadata string
	lastSeekTrack        int
	lastSeekTarget       string
	nrTracks             int
	mediaURI             string
	lastRemoveStart      int
	lastRemoveCount      int
	clearQueueCalled     bool
	lastReorderStart     int
	lastReorderCount     int
	lastReorderInsert    int
	failTopology         bool
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

func (m *MockClient) GetMediaInfo() (sonos.MediaInfo, error) {
	nr := m.nrTracks
	if nr == 0 {
		nr = 12
	}
	uri := m.mediaURI
	if uri == "" {
		uri = "x-rincon-queue:RINCON_123456#0"
	}
	return sonos.MediaInfo{
		NrTracks:   nr,
		CurrentURI: uri,
	}, nil
}

func (m *MockClient) GetCoordinatorIP() (string, error) {
	if m.coordinatorIP != "" {
		return m.coordinatorIP, nil
	}
	return m.ip, nil
}

func (m *MockClient) GetZoneGroupState() (sonos.ZoneGroupState, error) {
	if m.failTopology {
		return sonos.ZoneGroupState{}, errors.New("timeout connecting to sleeping battery speaker")
	}
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

func (m *MockClient) SeekTrack(track int) error {
	m.lastSeekTrack = track
	return nil
}

func (m *MockClient) SeekTime(target string) error {
	m.lastSeekTarget = target
	return nil
}

func (m *MockClient) RemoveTrackRangeFromQueue(start, count int) error {
	m.lastRemoveStart = start
	m.lastRemoveCount = count
	return nil
}

func (m *MockClient) RemoveAllTracksFromQueue() error {
	m.clearQueueCalled = true
	return nil
}

func (m *MockClient) ReorderTracksInQueue(startingIndex, numberOfTracks, insertBefore int) error {
	m.lastReorderStart = startingIndex
	m.lastReorderCount = numberOfTracks
	m.lastReorderInsert = insertBefore
	return nil
}

func (m *MockClient) BrowseFavorites() ([]sonos.Favorite, error) {
	return []sonos.Favorite{
		{ID: "FV:2/1", Title: "Chill Vibes", Type: "playlist", Description: "Spotify"},
		{ID: "FV:2/2", Title: "Morning Jazz", Type: "audioBroadcast", Description: "Sonos Radio"},
	}, nil
}

func (m *MockClient) PlayFavorite(idOrTitle string) error {
	m.title = "Favorite: " + idOrTitle
	m.state = "PLAYING"
	return nil
}

func (m *MockClient) PlayTrackOrFavorite(query string) (*sonos.PlayResult, error) {
	m.title = query
	m.state = "PLAYING"
	return &sonos.PlayResult{
		Source:  "favorite",
		Title:   query,
		Message: "Successfully initiated playback of " + query,
	}, nil
}

func (m *MockClient) PlayStream(streamURL, title string) error {
	m.title = title
	m.state = "PLAYING"
	return nil
}

func (m *MockClient) AddURIToQueue(uri, metadata string, asNext bool) (int, error) {
	m.lastEnqueuedMetadata = metadata
	return 4, nil
}

func (m *MockClient) GetQueue(start, count int) (sonos.QueueResult, error) {
	return sonos.QueueResult{
		Items: []sonos.QueueItem{
			{
				Position: 1,
				TrackID:  "Q:0/1",
				Title:    "Track One",
				Artist:   "Artist One",
				Album:    "Album One",
				Duration: "0:03:45",
				URI:      "x-sonos-http:track1.mp3",
			},
			{
				Position: 2,
				TrackID:  "Q:0/2",
				Title:    "Track Two",
				Artist:   "Artist Two",
				Album:    "Album Two",
				Duration: "0:04:10",
				URI:      "x-file-cifs://nas/track2.flac",
			},
		},
		Returned:     2,
		TotalMatches: 25,
		StartIndex:   start,
	}, nil
}

func (m *MockClient) ListMusicServices() ([]sonos.MusicService, error) {
	return []sonos.MusicService{
		{ID: "9", Name: "Spotify", Version: "1.1"},
		{ID: "204", Name: "Apple Music", Version: "1.1"},
	}, nil
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
				{Name: "Office Speaker", IP: "192.168.1.120", RinconID: "RINCON_001", ModelName: "Sonos One"},
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
		"sonos_list_favorites",
		"sonos_play_favorite",
		"sonos_play_stream",
		"sonos_add_to_queue",
		"sonos_list_services",
		"sonos_get_queue",
		"sonos_queue_edit",
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
		volume:   30,
		state:    "PLAYING",
		title:    "Take Five",
		artist:   "Dave Brubeck",
		album:    "Time Out",
		nrTracks: 15,
		mediaURI: "x-rincon-queue:RINCON_123456#0",
	}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_get_now_playing",
		Arguments: map[string]any{
			"ip": "192.168.1.120",
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
	if nowPlaying.QueueLength != 15 {
		t.Errorf("expected QueueLength 15, got %d", nowPlaying.QueueLength)
	}
	if nowPlaying.MediaURI != "x-rincon-queue:RINCON_123456#0" {
		t.Errorf("expected MediaURI 'x-rincon-queue:RINCON_123456#0', got %s", nowPlaying.MediaURI)
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
			"ip":     "192.168.1.120",
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
			"ip":     "192.168.1.120",
			"action": "pause",
		},
	})
	if err != nil {
		t.Fatalf("CallTool pause failed: %v", err)
	}
	if mock.state != "PAUSED_PLAYBACK" {
		t.Errorf("expected state PAUSED_PLAYBACK, got %s", mock.state)
	}

	// 3. Seek track
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "seek_track",
			"track":  4,
		},
	})
	if err != nil {
		t.Fatalf("CallTool seek_track failed: %v", err)
	}
	if mock.lastSeekTrack != 4 {
		t.Errorf("expected lastSeekTrack 4, got %d", mock.lastSeekTrack)
	}

	// 4. Seek track invalid (track < 1)
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "seek_track",
			"track":  0,
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("expected error or IsError=true for seek_track with track 0, got success")
	}

	// 5. Seek time
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "seek_time",
			"target": "0:02:15",
		},
	})
	if err != nil {
		t.Fatalf("CallTool seek_time failed: %v", err)
	}
	if mock.lastSeekTarget != "0:02:15" {
		t.Errorf("expected lastSeekTarget '0:02:15', got %s", mock.lastSeekTarget)
	}

	// 6. Seek time invalid (empty target)
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "seek_time",
			"target": "",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("expected error or IsError=true for seek_time with empty target, got success")
	}

	// 7. Invalid action
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
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
			"ip":     "192.168.1.120",
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
			"ip":    "192.168.1.120",
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
			"ip":     "192.168.1.120",
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
	const followerIP = "192.168.1.98"
	const coordinatorIP = "192.168.1.99"

	follower := &MockClient{
		ip:            followerIP,
		volume:        15,
		state:         "PLAYING", // false-positive transport state of a follower
		trackURI:      "x-rincon:RINCON_000E5800000000001",
		coordinatorIP: coordinatorIP,
	}
	coordinator := &MockClient{
		ip:       coordinatorIP,
		volume:   22,
		state:    "STOPPED",
		title:    "Poison",
		artist:   "Alice Cooper",
		nrTracks: 8,
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
	if np.QueueLength != 8 {
		t.Errorf("expected coordinator QueueLength 8, got %d", np.QueueLength)
	}
}

// TestSonosGetTopologyTool verifies the topology tool surfaces group/stereo-pair
// structure with coordinator identification. Regression for control-rs9.
func TestSonosGetTopologyTool(t *testing.T) {
	mock := &MockClient{
		ip: "192.168.1.99",
		zoneGroupState: sonos.ZoneGroupState{
			Groups: []sonos.ZoneGroup{
				{
					ID:          "RINCON_000E5800000000001:1",
					Coordinator: "RINCON_000E5800000000001",
					Members: []sonos.ZoneGroupMember{
						{UUID: "RINCON_000E5800000000001", RoomName: "Office", Location: "http://192.168.1.99:1400/xml/device_description.xml"},
						{UUID: "RINCON_000E5800000000002", RoomName: "Office", Location: "http://192.168.1.98:1400/xml/device_description.xml"},
					},
				},
				{
					ID:          "RINCON_ABC:2",
					Coordinator: "RINCON_ABC",
					Members: []sonos.ZoneGroupMember{
						{UUID: "RINCON_ABC", RoomName: "Kitchen", Location: "http://192.168.1.50:1400/xml/device_description.xml"},
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
			"ip": "192.168.1.99",
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
			if m.IP != "192.168.1.99" {
				t.Errorf("expected coordinator IP 192.168.1.99, got %q", m.IP)
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


func TestSonosListFavoritesTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_favorites",
		Arguments: map[string]any{"ip": "192.168.1.120"},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_favorites failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("expected at least 2 content items, got %d", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var favResult ListFavoritesResult
	if err := json.Unmarshal([]byte(textContent.Text), &favResult); err != nil {
		t.Fatalf("failed to unmarshal favorites JSON: %v", err)
	}

	if favResult.Count != 2 || len(favResult.Favorites) != 2 {
		t.Fatalf("expected 2 favorites, got %+v", favResult)
	}
	if favResult.Favorites[0].Title != "Chill Vibes" {
		t.Errorf("expected first favorite 'Chill Vibes', got %s", favResult.Favorites[0].Title)
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be a record, got %T", res.StructuredContent)
		}
	}
}

func TestSonosPlayFavoriteTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120", state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_play_favorite",
		Arguments: map[string]any{
			"ip":          "192.168.1.120",
			"favorite_id": "FV:2/1",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_play_favorite failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}
	if mock.state != "PLAYING" {
		t.Errorf("expected mock state PLAYING, got %s", mock.state)
	}
}

func TestSonosPlayStreamTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120", state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Valid stream
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_play_stream",
		Arguments: map[string]any{
			"ip":    "192.168.1.120",
			"url":   "https://stream.example.com/live.mp3",
			"title": "Live Radio",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_play_stream failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}
	if mock.state != "PLAYING" || mock.title != "Live Radio" {
		t.Errorf("unexpected state: state=%s, title=%s", mock.state, mock.title)
	}

	// 2. Invalid scheme (ftp)
	badRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_play_stream",
		Arguments: map[string]any{
			"ip":  "192.168.1.120",
			"url": "ftp://example.com/audio.mp3",
		},
	})
	if err == nil && (badRes == nil || !badRes.IsError) {
		t.Error("expected error for invalid ftp scheme, got success")
	}
}

func TestSonosAddToQueueTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	const testMetadata = "<DIDL-Lite><item><dc:title>Test Track</dc:title></item></DIDL-Lite>"
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_add_to_queue",
		Arguments: map[string]any{
			"ip":       "192.168.1.120",
			"uri":      "x-rincon-cpcontainer:1006004cALkSOiEkjznR2U-hY1gZPXICcnXWetzSRIrNhw?sid=284&flags=76&sn=2",
			"metadata": testMetadata,
			"as_next":  true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_add_to_queue failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}
	if mock.lastEnqueuedMetadata != testMetadata {
		t.Errorf("expected metadata %q, got %q", testMetadata, mock.lastEnqueuedMetadata)
	}
}

func TestSonosListServicesTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_services",
		Arguments: map[string]any{"ip": "192.168.1.120"},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_services failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("expected at least 2 content items, got %d", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var svcResult ListServicesResult
	if err := json.Unmarshal([]byte(textContent.Text), &svcResult); err != nil {
		t.Fatalf("failed to unmarshal services JSON: %v", err)
	}

	if svcResult.Count != 2 || len(svcResult.Services) != 2 {
		t.Fatalf("expected 2 services, got %+v", svcResult)
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be a record, got %T", res.StructuredContent)
		}
	}
}

func TestSonosGetQueueTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_get_queue",
		Arguments: map[string]any{
			"ip":    "192.168.1.120",
			"start": 0,
			"count": 10,
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_get_queue failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %+v", res)
	}

	if len(res.Content) < 2 {
		t.Fatalf("expected at least 2 content items, got %d", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var queueResult sonos.QueueResult
	if err := json.Unmarshal([]byte(textContent.Text), &queueResult); err != nil {
		t.Fatalf("failed to unmarshal queue JSON: %v", err)
	}

	if queueResult.Returned != 2 || queueResult.TotalMatches != 25 {
		t.Errorf("expected returned 2 and total_matches 25, got %+v", queueResult)
	}
	if len(queueResult.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(queueResult.Items))
	}
	if queueResult.Items[0].Title != "Track One" {
		t.Errorf("expected title 'Track One', got %s", queueResult.Items[0].Title)
	}
	if queueResult.Items[0].Position != 1 {
		t.Errorf("expected position 1, got %d", queueResult.Items[0].Position)
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be a record, got %T", res.StructuredContent)
		}
	}
}

func TestSonosQueueEditTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Remove action
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "remove",
			"track":  3,
			"count":  2,
		},
	})
	if err != nil {
		t.Fatalf("CallTool remove failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected remove success, got error: %+v", res)
	}
	if mock.lastRemoveStart != 3 || mock.lastRemoveCount != 2 {
		t.Errorf("expected remove start 3 count 2, got start %d count %d", mock.lastRemoveStart, mock.lastRemoveCount)
	}

	// 2. Remove missing track parameter
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "remove",
			"track":  0,
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("expected error for remove action with track 0, got success")
	}

	// 3. Clear action
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "clear",
		},
	})
	if err != nil {
		t.Fatalf("CallTool clear failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected clear success, got error: %+v", res)
	}
	if !mock.clearQueueCalled {
		t.Error("expected clearQueueCalled to be true")
	}

	// 4. Reorder action with insert_before
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":            "192.168.1.120",
			"action":        "reorder",
			"track":         5,
			"count":         1,
			"insert_before": 2,
		},
	})
	if err != nil {
		t.Fatalf("CallTool reorder failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected reorder success, got error: %+v", res)
	}
	if mock.lastReorderStart != 5 || mock.lastReorderCount != 1 || mock.lastReorderInsert != 2 {
		t.Errorf("expected reorder 5, 1, 2, got %d, %d, %d", mock.lastReorderStart, mock.lastReorderCount, mock.lastReorderInsert)
	}

	// 5. Reorder action with as_next: true
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":      "192.168.1.120",
			"action":  "reorder",
			"track":   8,
			"as_next": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool reorder as_next failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected reorder as_next success, got error: %+v", res)
	}
	// MockClient GetPositionInfo returns default Track 0, so 0 + 1 = 1
	if mock.lastReorderStart != 8 || mock.lastReorderInsert != 1 {
		t.Errorf("expected reorder as_next start 8 insert 1, got start %d insert %d", mock.lastReorderStart, mock.lastReorderInsert)
	}

	// 6. Invalid action
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "unknown",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("expected error for unknown action, got success")
	}

	// 7. Verify structured content is a record
	if res != nil && res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("expected StructuredContent to be a record, got %T", res.StructuredContent)
		}
	}
}

// TestSonosGetTopologyCachedFallback verifies that when the target speaker IP times out or fails
// (e.g. Move/Roam sleeping on battery), sonos_get_topology iterates across other cached speakers.
// Regression for control-d9v.
func TestSonosGetTopologyCachedFallback(t *testing.T) {
	const sleepingIP = "192.168.1.50"
	const activeIP = "192.168.1.100"

	sleepingSpeaker := &MockClient{
		ip:           sleepingIP,
		failTopology: true, // simulates sleeping/unreachable Roam/Move
	}

	activeSpeaker := &MockClient{
		ip:           activeIP,
		failTopology: false,
		zoneGroupState: sonos.ZoneGroupState{
			Groups: []sonos.ZoneGroup{
				{
					ID:          "Group-1",
					Coordinator: "RINCON_ACTIVE",
					Members: []sonos.ZoneGroupMember{
						{
							UUID:             "RINCON_ACTIVE",
							Location:         "http://192.168.1.100:1400/xml/device_description.xml",
							RoomName:         "Living Room",
							IsZoneStandAlone: true,
						},
					},
				},
			},
		},
	}

	factory := func(ip string) ClientInterface {
		if ip == sleepingIP {
			return sleepingSpeaker
		}
		return activeSpeaker
	}

	// Create test session with cache loader providing both devices
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Roam", IP: sleepingIP, RinconID: "RINCON_ROAM", ModelName: "Sonos Roam"},
				{Name: "Living Room", IP: activeIP, RinconID: "RINCON_ACTIVE", ModelName: "Sonos One"},
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
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	// Call topology querying the sleeping IP
	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_get_topology",
		Arguments: map[string]any{
			"ip": sleepingIP,
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_get_topology failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected fallback success, got error: %+v", res)
	}

	if len(res.Content) < 2 {
		t.Fatalf("expected at least 2 content items, got %d", len(res.Content))
	}

	summaryContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(summaryContent.Text, "resolved via cached fallback") {
		t.Errorf("expected summary to mention cached fallback, got %q", summaryContent.Text)
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[1])
	}

	var topo TopologyResult
	if err := json.Unmarshal([]byte(textContent.Text), &topo); err != nil {
		t.Fatalf("failed to unmarshal topology: %v", err)
	}

	if topo.Count != 1 || len(topo.Groups) != 1 || topo.Groups[0].Coordinator != "RINCON_ACTIVE" {
		t.Errorf("unexpected fallback topology result: %+v", topo)
	}
}
