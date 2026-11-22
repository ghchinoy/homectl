package roborock

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ghchinoy/homectl/modules/core"
)

// StackConfig configures the unified single-binary Roborock local server stack.
type StackConfig struct {
	Storage               core.Storage
	Logger                core.Logger
	Recoverer             ModulusRecoverer
	KeyRecoverCmd         string
	Certs                 *MultiSNICertManager
	HTTP                  HTTPServerConfig
	MQTT                  MQTTBrokerConfig
	TURN                  TURNConfig
	TraceJSONL            bool
	NewConnectionsEnabled bool
}

// Stack wires together the HTTPS bootstrap server, embedded Mochi MQTT broker,
// live telemetry hub, active RPC controller, and optional embedded Pion TURN relay.
type Stack struct {
	Keys       *KeyStore
	Creds      *CredentialStore
	Telemetry  *TelemetryHub
	TURN       *TURNRelay
	HTTP       *HTTPServer
	MQTT       *MQTTBroker
	Controller *MQTTController
}

// NewStack initializes all components of the local Roborock stack without binding network ports.
func NewStack(cfg StackConfig) (*Stack, error) {
	storage := cfg.Storage
	if storage == nil {
		storage = core.NewDirStorage(filepath.Join(core.NewXDGStorage("homectl").Dir(), "roborock"))
	}
	logger := cfg.Logger
	if logger == nil {
		logger = core.NewNoOpLogger()
	}
	recoverer := cfg.Recoverer
	if recoverer == nil {
		recoverer = NewDefaultModulusRecoverer(cfg.KeyRecoverCmd)
	}

	keys, err := NewKeyStore(storage, recoverer, logger)
	if err != nil {
		return nil, fmt.Errorf("roborock: init key store: %w", err)
	}
	creds, err := NewCredentialStore(storage)
	if err != nil {
		return nil, fmt.Errorf("roborock: init credential store: %w", err)
	}

	var tracePath string
	if cfg.TraceJSONL {
		tracePath = storage.Path("telemetry_trace.jsonl")
	}
	telemetry := NewTelemetryHub(1000, tracePath)
	turnRelay := NewTURNRelay(cfg.TURN, logger)

	mqttCfg := cfg.MQTT
	mqttCfg.NewConnectionsEnabled = cfg.NewConnectionsEnabled || mqttCfg.NewConnectionsEnabled
	mqttBroker := NewMQTTBroker(mqttCfg, creds, telemetry, cfg.Certs, logger)
	controller := NewMQTTController(mqttBroker, creds)
	httpSrv := NewHTTPServer(cfg.HTTP, keys, creds, telemetry, turnRelay, cfg.Certs, logger)

	return &Stack{
		Keys:       keys,
		Creds:      creds,
		Telemetry:  telemetry,
		TURN:       turnRelay,
		HTTP:       httpSrv,
		MQTT:       mqttBroker,
		Controller: controller,
	}, nil
}

// Start launches TURN (if embedded), MQTT, and HTTPS listeners.
func (s *Stack) Start(ctx context.Context) error {
	if s.TURN != nil {
		if err := s.TURN.Start(); err != nil {
			return err
		}
	}
	if err := s.MQTT.Start(ctx); err != nil {
		_ = s.TURN.Close()
		return err
	}
	if err := s.HTTP.Start(ctx); err != nil {
		_ = s.MQTT.Close()
		_ = s.TURN.Close()
		return err
	}
	return nil
}

// Close gracefully shuts down HTTPS, MQTT, and TURN listeners.
func (s *Stack) Close(ctx context.Context) error {
	var firstErr error
	if s.HTTP != nil {
		if err := s.HTTP.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.MQTT != nil {
		if err := s.MQTT.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.TURN != nil {
		if err := s.TURN.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
