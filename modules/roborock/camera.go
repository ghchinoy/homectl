package roborock

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	// MaxCameraBootstrapLen is the firmware truncation boundary (including optional :port and trailing '/').
	MaxCameraBootstrapLen = 15
	// MaxTokenRLen is the maximum onboarding token.r length including trailing '/'.
	MaxTokenRLen = 32
)

var validHostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// CameraBootstrapEndpoint is a resolved host:port that a camera-equipped vacuum will hit for POST /fwapi/createca.
type CameraBootstrapEndpoint struct {
	InputName string `json:"input_name"` // "token.r" or "country_domain"
	Host      string `json:"host"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
}

// ValidateCameraBootstrapInput checks that an onboarding field (token.r or country_domain)
// fits within Roborock's 15-character firmware limit including any :port and trailing '/'.
func ValidateCameraBootstrapInput(name, value string) (CameraBootstrapEndpoint, error) {
	if !strings.HasSuffix(value, "/") || len(value) > MaxCameraBootstrapLen {
		return CameraBootstrapEndpoint{}, fmt.Errorf(
			"roborock: camera %s %q must end with '/' and be <= %d characters (got %d); use a shorter hostname or port 443",
			name, value, MaxCameraBootstrapLen, len(value),
		)
	}
	parsed, err := url.Parse("https://" + value)
	if err != nil {
		return CameraBootstrapEndpoint{}, fmt.Errorf("roborock: invalid camera %s %q: %w", name, value, err)
	}
	if parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return CameraBootstrapEndpoint{}, fmt.Errorf("roborock: camera %s %q must contain only host[:port]/", name, value)
	}
	host := parsed.Hostname()
	if host == "" || !validHostnameRe.MatchString(host) {
		return CameraBootstrapEndpoint{}, fmt.Errorf("roborock: camera %s has invalid hostname %q", name, host)
	}
	port := 443
	if pStr := parsed.Port(); pStr != "" {
		p, err := strconv.Atoi(pStr)
		if err != nil || p <= 0 || p > 65535 {
			return CameraBootstrapEndpoint{}, fmt.Errorf("roborock: invalid port in camera %s %q", name, value)
		}
		port = p
	}
	return CameraBootstrapEndpoint{
		InputName: name,
		Host:      host,
		Port:      port,
		URL:       fmt.Sprintf("https://%s:%d/iot.roborock.com/fwapi/createca", host, port),
	}, nil
}

// CameraBootstrapEndpoints returns the exact endpoints required for a model's WebRTC camera bootstrap.
// Saros 10R (a144) uses country_domain; Saros 20 (a288) uses token.r; unknown models require both.
func CameraBootstrapEndpoints(model, tokenR, countryDomain string) ([]CameraBootstrapEndpoint, error) {
	mode := CameraBootstrapForModel(model)
	type pair struct{ name, val string }
	var inputs []pair
	switch mode {
	case CameraBootstrapCountryDomain:
		inputs = []pair{{"country_domain", countryDomain}}
	case CameraBootstrapTokenR:
		inputs = []pair{{"token.r", tokenR}}
	default:
		inputs = []pair{{"token.r", tokenR}, {"country_domain", countryDomain}}
	}

	var out []CameraBootstrapEndpoint
	seen := make(map[string]bool)
	for _, in := range inputs {
		ep, err := ValidateCameraBootstrapInput(in.name, in.val)
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
		if !seen[key] {
			seen[key] = true
			out = append(out, ep)
		}
	}
	return out, nil
}

// CameraInfo summarizes WebRTC camera configuration and go2rtc integration guidance for a vacuum.
type CameraInfo struct {
	DID                string                    `json:"did"`
	DUID               string                    `json:"duid"`
	Name               string                    `json:"name"`
	Model              string                    `json:"model"`
	BootstrapSource    CameraBootstrapMode       `json:"bootstrap_source"`
	TURNMode           TURNMode                  `json:"turn_mode"`
	TURNRunning        bool                      `json:"turn_running"`
	TURNEndpoint       string                    `json:"turn_endpoint,omitempty"`
	BootstrapEndpoints []CameraBootstrapEndpoint `json:"bootstrap_endpoints,omitempty"`
	Notes              string                    `json:"notes,omitempty"`
}

// BuildCameraInfo assembles camera diagnostics for a device.
func BuildCameraInfo(dev DeviceCredential, turn *TURNRelay, stackTokenR, countryDomain string) CameraInfo {
	mode := CameraBootstrapForModel(dev.Model)
	info := CameraInfo{
		DID:             dev.DID,
		DUID:            dev.DUID,
		Name:            dev.Name,
		Model:           dev.Model,
		BootstrapSource: mode,
		TURNMode:        TURNModeDisabled,
	}
	if turn != nil {
		cfg := turn.Config()
		info.TURNMode = cfg.Mode
		info.TURNRunning = turn.Running()
		if turn.Enabled() {
			info.TURNEndpoint = fmt.Sprintf("turn:%s:%d", cfg.Host, cfg.Port)
		}
	}
	if stackTokenR != "" || countryDomain != "" {
		if eps, err := CameraBootstrapEndpoints(dev.Model, stackTokenR, countryDomain); err == nil {
			info.BootstrapEndpoints = eps
		}
	}
	switch mode {
	case CameraBootstrapCountryDomain:
		info.Notes = "Saros 10R (a144): uses country_domain (<=15 chars with '/') on port 443 for /fwapi/createca. Remote view requires setting PIN (set_homesec_password) and two-key physical Power button activation."
	case CameraBootstrapTokenR:
		info.Notes = "Saros 20 (a288): uses token.r (<=15 chars including port and '/') for /fwapi/createca. Block WAN access at router so device obeys local DHCP DNS instead of 8.8.8.8."
	default:
		info.Notes = "Model camera bootstrap behavior not explicitly mapped; ensure both token.r and country_domain fit <=15 chars if camera is supported."
	}
	return info
}
