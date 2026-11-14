package qolsys

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestClient_ConnectSendAndReadLoop(t *testing.T) {
	var receivedMu sync.Mutex
	var serverReceived map[string]any

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		var incoming map[string]any
		if err := wsjson.Read(r.Context(), conn, &incoming); err != nil {
			return
		}
		receivedMu.Lock()
		serverReceived = incoming
		receivedMu.Unlock()

		// Push a synthetic Qolsys zone event back to the client
		_ = wsjson.Write(r.Context(), conn, map[string]any{
			"event":     "ZONE_EVENT",
			"zone_type": "DOOR_WINDOW",
			"status":    "OPEN",
		})

		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "https://")
	client := NewClient(addr, "654321")

	// Send before Connect must fail cleanly
	if err := client.Send(t.Context(), "INFO", nil); err == nil {
		t.Fatal("client.Send(disconnected) = nil, want error")
	}

	eventCh := make(chan map[string]any, 1)
	client.OnEvent = func(msg map[string]any) {
		select {
		case eventCh <- msg:
		default:
		}
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("client.Connect(%q) failed: %v", addr, err)
	}
	defer client.Close()

	if err := client.Send(ctx, "INFO", map[string]any{"partition_id": 0}); err != nil {
		t.Fatalf("client.Send(INFO) failed: %v", err)
	}

	// ReadLoop reads the ZONE_EVENT and then returns on normal closure
	_ = client.ReadLoop(ctx)

	select {
	case ev := <-eventCh:
		if ev["event"] != "ZONE_EVENT" || ev["status"] != "OPEN" {
			t.Errorf("client.ReadLoop(event) = %+v, want ZONE_EVENT OPEN", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client.ReadLoop(event) timed out waiting for OnEvent")
	}

	receivedMu.Lock()
	gotAction := serverReceived["action"]
	gotPin := serverReceived["user_pin"]
	gotSource := serverReceived["source"]
	receivedMu.Unlock()

	if gotAction != "INFO" || gotPin != "654321" || gotSource != "homectl" {
		t.Errorf("client.Send(INFO) server received (%v, %v, %v), want (INFO, 654321, homectl)",
			gotAction, gotPin, gotSource)
	}
}
