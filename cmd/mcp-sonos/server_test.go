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
	zoneGroupState       sonos.ZoneGroupState
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
	playMode             sonos.PlayModeSettings
	crossfade            bool
	lastSetPlayMode      sonos.PlayMode
	lastSetShuffle       *bool
	lastSetRepeat        string
	lastSetCrossfade     *bool
	failPlayMode         bool
	failCrossfade        bool
	lastJoinedRincon     string
	leftGroup            bool
	failJoin             bool
	failLeave            bool
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

func (m *MockClient) GetPlayMode() (sonos.PlayModeSettings, error) {
	if m.failPlayMode {
		return sonos.PlayModeSettings{}, errors.New("play mode read failure")
	}
	if m.playMode.Mode == "" {
		return sonos.PlayModeSettings{Mode: sonos.PlayModeNormal, Shuffle: false, RepeatMode: "off"}, nil
	}
	return m.playMode, nil
}

func (m *MockClient) SetPlayMode(mode sonos.PlayMode) error {
	if m.failPlayMode {
		return errors.New("play mode write failure")
	}
	m.lastSetPlayMode = mode
	m.playMode = sonos.ParsePlayMode(string(mode))
	return nil
}

func (m *MockClient) SetShuffle(enabled bool) error {
	if m.failPlayMode {
		return errors.New("shuffle write failure")
	}
	m.lastSetShuffle = &enabled
	current, _ := m.GetPlayMode()
	newMode := sonos.BuildPlayMode(enabled, current.RepeatMode)
	return m.SetPlayMode(newMode)
}

func (m *MockClient) SetRepeat(repeatMode string) error {
	if m.failPlayMode {
		return errors.New("repeat write failure")
	}
	m.lastSetRepeat = repeatMode
	current, _ := m.GetPlayMode()
	newMode := sonos.BuildPlayMode(current.Shuffle, repeatMode)
	return m.SetPlayMode(newMode)
}

func (m *MockClient) GetCrossfadeMode() (bool, error) {
	if m.failCrossfade {
		return false, errors.New("crossfade read failure")
	}
	return m.crossfade, nil
}

func (m *MockClient) SetCrossfadeMode(enabled bool) error {
	if m.failCrossfade {
		return errors.New("crossfade write failure")
	}
	m.lastSetCrossfade = &enabled
	m.crossfade = enabled
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

func (m *MockClient) Join(coordinatorRincon string) error {
	if m.failJoin {
		return errors.New("join failure")
	}
	m.lastJoinedRincon = coordinatorRincon
	return nil
}

func (m *MockClient) LeaveGroup() error {
	if m.failLeave {
		return errors.New("leave failure")
	}
	m.leftGroup = true
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
	ctx, cancel := context.WithCancel(t.Context())

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Office Speaker", IP: "192.168.1.120", RinconID: "RINCON_001", ModelName: "Sonos One", Generation: "S2", IsRenderer: true},
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_speakers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_speakers failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("len(res.Content) = %d, want at least 2", len(res.Content))
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var listResult ListSpeakersResult
	if err := json.Unmarshal([]byte(textContent.Text), &listResult); err != nil {
		t.Fatalf("failed to unmarshal ListSpeakersResult: %v", err)
	}

	if listResult.Count != 1 || len(listResult.Speakers) != 1 || listResult.Speakers[0].Name != "Office Speaker" {
		t.Errorf("listResult = %+v, want 1 speaker named %q", listResult, "Office Speaker")
	}

	// Verify that structuredContent is an object/record (not a bare array) per SEP-2106
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any (record/object)", res.StructuredContent)
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
		playMode: sonos.PlayModeSettings{
			Mode:       sonos.PlayModeShuffle,
			Shuffle:    true,
			RepeatMode: "all",
		},
		crossfade: true,
	}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var nowPlaying NowPlayingResult
	if err := json.Unmarshal([]byte(textContent.Text), &nowPlaying); err != nil {
		t.Fatalf("failed to unmarshal now playing JSON: %v", err)
	}

	if nowPlaying.Title != "Take Five" || nowPlaying.Artist != "Dave Brubeck" || nowPlaying.Volume != 30 {
		t.Errorf("nowPlaying = %+v, want Take Five by Dave Brubeck at volume 30", nowPlaying)
	}
	if got := nowPlaying.QueueLength; got != 15 {
		t.Errorf("nowPlaying.QueueLength = %d, want 15", got)
	}
	if got := nowPlaying.MediaURI; got != "x-rincon-queue:RINCON_123456#0" {
		t.Errorf("nowPlaying.MediaURI = %q, want %q", got, "x-rincon-queue:RINCON_123456#0")
	}
	if got := nowPlaying.PlayMode; got != "SHUFFLE" {
		t.Errorf("nowPlaying.PlayMode = %q, want %q", got, "SHUFFLE")
	}
	if !nowPlaying.Shuffle {
		t.Errorf("nowPlaying.Shuffle = false, want true")
	}
	if got := nowPlaying.Repeat; got != "all" {
		t.Errorf("nowPlaying.Repeat = %q, want %q", got, "all")
	}
	if !nowPlaying.Crossfade {
		t.Errorf("nowPlaying.Crossfade = false, want true")
	}
}

func TestSonosControlTool(t *testing.T) {
	mock := &MockClient{state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
	if got := mock.state; got != "PLAYING" {
		t.Errorf("mock.state = %q, want %q", got, "PLAYING")
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
	if got := mock.state; got != "PAUSED_PLAYBACK" {
		t.Errorf("mock.state = %q, want %q", got, "PAUSED_PLAYBACK")
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
	if got := mock.lastSeekTrack; got != 4 {
		t.Errorf("mock.lastSeekTrack = %d, want 4", got)
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
		t.Error("CallTool seek_track with track 0 returned success, want error")
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
	if got := mock.lastSeekTarget; got != "0:02:15" {
		t.Errorf("mock.lastSeekTarget = %q, want %q", got, "0:02:15")
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
		t.Error("CallTool seek_time with empty target returned success, want error")
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
		t.Fatal("CallTool with invalid action returned success, want error")
	}
}

func TestSonosSetVolumeTool(t *testing.T) {
	mock := &MockClient{volume: 20}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
	if got := mock.volume; got != 45 {
		t.Errorf("mock.volume = %d, want 45", got)
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
	if got := mock.volume; got != 55 {
		t.Errorf("mock.volume = %d, want 55", got)
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
	if got := mock.volume; got != 100 {
		t.Errorf("mock.volume = %d, want 100", got)
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var np NowPlayingResult
	if err := json.Unmarshal([]byte(textContent.Text), &np); err != nil {
		t.Fatalf("failed to unmarshal now-playing JSON: %v", err)
	}

	if !np.IsFollower {
		t.Errorf("np.IsFollower = false, want true: %+v", np)
	}
	if got := np.CoordinatorIP; got != coordinatorIP {
		t.Errorf("np.CoordinatorIP = %q, want %q", got, coordinatorIP)
	}
	if got := np.IP; got != coordinatorIP {
		t.Errorf("np.IP = %q, want coordinator %q", got, coordinatorIP)
	}
	if got := np.State; got != "STOPPED" {
		t.Errorf("np.State = %q, want %q", got, "STOPPED")
	}
	if np.Title != "Poison" || np.Artist != "Alice Cooper" {
		t.Errorf("np track = %q by %q, want %q by %q", np.Title, np.Artist, "Poison", "Alice Cooper")
	}
	if got := np.QueueLength; got != 8 {
		t.Errorf("np.QueueLength = %d, want 8", got)
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

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var topo TopologyResult
	if err := json.Unmarshal([]byte(textContent.Text), &topo); err != nil {
		t.Fatalf("failed to unmarshal topology JSON: %v", err)
	}

	if topo.Count != 2 || len(topo.Groups) != 2 {
		t.Fatalf("topo.Count = %d (len = %d), want 2 groups: %+v", topo.Count, len(topo.Groups), topo)
	}

	pair := topo.Groups[0]
	if !pair.IsPair {
		t.Errorf("pair.IsPair = false, want true")
	}
	if got := len(pair.Members); got != 2 {
		t.Fatalf("len(pair.Members) = %d, want 2", got)
	}
	var coordFound bool
	for _, m := range pair.Members {
		if m.UUID == pair.Coordinator {
			if !m.IsCoordinator {
				t.Errorf("coordinator member %s IsCoordinator = false, want true", m.UUID)
			}
			if got := m.IP; got != "192.168.1.99" {
				t.Errorf("coordinator member IP = %q, want %q", got, "192.168.1.99")
			}
			coordFound = true
		}
	}
	if !coordFound {
		t.Error("coordinator member not present in pair members")
	}

	if topo.Groups[1].IsPair {
		t.Error("Kitchen group IsPair = true, want false")
	}

	// StructuredContent must be an object/record per SEP-2106.
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any", res.StructuredContent)
		}
	}
}

func TestSonosListFavoritesTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_favorites",
		Arguments: map[string]any{"ip": "192.168.1.120"},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_favorites failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("len(res.Content) = %d, want at least 2", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var favResult ListFavoritesResult
	if err := json.Unmarshal([]byte(textContent.Text), &favResult); err != nil {
		t.Fatalf("failed to unmarshal favorites JSON: %v", err)
	}

	if favResult.Count != 2 || len(favResult.Favorites) != 2 {
		t.Fatalf("favResult.Count = %d, want 2: %+v", favResult.Count, favResult)
	}
	if got := favResult.Favorites[0].Title; got != "Chill Vibes" {
		t.Errorf("Favorites[0].Title = %q, want %q", got, "Chill Vibes")
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any", res.StructuredContent)
		}
	}
}

func TestSonosPlayFavoriteTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120", state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool sonos_play_favorite returned error: %+v", res)
	}
	if got := mock.state; got != "PLAYING" {
		t.Errorf("mock.state = %q, want %q", got, "PLAYING")
	}
}

func TestSonosPlayStreamTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120", state: "STOPPED"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool sonos_play_stream returned error: %+v", res)
	}
	if mock.state != "PLAYING" || mock.title != "Live Radio" {
		t.Errorf("playback state = (%q, %q), want (%q, %q)", mock.state, mock.title, "PLAYING", "Live Radio")
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
		t.Error("CallTool sonos_play_stream with invalid ftp scheme returned success, want error")
	}
}

func TestSonosAddToQueueTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool sonos_add_to_queue returned error: %+v", res)
	}
	if got := mock.lastEnqueuedMetadata; got != testMetadata {
		t.Errorf("mock.lastEnqueuedMetadata = %q, want %q", got, testMetadata)
	}
}

func TestSonosListServicesTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sonos_list_services",
		Arguments: map[string]any{"ip": "192.168.1.120"},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_services failed: %v", err)
	}

	if len(res.Content) < 2 {
		t.Fatalf("len(res.Content) = %d, want at least 2", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var svcResult ListServicesResult
	if err := json.Unmarshal([]byte(textContent.Text), &svcResult); err != nil {
		t.Fatalf("failed to unmarshal services JSON: %v", err)
	}

	if svcResult.Count != 2 || len(svcResult.Services) != 2 {
		t.Fatalf("svcResult.Count = %d, want 2: %+v", svcResult.Count, svcResult)
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any", res.StructuredContent)
		}
	}
}

func TestSonosGetQueueTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool sonos_get_queue returned error: %+v", res)
	}

	if len(res.Content) < 2 {
		t.Fatalf("len(res.Content) = %d, want at least 2", len(res.Content))
	}
	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var queueResult sonos.QueueResult
	if err := json.Unmarshal([]byte(textContent.Text), &queueResult); err != nil {
		t.Fatalf("failed to unmarshal queue JSON: %v", err)
	}

	if queueResult.Returned != 2 || queueResult.TotalMatches != 25 {
		t.Errorf("queueResult returned/matches = (%d, %d), want (2, 25)", queueResult.Returned, queueResult.TotalMatches)
	}
	if got := len(queueResult.Items); got != 2 {
		t.Fatalf("len(queueResult.Items) = %d, want 2", got)
	}
	if got := queueResult.Items[0].Title; got != "Track One" {
		t.Errorf("Items[0].Title = %q, want %q", got, "Track One")
	}
	if got := queueResult.Items[0].Position; got != 1 {
		t.Errorf("Items[0].Position = %d, want 1", got)
	}

	// Verify structuredContent is a record/object (SEP-2106)
	if res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any", res.StructuredContent)
		}
	}
}

func TestSonosQueueEditTool(t *testing.T) {
	mock := &MockClient{ip: "192.168.1.120"}
	session, cleanup := setupTestSession(t, mock)
	defer cleanup()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool remove returned error: %+v", res)
	}
	if mock.lastRemoveStart != 3 || mock.lastRemoveCount != 2 {
		t.Errorf("lastRemoveStart/Count = (%d, %d), want (3, 2)", mock.lastRemoveStart, mock.lastRemoveCount)
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
		t.Error("CallTool remove with track 0 returned success, want error")
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
		t.Fatalf("CallTool clear returned error: %+v", res)
	}
	if !mock.clearQueueCalled {
		t.Error("mock.clearQueueCalled = false, want true")
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
		t.Fatalf("CallTool reorder returned error: %+v", res)
	}
	if mock.lastReorderStart != 5 || mock.lastReorderCount != 1 || mock.lastReorderInsert != 2 {
		t.Errorf("reorder params = (%d, %d, %d), want (5, 1, 2)", mock.lastReorderStart, mock.lastReorderCount, mock.lastReorderInsert)
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
		t.Fatalf("CallTool reorder as_next returned error: %+v", res)
	}
	// MockClient GetPositionInfo returns default Track 0, so 0 + 1 = 1
	if mock.lastReorderStart != 8 || mock.lastReorderInsert != 1 {
		t.Errorf("reorder as_next start/insert = (%d, %d), want (8, 1)", mock.lastReorderStart, mock.lastReorderInsert)
	}

	// 6. Shuffle action (enable)
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":      "192.168.1.120",
			"action":  "shuffle",
			"enabled": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool shuffle failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool shuffle returned error: %+v", res)
	}
	if mock.lastSetShuffle == nil || !*mock.lastSetShuffle {
		t.Errorf("lastSetShuffle = %+v, want true", mock.lastSetShuffle)
	}
	if got := mock.lastSetPlayMode; got != sonos.PlayModeShuffleNoRepeat {
		t.Errorf("lastSetPlayMode = %q, want %q", got, sonos.PlayModeShuffleNoRepeat)
	}

	// 7. Shuffle missing enabled parameter
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "shuffle",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("CallTool shuffle without enabled parameter returned success, want error")
	}

	// 8. Repeat action with repeat_mode: "all"
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":          "192.168.1.120",
			"action":      "repeat",
			"repeat_mode": "all",
		},
	})
	if err != nil {
		t.Fatalf("CallTool repeat failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool repeat returned error: %+v", res)
	}
	if got := mock.lastSetRepeat; got != "all" {
		t.Errorf("lastSetRepeat = %q, want %q", got, "all")
	}
	// Since shuffle was enabled previously, repeat: all -> SHUFFLE
	if got := mock.lastSetPlayMode; got != sonos.PlayModeShuffle {
		t.Errorf("lastSetPlayMode = %q, want %q", got, sonos.PlayModeShuffle)
	}

	// 9. Repeat action with enabled: false (should map to repeat "off")
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":      "192.168.1.120",
			"action":  "repeat",
			"enabled": false,
		},
	})
	if err != nil {
		t.Fatalf("CallTool repeat with enabled: false failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool repeat with enabled: false returned error: %+v", res)
	}
	if got := mock.lastSetRepeat; got != "off" {
		t.Errorf("lastSetRepeat = %q, want %q", got, "off")
	}

	// 10. Repeat action with invalid repeat_mode
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":          "192.168.1.120",
			"action":      "repeat",
			"repeat_mode": "random_mode",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("CallTool repeat with invalid repeat_mode returned success, want error")
	}

	// 11. Crossfade action (enable)
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":      "192.168.1.120",
			"action":  "crossfade",
			"enabled": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool crossfade failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool crossfade returned error: %+v", res)
	}
	if mock.lastSetCrossfade == nil || !*mock.lastSetCrossfade {
		t.Errorf("lastSetCrossfade = %+v, want true", mock.lastSetCrossfade)
	}

	// 12. Crossfade missing enabled parameter
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "crossfade",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("CallTool crossfade without enabled parameter returned success, want error")
	}

	// 13. Invalid action
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "sonos_queue_edit",
		Arguments: map[string]any{
			"ip":     "192.168.1.120",
			"action": "unknown",
		},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Error("CallTool queue edit with unknown action returned success, want error")
	}

	// 14. Verify structured content is a record
	if res != nil && res.StructuredContent != nil {
		if _, ok := res.StructuredContent.(map[string]any); !ok {
			t.Errorf("StructuredContent type = %T, want map[string]any", res.StructuredContent)
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
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Roam", IP: sleepingIP, RinconID: "RINCON_ROAM", ModelName: "Sonos Roam", Generation: "S2", IsRenderer: true},
				{Name: "Living Room", IP: activeIP, RinconID: "RINCON_ACTIVE", ModelName: "Sonos One", Generation: "S2", IsRenderer: true},
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

	callCtx, callCancel := context.WithTimeout(t.Context(), 3*time.Second)
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
		t.Fatalf("CallTool sonos_get_topology fallback returned error: %+v", res)
	}

	if len(res.Content) < 2 {
		t.Fatalf("len(res.Content) = %d, want at least 2", len(res.Content))
	}

	summaryContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(summaryContent.Text, "resolved via cached fallback") {
		t.Errorf("summaryContent.Text = %q, want to contain %q", summaryContent.Text, "resolved via cached fallback")
	}

	textContent, ok := res.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("res.Content[1] type = %T, want *mcp.TextContent", res.Content[1])
	}

	var topo TopologyResult
	if err := json.Unmarshal([]byte(textContent.Text), &topo); err != nil {
		t.Fatalf("failed to unmarshal topology: %v", err)
	}

	if topo.Count != 1 || len(topo.Groups) != 1 || topo.Groups[0].Coordinator != "RINCON_ACTIVE" {
		t.Errorf("unexpected fallback topology result: %+v", topo)
	}
}

func TestSonosControlTool_Join_Success(t *testing.T) {
	mockSpeaker1 := &MockClient{ip: "192.168.1.10", volume: 30, state: "PLAYING"}
	mockSpeaker2 := &MockClient{ip: "192.168.1.11", volume: 25, state: "PLAYING"}

	factory := func(ip string) ClientInterface {
		if ip == "192.168.1.10" {
			return mockSpeaker1
		}
		return mockSpeaker2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Living Room", IP: "192.168.1.10", RinconID: "RINCON_LR", Generation: "S2", IsRenderer: true},
				{Name: "Kitchen", IP: "192.168.1.11", RinconID: "RINCON_KT", Generation: "S2", IsRenderer: true},
			}, nil
		}),
	)

	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	// Kitchen joins Living Room
	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.11",
			"action": "join",
			"target": "192.168.1.10",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_control join failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got tool error: %+v", res)
	}
	if mockSpeaker2.lastJoinedRincon != "RINCON_LR" {
		t.Errorf("expected lastJoinedRincon = RINCON_LR, got %q", mockSpeaker2.lastJoinedRincon)
	}

	// Kitchen leaves group
	res, err = session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.11",
			"action": "leave",
		},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_control leave failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success on leave, got tool error: %+v", res)
	}
	if !mockSpeaker2.leftGroup {
		t.Errorf("expected leftGroup to be true")
	}
}

func TestSonosControlTool_Join_CrossGeneration_Rejection(t *testing.T) {
	mockSpeakerS1 := &MockClient{ip: "192.168.1.10", volume: 30, state: "PLAYING"}
	mockSpeakerS2 := &MockClient{ip: "192.168.1.20", volume: 25, state: "PLAYING"}

	factory := func(ip string) ClientInterface {
		if ip == "192.168.1.10" {
			return mockSpeakerS1
		}
		return mockSpeakerS2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Play:5 Gen 1", IP: "192.168.1.10", RinconID: "RINCON_S1", Generation: "S1", IsRenderer: true},
				{Name: "Sonos One", IP: "192.168.1.20", RinconID: "RINCON_S2", Generation: "S2", IsRenderer: true},
			}, nil
		}),
	)

	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	// S1 attempts to join S2
	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.10",
			"action": "join",
			"target": "192.168.1.20",
		},
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error when cross-grouping S1 and S2, got success")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(textContent.Text, "cannot group S1 speaker") {
		t.Errorf("expected error message to mention 'cannot group S1 speaker', got %v", res.Content)
	}
}

func TestSonosControlTool_NonRenderer_Rejection(t *testing.T) {
	mockBridge := &MockClient{ip: "192.168.1.50"}

	factory := func(ip string) ClientInterface {
		return mockBridge
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithClientFactory(factory),
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Bridge", IP: "192.168.1.50", RinconID: "RINCON_BRIDGE", ModelName: "Sonos Bridge", Generation: "S1", IsRenderer: false},
			}, nil
		}),
	)

	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	// Play on bridge should be rejected
	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_control",
		Arguments: map[string]any{
			"ip":     "192.168.1.50",
			"action": "play",
		},
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error when playing on bridge, got success")
	}
	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(textContent.Text, "non-rendering device") {
		t.Errorf("expected error to mention 'non-rendering device', got %v", res.Content)
	}

	// Volume on bridge should also be rejected
	res, err = session.CallTool(callCtx, &mcp.CallToolParams{
		Name: "sonos_set_volume",
		Arguments: map[string]any{
			"ip":     "192.168.1.50",
			"volume": 30,
		},
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error when setting volume on bridge, got success")
	}
}

func TestSonosListSpeakersTool_IncludesGenerationAndRenderer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := CreateMCPServer(
		WithCacheLoader(func() ([]sonos.Device, error) {
			return []sonos.Device{
				{Name: "Living Room", IP: "192.168.1.10", RinconID: "RINCON_LR", ModelName: "Sonos One", Generation: "S2", IsRenderer: true},
				{Name: "Bridge", IP: "192.168.1.11", RinconID: "RINCON_BR", ModelName: "Sonos Bridge", Generation: "S1", IsRenderer: false},
			}, nil
		}),
	)

	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer callCancel()

	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      "sonos_list_speakers",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool sonos_list_speakers failed: %v", err)
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

	var listRes ListSpeakersResult
	if err := json.Unmarshal([]byte(textContent.Text), &listRes); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if listRes.Count != 2 {
		t.Fatalf("expected count 2, got %d", listRes.Count)
	}

	if listRes.Speakers[0].Generation != "S2" || !listRes.Speakers[0].IsRenderer {
		t.Errorf("expected speaker 0 to have Generation=S2 and IsRenderer=true, got %+v", listRes.Speakers[0])
	}
	if listRes.Speakers[1].Generation != "S1" || listRes.Speakers[1].IsRenderer {
		t.Errorf("expected speaker 1 to have Generation=S1 and IsRenderer=false, got %+v", listRes.Speakers[1])
	}
}
