package core

import "sync"

// Settings provides access to application and environment configuration.
type Settings interface {
	// CallbackIP returns the callback IP address used for event subscriptions (e.g. GENA).
	CallbackIP() string

	// CameraAuth returns the global username:password authentication string for RTSP cameras.
	CameraAuth() string

	// Get retrieves a generic string configuration value.
	Get(key string) string
}

// StaticSettings is an in-memory implementation of Settings.
type StaticSettings struct {
	mu         sync.RWMutex
	callbackIP string
	cameraAuth string
	values     map[string]string
}

// NewStaticSettings constructs a StaticSettings instance.
func NewStaticSettings(callbackIP, cameraAuth string) *StaticSettings {
	return &StaticSettings{
		callbackIP: callbackIP,
		cameraAuth: cameraAuth,
		values:     make(map[string]string),
	}
}

func (s *StaticSettings) CallbackIP() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.callbackIP
}

func (s *StaticSettings) SetCallbackIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbackIP = ip
}

func (s *StaticSettings) CameraAuth() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cameraAuth
}

func (s *StaticSettings) SetCameraAuth(auth string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cameraAuth = auth
}

func (s *StaticSettings) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

func (s *StaticSettings) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}
