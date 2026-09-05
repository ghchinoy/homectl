// Package discovery provides a coordinated registry and scanner for IoT devices across multiple protocols.
package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

// Device represents a generic network-discovered device
type Device = core.Device

// Provider defines the interface for a device discovery implementation
type Provider = core.DiscoveryProvider

// Manager coordinates multiple discovery providers
type Manager struct {
	providers []Provider
}

// NewManager creates a new discovery manager
func NewManager() *Manager {
	return &Manager{
		providers: make([]Provider, 0),
	}
}

// AddProvider registers a new discovery provider
func (m *Manager) AddProvider(p Provider) {
	m.providers = append(m.providers, p)
}

// DiscoverAll runs discovery on all registered providers concurrently
func (m *Manager) DiscoverAll(timeout time.Duration) []Device {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan []Device, len(m.providers))

	for _, p := range m.providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			devices, err := p.Discover(ctx)
			if err != nil {
				// We log or ignore errors from individual providers to keep the process going
				results <- nil
				return
			}
			results <- devices
		}(p)
	}

	// Closer
	go func() {
		wg.Wait()
		close(results)
	}()

	var allDevices []Device
	for devices := range results {
		if devices != nil {
			allDevices = append(allDevices, devices...)
		}
	}

	return allDevices
}
