package roborock

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/pion/turn/v4"
)

const (
	DefaultTURNPort     = 3478
	DefaultTURNRelayMin = 49160
	DefaultTURNRelayMax = 49179
	DefaultTURNTTL      = 86400
)

// TURNMode controls WebRTC TURN/STUN provisioning for camera-equipped vacuums (Saros 10R, Saros 20).
type TURNMode string

const (
	TURNModeDisabled TURNMode = "disabled"
	TURNModeEmbedded TURNMode = "embedded"
	TURNModeExternal TURNMode = "external"
)

// TURNConfig configures the STUN/TURN relay advertised by POST /fwapi/createca.
type TURNConfig struct {
	Mode       TURNMode `json:"mode,omitempty"`        // disabled | embedded | external
	BindHost   string   `json:"bind_host,omitempty"`   // default "0.0.0.0"
	Host       string   `json:"host,omitempty"`        // LAN hostname or IP reachable by robot & viewer
	PublicIP   string   `json:"public_ip,omitempty"`   // optional override for embedded relay advertised IPv4
	Port       int      `json:"port,omitempty"`        // default 3478
	RelayMin   uint16   `json:"relay_min,omitempty"`   // default 49160
	RelayMax   uint16   `json:"relay_max,omitempty"`   // default 49179
	Username   string   `json:"username,omitempty"`    // long-term TURN username
	Password   string   `json:"password,omitempty"`    // long-term TURN password
	Realm      string   `json:"realm,omitempty"`       // defaults to Host
	TTL        int      `json:"ttl,omitempty"`         // default 86400
}

// TURNPayload matches the exact JSON shape expected by Saros 10R (a144) / Saros 20 (a288) / Qrevo firmware
// on POST /fwapi/createca (both legacy user/pwd and ICE username/credential fields populated).
type TURNPayload struct {
	URL        string   `json:"url"`
	User       string   `json:"user"`
	Pwd        string   `json:"pwd"`
	TTL        int      `json:"ttl"`
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	Realm      string   `json:"realm"`
}

// TURNRelay manages an optional embedded pure-Go Pion STUN/TURN UDP relay.
type TURNRelay struct {
	cfg     TURNConfig
	logger  core.Logger
	mu      sync.Mutex
	server  *turn.Server
	conn    net.PacketConn
	running bool
	bound   string
}

// NewTURNRelay constructs a TURNRelay from config (applying safe defaults).
func NewTURNRelay(cfg TURNConfig, logger core.Logger) *TURNRelay {
	if cfg.Mode == "" {
		cfg.Mode = TURNModeDisabled
	}
	if cfg.BindHost == "" {
		cfg.BindHost = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = DefaultTURNPort
	}
	if cfg.RelayMin == 0 {
		cfg.RelayMin = DefaultTURNRelayMin
	}
	if cfg.RelayMax == 0 || cfg.RelayMax < cfg.RelayMin {
		cfg.RelayMax = DefaultTURNRelayMax
	}
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultTURNTTL
	}
	if cfg.Realm == "" {
		cfg.Realm = cfg.Host
	}
	if logger == nil {
		logger = core.NewNoOpLogger()
	}
	return &TURNRelay{cfg: cfg, logger: logger}
}

// Enabled reports whether TURN credentials should be advertised by POST /fwapi/createca.
func (r *TURNRelay) Enabled() bool {
	return r != nil && (r.cfg.Mode == TURNModeEmbedded || r.cfg.Mode == TURNModeExternal) && strings.TrimSpace(r.cfg.Host) != ""
}

// Running reports whether the embedded TURN listener is active.
func (r *TURNRelay) Running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Addr returns the bound UDP address when running in embedded mode.
func (r *TURNRelay) Addr() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bound
}

// Config returns the normalized TURN configuration.
func (r *TURNRelay) Config() TURNConfig {
	if r == nil {
		return TURNConfig{Mode: TURNModeDisabled}
	}
	return r.cfg
}

// Payload builds the firmware-compatible TURN/STUN credential object.
func (r *TURNRelay) Payload() TURNPayload {
	turnURL := fmt.Sprintf("turn:%s:%d", r.cfg.Host, r.cfg.Port)
	stunURL := fmt.Sprintf("stun:%s:%d", r.cfg.Host, r.cfg.Port)
	realm := r.cfg.Realm
	if realm == "" {
		realm = r.cfg.Host
	}
	return TURNPayload{
		URL:        turnURL,
		User:       r.cfg.Username,
		Pwd:        r.cfg.Password,
		TTL:        r.cfg.TTL,
		URLs:       []string{turnURL, stunURL},
		Username:   r.cfg.Username,
		Credential: r.cfg.Password,
		Realm:      realm,
	}
}

// Start launches the embedded Pion STUN/TURN UDP server if Mode == TURNModeEmbedded.
func (r *TURNRelay) Start() error {
	if r == nil || r.cfg.Mode != TURNModeEmbedded {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}
	if r.cfg.Username == "" || r.cfg.Password == "" {
		return errors.New("roborock: embedded TURN requires non-empty username and password")
	}

	relayIP, err := resolveRelayIPv4(r.cfg.PublicIP, r.cfg.Host)
	if err != nil {
		return fmt.Errorf("roborock: resolve TURN relay IPv4: %w", err)
	}

	listenAddr := fmt.Sprintf("%s:%d", r.cfg.BindHost, r.cfg.Port)
	udpConn, err := net.ListenPacket("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("roborock: listen TURN UDP %s: %w", listenAddr, err)
	}

	realm := r.cfg.Realm
	if realm == "" {
		realm = r.cfg.Host
	}
	expectedUser := r.cfg.Username
	authKey := turn.GenerateAuthKey(expectedUser, realm, r.cfg.Password)

	srv, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		AuthHandler: func(username, reqRealm string, _ net.Addr) ([]byte, bool) {
			if username == expectedUser && (reqRealm == "" || reqRealm == realm) {
				return authKey, true
			}
			return nil, false
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpConn,
				RelayAddressGenerator: &turn.RelayAddressGeneratorPortRange{
					RelayAddress: relayIP,
					Address:      r.cfg.BindHost,
					MinPort:      r.cfg.RelayMin,
					MaxPort:      r.cfg.RelayMax,
				},
			},
		},
	})
	if err != nil {
		_ = udpConn.Close()
		return fmt.Errorf("roborock: start Pion TURN server: %w", err)
	}

	r.server = srv
	r.conn = udpConn
	r.bound = udpConn.LocalAddr().String()
	r.running = true
	r.logger.Printf("roborock: embedded Pion TURN/STUN relay listening on udp://%s (relay IP %s, ports %d-%d)",
		r.bound, relayIP.String(), r.cfg.RelayMin, r.cfg.RelayMax)
	return nil
}

// Close shuts down the embedded Pion TURN server cleanly.
func (r *TURNRelay) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return nil
	}
	r.running = false
	var err error
	if r.server != nil {
		err = r.server.Close()
		r.server = nil
	}
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	return err
}

func resolveRelayIPv4(explicitIP, host string) (net.IP, error) {
	if ip := net.ParseIP(strings.TrimSpace(explicitIP)); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(host)); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
	}
	if strings.TrimSpace(host) != "" {
		addrs, err := net.LookupIP(strings.TrimSpace(host))
		if err == nil {
			for _, a := range addrs {
				if v4 := a.To4(); v4 != nil {
					return v4, nil
				}
			}
		}
	}
	// Fallback to first non-loopback local IPv4 interface.
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, a := range addrs {
				if ipNet, ok := a.(*net.IPNet); ok {
					if v4 := ipNet.IP.To4(); v4 != nil && !v4.IsLoopback() {
						return v4, nil
					}
				}
			}
		}
	}
	return net.IPv4(127, 0, 0, 1), nil
}
