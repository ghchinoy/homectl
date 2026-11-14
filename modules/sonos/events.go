package sonos

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

const maxGENABodyBytes = 1 << 20 // 1 MiB cap on inbound UPnP NOTIFY XML bodies

// EventMsg represents a processed event from a Sonos device
type EventMsg struct {
	IP           string
	Volume       int
	Status       string
	Metadata     TrackMetadata
	NextMetadata TrackMetadata
}

// LastChangeParser parses the complex XML fragment sent by Sonos in GENA events
type LastChangeParser struct {
	XMLName xml.Name `xml:"Event"`
	Volume  []struct {
		Val int `xml:"val,attr"`
	} `xml:"InstanceID>Volume"`
	TransportState []struct {
		Val string `xml:"val,attr"`
	} `xml:"InstanceID>TransportState"`
	CurrentTrackMetaData []struct {
		Val string `xml:"val,attr"`
	} `xml:"InstanceID>CurrentTrackMetaData"`
	NextTrackMetaData []struct {
		Val string `xml:"val,attr"`
	} `xml:"InstanceID>NextTrackMetaData"`
}

// GENAListener handles incoming UPnP notifications.
type GENAListener struct {
	Port     int
	Handler  func(EventMsg)
	Logger   core.Logger
	Settings core.Settings

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
}

func (l *GENAListener) getLogger() core.Logger {
	if l != nil && l.Logger != nil {
		return l.Logger
	}
	return defaultLogger
}

func (l *GENAListener) getSettings() core.Settings {
	if l != nil && l.Settings != nil {
		return l.Settings
	}
	return defaultSettings
}

// Start starts the HTTP listener on a random available port.
func (l *GENAListener) Start() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return "", err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handleNotify)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	l.mu.Lock()
	l.Port = listener.Addr().(*net.TCPAddr).Port
	l.listener = listener
	l.server = srv
	l.mu.Unlock()

	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			l.getLogger().Printf("GENA server error: %v\n", serveErr)
		}
	}()

	return l.CallbackURL(), nil
}

// Stop gracefully shuts down the GENA HTTP listener using the provided context.
func (l *GENAListener) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	srv := l.server
	ln := l.listener
	l.server = nil
	l.listener = nil
	l.mu.Unlock()

	if srv != nil {
		return srv.Shutdown(ctx)
	}
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// Close shuts down the GENA HTTP listener with a bounded 2-second timeout.
func (l *GENAListener) Close() error {
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()
	return l.Stop(ctx)
}

// CallbackURL returns the HTTP callback URL for GENA event notifications.
func (l *GENAListener) CallbackURL() string {
	settings := l.getSettings()
	logger := l.getLogger()
	if cb := settings.CallbackIP(); cb != "" {
		logger.Printf("Using Callback IP from config: %s\n", cb)
		return fmt.Sprintf("http://%s:%d", cb, l.Port)
	}

	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ipStr := ipnet.IP.String()
			logger.Printf("IP Discovery Candidate: %s\n", ipStr)
			if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil && strings.HasPrefix(ipStr, "192.168.") {
				return fmt.Sprintf("http://%s:%d", ipStr, l.Port)
			}
		}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ipStr := ipnet.IP.String()
			if ipnet.IP.To4() != nil {
				return fmt.Sprintf("http://%s:%d", ipStr, l.Port)
			}
		}
	}
	return ""
}

// GetLocalIP returns the callback URL.
// Deprecated: use CallbackURL instead.
func (l *GENAListener) GetLocalIP() string {
	return l.CallbackURL()
}

func (l *GENAListener) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != "NOTIFY" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxGENABodyBytes)
	body, err := io.ReadAll(io.LimitReader(r.Body, maxGENABodyBytes+1))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "notify payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	l.getLogger().Printf("NOTIFY from %s\n", ip)

	var outer struct {
		LastChange string `xml:"property>LastChange"`
	}
	if err := xml.Unmarshal(body, &outer); err != nil {
		l.getLogger().Printf("XML Unmarshal Error (Outer): %v\n", err)
	}

	msg := EventMsg{IP: ip, Volume: -1}

	if outer.LastChange != "" {
		var lc LastChangeParser
		if err := xml.Unmarshal([]byte(outer.LastChange), &lc); err != nil {
			l.getLogger().Printf("XML Unmarshal Error (LastChange): %v\n", err)
		}

		if len(lc.Volume) > 0 {
			msg.Volume = lc.Volume[0].Val
		}
		if len(lc.TransportState) > 0 {
			msg.Status = lc.TransportState[0].Val
		}
		if len(lc.CurrentTrackMetaData) > 0 && lc.CurrentTrackMetaData[0].Val != "" {
			meta, err := ParseTrackMetadata(lc.CurrentTrackMetaData[0].Val)
			if err == nil {
				msg.Metadata = meta
			}
		}
		if len(lc.NextTrackMetaData) > 0 && lc.NextTrackMetaData[0].Val != "" {
			meta, err := ParseTrackMetadata(lc.NextTrackMetaData[0].Val)
			if err == nil {
				msg.NextMetadata = meta
			}
		}
	}

	if l.Handler != nil {
		l.Handler(msg)
	}

	w.WriteHeader(http.StatusOK)
}
