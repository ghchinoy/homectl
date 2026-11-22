package roborock

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

func startMockL01Vacuum(t *testing.T, localKey string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go handleMockL01Conn(conn, localKey)
		}
	}()
	return ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
	}
}

func handleMockL01Conn(conn net.Conn, localKey string) {
	defer conn.Close()
	buf := make([]byte, 8192)
	var connectNonce uint32
	ackNonce := uint32(0x13572468)
	negotiated := false

	for {
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		opts := CodecOptions{LocalKey: localKey, Prefixed: true, ConnectNonce: connectNonce}
		if negotiated {
			opts.AckNonce = &ackNonce
		}
		msgs, _, err := DecodeMessages(buf[:n], opts)
		if err != nil {
			return
		}
		for _, m := range msgs {
			if m.Protocol == ProtoHelloRequest {
				// Simulate a modern L01-only vacuum: ignore 1.0 hellos, answer L01 hellos.
				if m.Version != VersionL01 {
					continue
				}
				connectNonce = m.Random
				negotiated = true
				resp, _ := EncodeMessage(Message{
					Version:   VersionL01,
					Seq:       m.Seq,
					Random:    ackNonce,
					Timestamp: m.Timestamp,
					Protocol:  ProtoHelloResponse,
				}, CodecOptions{LocalKey: localKey, Prefixed: true})
				_, _ = conn.Write(resp)
				continue
			}
			if m.Protocol == ProtoGeneralRequest && negotiated {
				var env map[string]any
				_ = json.Unmarshal(m.Payload, &env)
				dps, _ := env["dps"].(map[string]any)
				req, ok := extractRPCFromDPS(dps, "101")
				if !ok {
					continue
				}
				id, _ := anyToInt(req["id"])
				method, _ := req["method"].(string)
				var result any = []string{"ok"}
				switch method {
				case "get_status":
					result = []map[string]any{{
						"battery":    94,
						"state":      8,
						"error_code": 0,
						"fan_power":  102,
					}}
				case "get_consumable":
					result = []map[string]any{{"filter_work_time": 1200}}
				case "get_room_mapping":
					result = [][]any{{16, "100"}, {17, "101"}}
				}
				replyInner, _ := json.Marshal(map[string]any{"id": id, "result": result})
				replyEnv, _ := json.Marshal(map[string]any{"t": time.Now().Unix(), "dps": map[string]any{"102": string(replyInner)}})
				replyWire, _ := EncodeMessage(Message{
					Version:   VersionL01,
					Seq:       m.Seq,
					Random:    m.Random,
					Timestamp: uint32(time.Now().Unix()),
					Protocol:  ProtoGeneralResponse,
					Payload:   replyEnv,
				}, CodecOptions{LocalKey: localKey, Prefixed: true, ConnectNonce: connectNonce, AckNonce: &ackNonce})
				_, _ = conn.Write(replyWire)
			}
		}
	}
}

func TestLANFleetPollRoomsAndControl(t *testing.T) {
	localKey := "0123456789abcdef"
	addr, cleanup := startMockL01Vacuum(t, localKey)
	defer cleanup()

	store, err := NewCloudDeviceStore(core.NewMemoryStorage())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveHomeData("user@example.com", "us", &CloudHomeData{
		ID:       1,
		Name:     "Home",
		Products: []CloudHomeProduct{{ID: "p1", Model: "roborock.vacuum.a144"}},
		Devices: []CloudHomeDevice{{
			DUID:      "duid-10r",
			Name:      "Saros 10R",
			LocalKey:  localKey,
			ProductID: "p1",
			FV:        "02.18.04",
		}},
		Rooms: []CloudHomeRoom{
			{ID: 100, Name: "Kitchen"},
			{ID: 101, Name: "Dining"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.UpdateDiscovered([]BroadcastDevice{
		{DUID: "duid-10r", IP: addr, Version: VersionL01},
	})
	if err != nil {
		t.Fatal(err)
	}

	fleet := NewLANFleet(store)
	fleet.DialTimeout = 3 * time.Second

	snaps := fleet.Snapshot(t.Context())
	if len(snaps) != 1 {
		t.Fatalf("Snapshot len: got: %d, want: 1", len(snaps))
	}
	if !snaps[0].Connected || snaps[0].Battery == nil || *snaps[0].Battery != 94 || snaps[0].StateName != "charging" {
		t.Fatalf("Snapshot telemetry mismatch: got: %+v", snaps[0])
	}

	rooms, _, err := fleet.Rooms(t.Context(), "saros")
	if err != nil {
		t.Fatalf("Rooms: %v", err)
	}
	if len(rooms) != 2 || rooms[0].SegmentID != 16 || rooms[0].Name != "Kitchen" {
		t.Fatalf("Rooms mismatch: got: %+v", rooms)
	}

	res, err := fleet.Control(t.Context(), "saros", "find", nil, 1)
	if err != nil {
		t.Fatalf("Control(find): %v", err)
	}
	if string(res.Result) != `["ok"]` {
		t.Fatalf("Control result: got: %s, want: [\"ok\"]", string(res.Result))
	}
}
