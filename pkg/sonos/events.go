package sonos

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/ghchinoy/control/pkg/config"
)

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

// GENAListener handles incoming UPnP notifications
type GENAListener struct {
	Port    int
	Handler func(EventMsg)
}

// Start starts the HTTP listener on a random available port
func (l *GENAListener) Start() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return "", err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}

	l.Port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/", l.handleNotify)

	go http.Serve(listener, mux)

	return l.GetLocalIP(), nil
}

func (l *GENAListener) GetLocalIP() string {
	cfg := config.LoadConfig()
	if cfg.CallbackIP != "" {
		if sonosLogger != nil {
			sonosLogger.Printf("Using Callback IP from config: %s\n", cfg.CallbackIP)
		}
		return fmt.Sprintf("http://%s:%d", cfg.CallbackIP, l.Port)
	}

	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ipStr := ipnet.IP.String()
			if sonosLogger != nil {
				sonosLogger.Printf("IP Discovery Candidate: %s\n", ipStr)
			}
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

func (l *GENAListener) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != "NOTIFY" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	ip := strings.Split(r.RemoteAddr, ":")[0]
	
	if sonosLogger != nil {
		sonosLogger.Printf("NOTIFY from %s\n", ip)
	}

	var outer struct {
		LastChange string `xml:"property>LastChange"`
	}
	if err := xml.Unmarshal(body, &outer); err != nil {
		if sonosLogger != nil {
			sonosLogger.Printf("XML Unmarshal Error (Outer): %v\n", err)
		}
	}

	msg := EventMsg{IP: ip, Volume: -1}

	if outer.LastChange != "" {
		var lc LastChangeParser
		if err := xml.Unmarshal([]byte(outer.LastChange), &lc); err != nil {
			if sonosLogger != nil {
				sonosLogger.Printf("XML Unmarshal Error (LastChange): %v\n", err)
			}
		}

		if len(lc.Volume) > 0 {
			msg.Volume = lc.Volume[0].Val
		}
		if len(lc.TransportState) > 0 {
			msg.Status = lc.TransportState[0].Val
		}
		if len(lc.CurrentTrackMetaData) > 0 && lc.CurrentTrackMetaData[0].Val != "" {
			c := &Client{}
			meta, err := c.ParseTrackMetadata(lc.CurrentTrackMetaData[0].Val)
			if err == nil {
				msg.Metadata = meta
			}
		}
		if len(lc.NextTrackMetaData) > 0 && lc.NextTrackMetaData[0].Val != "" {
			c := &Client{}
			meta, err := c.ParseTrackMetadata(lc.NextTrackMetaData[0].Val)
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