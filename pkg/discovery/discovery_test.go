package discovery

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type mockProvider struct {
	name     string
	devices  []Device
	err      error
	delay    time.Duration
	called   atomic.Bool
	canceled atomic.Bool
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Discover(ctx context.Context) ([]Device, error) {
	m.called.Store(true)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			m.canceled.Store(true)
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.devices, nil
}

func TestManager_DiscoverAllContext(t *testing.T) {
	t.Run("empty manager returns empty", func(t *testing.T) {
		m := NewManager()
		devices := m.DiscoverAllContext(t.Context(), 100*time.Millisecond)
		if len(devices) != 0 {
			t.Errorf("got %d devices, want 0", len(devices))
		}
	})

	t.Run("aggregates devices from multiple providers", func(t *testing.T) {
		m := NewManager()
		p1 := &mockProvider{
			name: "provider1",
			devices: []Device{
				{ID: "d1", Name: "Device 1", IP: "192.168.1.10", Provider: "provider1"},
			},
		}
		p2 := &mockProvider{
			name: "provider2",
			devices: []Device{
				{ID: "d2", Name: "Device 2", IP: "192.168.1.20", Provider: "provider2"},
			},
		}
		m.AddProvider(p1)
		m.AddProvider(p2)

		devices := m.DiscoverAllContext(t.Context(), 200*time.Millisecond)
		if got, want := len(devices), 2; got != want {
			t.Fatalf("got %d devices, want %d", got, want)
		}

		found := make(map[string]bool)
		for _, d := range devices {
			found[d.ID] = true
		}
		if !found["d1"] || !found["d2"] {
			t.Errorf("missing expected devices: %v", found)
		}
	})

	t.Run("fault tolerant when provider returns error", func(t *testing.T) {
		m := NewManager()
		p1 := &mockProvider{
			name: "failing",
			err:  errors.New("network failure"),
		}
		p2 := &mockProvider{
			name: "working",
			devices: []Device{
				{ID: "d2", Name: "Device 2", IP: "192.168.1.20", Provider: "working"},
			},
		}
		m.AddProvider(p1)
		m.AddProvider(p2)

		devices := m.DiscoverAllContext(t.Context(), 200*time.Millisecond)
		if got, want := len(devices), 1; got != want {
			t.Fatalf("got %d devices, want %d", got, want)
		}
		if got, want := devices[0].ID, "d2"; got != want {
			t.Errorf("got device ID %q, want %q", got, want)
		}
	})

	t.Run("honors context timeout and cancels slow providers", func(t *testing.T) {
		m := NewManager()
		slowProvider := &mockProvider{
			name:  "slow",
			delay: 200 * time.Millisecond,
		}
		fastProvider := &mockProvider{
			name: "fast",
			devices: []Device{
				{ID: "fast-1", Name: "Fast Device", IP: "192.168.1.30", Provider: "fast"},
			},
		}
		m.AddProvider(slowProvider)
		m.AddProvider(fastProvider)

		start := time.Now()
		devices := m.DiscoverAllContext(t.Context(), 50*time.Millisecond)
		elapsed := time.Since(start)

		if elapsed > 150*time.Millisecond {
			t.Errorf("DiscoverAllContext took %v, expected <= 150ms", elapsed)
		}

		if got, want := len(devices), 1; got != want {
			t.Errorf("got %d devices, want %d", got, want)
		}
		if !slowProvider.canceled.Load() {
			t.Errorf("expected slow provider to be canceled")
		}
	})

	t.Run("DiscoverAll convenience wrapper works", func(t *testing.T) {
		m := NewManager()
		p := &mockProvider{
			name: "p",
			devices: []Device{
				{ID: "d", Name: "Device", IP: "192.168.1.1", Provider: "p"},
			},
		}
		m.AddProvider(p)
		devices := m.DiscoverAll(100 * time.Millisecond)
		if got, want := len(devices), 1; got != want {
			t.Errorf("got %d devices, want %d", got, want)
		}
	})
}
