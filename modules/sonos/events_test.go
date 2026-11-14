package sonos

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

func TestGENAListener_LifecycleAndSecurity(t *testing.T) {
	var mu sync.Mutex
	var received []EventMsg

	listener := &GENAListener{
		Logger:   core.NewNoOpLogger(),
		Settings: core.NewStaticSettings("127.0.0.1", ""),
		Handler: func(ev EventMsg) {
			mu.Lock()
			received = append(received, ev)
			mu.Unlock()
		},
	}

	cbURL, err := listener.Start()
	if err != nil {
		t.Fatalf("listener.Start(127.0.0.1) failed: %v", err)
	}
	defer listener.Close()

	if !strings.HasPrefix(cbURL, "http://127.0.0.1:") {
		t.Fatalf("listener.Start(127.0.0.1) callback = %q, want http://127.0.0.1:<port>", cbURL)
	}

	client := &http.Client{Timeout: 2 * time.Second}

	// 1. Non-NOTIFY method rejected with 405
	getReq, _ := http.NewRequest(http.MethodGet, cbURL, nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("client.Do(GET) failed: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GENAListener(GET) status = %d, want %d", getResp.StatusCode, http.StatusMethodNotAllowed)
	}

	// 2. Valid NOTIFY event parsed and dispatched
	notifyXML := `<e:propertyset xmlns:e="urn:schemas-upnp-org:event-1-0">
  <e:property>
    <LastChange>&lt;Event xmlns=&quot;urn:schemas-upnp-org:metadata-1-0/RCS/&quot;&gt;&lt;InstanceID val=&quot;0&quot;&gt;&lt;Volume channel=&quot;Master&quot; val=&quot;33&quot;/&gt;&lt;TransportState val=&quot;PLAYING&quot;/&gt;&lt;/InstanceID&gt;&lt;/Event&gt;</LastChange>
  </e:property>
</e:propertyset>`
	notifyReq, _ := http.NewRequest("NOTIFY", cbURL, strings.NewReader(notifyXML))
	notifyResp, err := client.Do(notifyReq)
	if err != nil {
		t.Fatalf("client.Do(NOTIFY) failed: %v", err)
	}
	notifyResp.Body.Close()
	if notifyResp.StatusCode != http.StatusOK {
		t.Errorf("GENAListener(NOTIFY) status = %d, want %d", notifyResp.StatusCode, http.StatusOK)
	}

	mu.Lock()
	gotCount := len(received)
	var first EventMsg
	if gotCount > 0 {
		first = received[0]
	}
	mu.Unlock()

	if gotCount != 1 || first.Volume != 33 || first.Status != "PLAYING" {
		t.Errorf("GENAListener(NOTIFY) event = %+v (count=%d), want Volume=33 Status=PLAYING", first, gotCount)
	}

	// 3. Oversized NOTIFY body (> 1 MiB) rejected with 413
	hugeXML := fmt.Sprintf("<e:propertyset>%s</e:propertyset>", strings.Repeat("x", maxGENABodyBytes+4096))
	hugeReq, _ := http.NewRequest("NOTIFY", cbURL, strings.NewReader(hugeXML))
	hugeResp, err := client.Do(hugeReq)
	if err != nil {
		t.Fatalf("client.Do(oversized NOTIFY) failed: %v", err)
	}
	hugeResp.Body.Close()
	if hugeResp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("GENAListener(oversized NOTIFY) status = %d, want %d", hugeResp.StatusCode, http.StatusRequestEntityTooLarge)
	}

	// 4. Close shuts down the listener cleanly
	if err := listener.Close(); err != nil {
		t.Errorf("listener.Close(active) = %v, want nil", err)
	}
}
