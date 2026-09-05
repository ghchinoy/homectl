package core

import (
	"context"
	"time"
)

// Device represents a generic network-discovered device.
type Device struct {
	ID       string            `json:"id"`                 // Unique ID (MAC, Serial, or UDN)
	Name     string            `json:"name"`               // Friendly name
	IP       string            `json:"ip"`                 // IP address
	Provider string            `json:"provider"`           // Provider name (e.g., "sonos", "lutron")
	Type     string            `json:"type"`               // Device type (e.g., "Speaker", "Bridge")
	Model    string            `json:"model"`              // Model name/number
	Extra    map[string]string `json:"extra,omitempty"`    // Provider-specific metadata
}

// DiscoveryProvider defines the interface for a device discovery implementation.
type DiscoveryProvider interface {
	Name() string
	Discover(ctx context.Context) ([]Device, error)
}

// DiscoveryCoordinator coordinates discovery across multiple providers.
type DiscoveryCoordinator interface {
	AddProvider(p DiscoveryProvider)
	DiscoverAll(timeout time.Duration) []Device
}
