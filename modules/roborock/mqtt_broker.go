package roborock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

var (
	deviceInTopicRe = regexp.MustCompile(`^rr/d/i/([^/]+)/([^/]+)$`)
	cloudInTopicRe  = regexp.MustCompile(`^rr/m/i/([^/]+)/([^/]+)/([^/]+)$`)
)

// MQTTBrokerConfig configures the embedded Mochi MQTT v2 TLS broker.
type MQTTBrokerConfig struct {
	BindHost              string
	Port                  int
	AllowLegacyTLS        bool
	DisableTLS            bool // true only for external TLS proxy or plaintext unit tests
	NewConnectionsEnabled bool // allow learning new device MQTT credentials during onboarding
}

// UserMQTTVerifier validates optional app / Home Assistant MQTT credentials (Phase 4).
type UserMQTTVerifier func(username, password string) bool

// RPCCallback is invoked when an encrypted protocol-102 RPC response is decoded from a vacuum.
type RPCCallback func(did string, rpcID int64, rawResp map[string]any)

// MQTTBroker embeds Mochi MQTT v2 with native TLS, Roborock auth, rr/d <-> rr/m bridging, and live telemetry decoding.
type MQTTBroker struct {
	cfg          MQTTBrokerConfig
	creds        *CredentialStore
	telemetry    *TelemetryHub
	certs        *MultiSNICertManager
	logger       core.Logger
	userVerifier UserMQTTVerifier
	onRPCResp    RPCCallback

	server    *mqtt.Server
	listener  *listeners.TCP
	hook      *roborockBrokerHook
	closeOnce sync.Once
}

// NewMQTTBroker constructs an embedded Roborock MQTT broker.
func NewMQTTBroker(
	cfg MQTTBrokerConfig,
	creds *CredentialStore,
	telemetry *TelemetryHub,
	certs *MultiSNICertManager,
	logger core.Logger,
) *MQTTBroker {
	if cfg.BindHost == "" {
		cfg.BindHost = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = 8881
	}
	if logger == nil {
		logger = core.NewNoOpLogger()
	}
	return &MQTTBroker{
		cfg:       cfg,
		creds:     creds,
		telemetry: telemetry,
		certs:     certs,
		logger:    logger,
	}
}

// SetUserVerifier attaches an optional Phase 4 HA/LocalRock MQTT user verifier.
func (b *MQTTBroker) SetUserVerifier(fn UserMQTTVerifier) {
	b.userVerifier = fn
}

// SetRPCCallback registers a handler notified whenever a decoded protocol-102 RPC response arrives.
func (b *MQTTBroker) SetRPCCallback(fn RPCCallback) {
	b.onRPCResp = fn
	if b.hook != nil {
		b.hook.onRPCResp = fn
	}
}

// Start launches the embedded Mochi MQTT server and TLS/TCP listener.
func (b *MQTTBroker) Start(ctx context.Context) error {
	quietLog := slog.New(slog.NewTextHandler(&discardWriter{}, nil))
	b.server = mqtt.New(&mqtt.Options{
		InlineClient: true,
		Logger:       quietLog,
	})

	b.hook = &roborockBrokerHook{
		broker:         b,
		onRPCResp:      b.onRPCResp,
		pendingOnboard: make(map[string]pendingMQTTAuth),
		connDevice:     make(map[string]deviceTopicKey),
		seenDevices:    make(map[string]deviceTopicKey),
		cloudRoutes:    make(map[cloudTopicKey]deviceTopicKey),
	}
	if err := b.server.AddHook(b.hook, nil); err != nil {
		return fmt.Errorf("roborock: register mqtt hook: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", b.cfg.BindHost, b.cfg.Port)
	lCfg := listeners.Config{
		ID:      "roborock-mqtt",
		Address: addr,
	}
	if !b.cfg.DisableTLS {
		if b.certs == nil {
			return fmt.Errorf("roborock: MQTT TLS enabled on %s but no certificate manager provided", addr)
		}
		lCfg.TLSConfig = b.certs.TLSConfig(b.cfg.AllowLegacyTLS)
	}
	b.listener = listeners.NewTCP(lCfg)
	if err := b.server.AddListener(b.listener); err != nil {
		return fmt.Errorf("roborock: add mqtt listener %s: %w", addr, err)
	}
	if err := b.server.Serve(); err != nil {
		return fmt.Errorf("roborock: serve mqtt: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = b.Close()
	}()

	scheme := "ssl"
	if b.cfg.DisableTLS {
		scheme = "tcp"
	}
	b.logger.Printf("roborock: embedded Mochi MQTT broker listening on %s://%s", scheme, b.Addr())
	return nil
}

// Addr returns the listener's bound host:port address.
func (b *MQTTBroker) Addr() string {
	if b.listener == nil {
		return ""
	}
	return b.listener.Address()
}

// Close stops the embedded MQTT broker cleanly (idempotent).
func (b *MQTTBroker) Close() error {
	if b.server == nil {
		return nil
	}
	var err error
	b.closeOnce.Do(func() {
		err = b.server.Close()
	})
	return err
}

// PublishInline publishes a payload from the in-process server client (bypasses topic-bridge echo loops).
func (b *MQTTBroker) PublishInline(topic string, payload []byte, qos byte) error {
	if b.server == nil {
		return fmt.Errorf("roborock: mqtt broker is not running")
	}
	return b.server.Publish(topic, payload, false, qos)
}

// ActiveDeviceTopic returns the latest observed outbound topic (rr/d/o/{did}/{usr}) for a device DID.
func (b *MQTTBroker) ActiveDeviceTopic(did string) (deviceTopicKey, bool) {
	if b.hook == nil {
		return deviceTopicKey{}, false
	}
	b.hook.mu.RLock()
	defer b.hook.mu.RUnlock()
	if dt, ok := b.hook.seenDevices[did]; ok {
		return dt, true
	}
	if b.creds != nil {
		if dev, ok := b.creds.ResolveDevice(did, did, ""); ok && dev.DID != "" && dev.DeviceMQTTUser != "" {
			return deviceTopicKey{DID: dev.DID, MQTTUser: dev.DeviceMQTTUser}, true
		}
	}
	return deviceTopicKey{}, false
}

type deviceTopicKey struct {
	DID      string
	MQTTUser string
}

func (d deviceTopicKey) topicIn() string  { return fmt.Sprintf("rr/d/i/%s/%s", d.DID, d.MQTTUser) }
func (d deviceTopicKey) topicOut() string { return fmt.Sprintf("rr/d/o/%s/%s", d.DID, d.MQTTUser) }

type cloudTopicKey struct {
	RRiotU   string
	MQTTUser string
	DUID     string
}

func (c cloudTopicKey) topicOut() string {
	return fmt.Sprintf("rr/m/o/%s/%s/%s", c.RRiotU, c.MQTTUser, c.DUID)
}

type pendingMQTTAuth struct {
	Username string
	Password string
	RemoteIP string
}

type roborockBrokerHook struct {
	mqtt.HookBase
	broker    *MQTTBroker
	onRPCResp RPCCallback

	mu             sync.RWMutex
	pendingOnboard map[string]pendingMQTTAuth
	connDevice     map[string]deviceTopicKey
	seenDevices    map[string]deviceTopicKey
	cloudRoutes    map[cloudTopicKey]deviceTopicKey
}

func (h *roborockBrokerHook) ID() string { return "roborock-bridge-telemetry" }

func (h *roborockBrokerHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnDisconnect,
		mqtt.OnPublish,
	}, []byte{b})
}

func (h *roborockBrokerHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	user := strings.TrimSpace(string(pk.Connect.Username))
	pass := strings.TrimSpace(string(pk.Connect.Password))
	remoteIP, _, _ := net.SplitHostPort(cl.Net.Remote)
	if remoteIP == "" {
		remoteIP = cl.Net.Remote
	}

	if user == "" || pass == "" {
		return false
	}

	// 1. Optional HA / LocalRock user credentials.
	if h.broker.userVerifier != nil && h.broker.userVerifier(user, pass) {
		return true
	}

	// 2. Stored bootstrap or device MQTT credentials.
	if h.broker.creds != nil {
		ok, reason, dev := h.broker.creds.VerifyDeviceMQTT(user, pass)
		if ok {
			if reason == "device_mqtt_user" && dev.DID != "" {
				dt := deviceTopicKey{DID: dev.DID, MQTTUser: user}
				h.mu.Lock()
				h.connDevice[cl.ID] = dt
				h.seenDevices[dev.DID] = dt
				h.mu.Unlock()
				if h.broker.telemetry != nil {
					h.broker.telemetry.RecordConnection(dev.DID, dev.DUID, dev.Name, dev.Model, remoteIP, true)
				}
			}
			return true
		}
		if reason == "device_mqtt_password_missing" && dev.DID != "" {
			_, _ = h.broker.creds.EnsureDevice(DeviceCredential{
				DID:            dev.DID,
				DeviceMQTTUser: user,
				DeviceMQTTPass: pass,
				LastIP:         remoteIP,
			})
			dt := deviceTopicKey{DID: dev.DID, MQTTUser: user}
			h.mu.Lock()
			h.connDevice[cl.ID] = dt
			h.seenDevices[dev.DID] = dt
			h.mu.Unlock()
			if h.broker.telemetry != nil {
				h.broker.telemetry.RecordConnection(dev.DID, dev.DUID, dev.Name, dev.Model, remoteIP, true)
			}
			return true
		}
		// 3. Provisional onboarding auth learning (confirmed on first valid rr/d/i/{did}/{user} publish).
		if reason == "unknown_device_mqtt_username" && h.broker.cfg.NewConnectionsEnabled {
			h.mu.Lock()
			h.pendingOnboard[cl.ID] = pendingMQTTAuth{
				Username: user,
				Password: pass,
				RemoteIP: remoteIP,
			}
			h.mu.Unlock()
			return true
		}
	}
	return false
}

func (h *roborockBrokerHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return true
}

func (h *roborockBrokerHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	h.mu.Lock()
	delete(h.pendingOnboard, cl.ID)
	dt, hadDev := h.connDevice[cl.ID]
	delete(h.connDevice, cl.ID)
	h.mu.Unlock()

	if hadDev && h.broker.telemetry != nil {
		var duid, name, model string
		if h.broker.creds != nil {
			if dev, ok := h.broker.creds.ResolveDevice(dt.DID, dt.DID, dt.MQTTUser); ok {
				duid, name, model = dev.DUID, dev.Name, dev.Model
			}
		}
		h.broker.telemetry.RecordConnection(dt.DID, duid, name, model, "", false)
	}
}

func (h *roborockBrokerHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// Skip bridging/recursion on messages published by our own inline client.
	if cl.Net.Inline {
		return pk, nil
	}

	topic := pk.TopicName
	payload := pk.Payload

	// Case A: Vacuum -> Server (rr/d/i/{did}/{mqtt_usr})
	if m := deviceInTopicRe.FindStringSubmatch(topic); len(m) == 3 {
		did, usr := m[1], m[2]
		dt := deviceTopicKey{DID: did, MQTTUser: usr}

		h.confirmProvisionalOnboarding(cl.ID, dt)

		h.mu.Lock()
		h.seenDevices[did] = dt
		targets := h.cloudTargetsForDeviceLocked(dt)
		h.mu.Unlock()

		// Decode encrypted Roborock payload in-process for telemetry & RPC correlation.
		h.inspectPayload(did, topic, payload)

		// Fan out to subscribed cloud-style topics (rr/m/o/{u}/{user}/{duid}).
		for _, ct := range targets {
			_ = h.broker.PublishInline(ct.topicOut(), payload, pk.FixedHeader.Qos)
		}
		return pk, nil
	}

	// Case B: App / Home Assistant -> Server (rr/m/i/{rriot_u}/{mqtt_usr}/{duid})
	if m := cloudInTopicRe.FindStringSubmatch(topic); len(m) == 4 {
		ct := cloudTopicKey{RRiotU: m[1], MQTTUser: m[2], DUID: m[3]}
		if dt, ok := h.resolveDeviceForCloud(ct); ok {
			h.mu.Lock()
			h.cloudRoutes[ct] = dt
			h.mu.Unlock()
			h.inspectPayload(dt.DID, topic, payload)
			_ = h.broker.PublishInline(dt.topicOut(), payload, pk.FixedHeader.Qos)
		}
		return pk, nil
	}

	return pk, nil
}

func (h *roborockBrokerHook) confirmProvisionalOnboarding(clientID string, dt deviceTopicKey) {
	h.mu.Lock()
	pending, ok := h.pendingOnboard[clientID]
	if ok {
		delete(h.pendingOnboard, clientID)
	}
	h.connDevice[clientID] = dt
	h.seenDevices[dt.DID] = dt
	h.mu.Unlock()

	if !ok || h.broker.creds == nil {
		return
	}
	if pending.Username != dt.MQTTUser {
		return
	}
	dev, err := h.broker.creds.EnsureDevice(DeviceCredential{
		DID:             dt.DID,
		DeviceMQTTUser:  pending.Username,
		DeviceMQTTPass:  pending.Password,
		DeviceMQTTTopic: dt.topicIn(),
		LastMQTTAt:      time.Now().UTC().Format(time.RFC3339),
		LastIP:          pending.RemoteIP,
	})
	if err == nil && h.broker.telemetry != nil {
		h.broker.telemetry.RecordConnection(dev.DID, dev.DUID, dev.Name, dev.Model, pending.RemoteIP, true)
	}
}

func (h *roborockBrokerHook) resolveDeviceForCloud(ct cloudTopicKey) (deviceTopicKey, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 1. Direct lookup via CredentialStore DUID -> DID link.
	if h.broker.creds != nil {
		if dev, ok := h.broker.creds.ResolveDevice("", ct.DUID, ""); ok && dev.DID != "" {
			if seen, ok := h.seenDevices[dev.DID]; ok {
				return seen, true
			}
			if dev.DeviceMQTTUser != "" {
				return deviceTopicKey{DID: dev.DID, MQTTUser: dev.DeviceMQTTUser}, true
			}
		}
	}
	// 2. Existing learned route.
	if dt, ok := h.cloudRoutes[ct]; ok {
		return dt, true
	}
	// 3. Direct DID == DUID match.
	if dt, ok := h.seenDevices[ct.DUID]; ok {
		return dt, true
	}
	// 4. Single-device fallback when only one vacuum is connected.
	if len(h.seenDevices) == 1 {
		for _, dt := range h.seenDevices {
			return dt, true
		}
	}
	return deviceTopicKey{}, false
}

func (h *roborockBrokerHook) cloudTargetsForDeviceLocked(dt deviceTopicKey) []cloudTopicKey {
	var out []cloudTopicKey
	for ct, mapped := range h.cloudRoutes {
		if mapped.DID == dt.DID {
			out = append(out, ct)
		}
	}
	return out
}

func (h *roborockBrokerHook) inspectPayload(did, topic string, payload []byte) {
	if h.broker.creds == nil || len(payload) == 0 {
		return
	}
	localKey := h.broker.creds.LocalKeyForTopic(topic)
	if localKey == "" {
		return
	}
	msgs, _, err := DecodeMessages(payload, CodecOptions{LocalKey: localKey, Prefixed: false})
	if err != nil || len(msgs) == 0 {
		// Retry with default fallback key if per-topic key differed.
		fallback := h.broker.creds.DefaultLocalKey()
		if fallback != "" && fallback != localKey {
			msgs, _, err = DecodeMessages(payload, CodecOptions{LocalKey: fallback, Prefixed: false})
		}
	}
	if err != nil || len(msgs) == 0 {
		return
	}

	duid := did
	if dev, ok := h.broker.creds.ResolveDevice(did, did, ""); ok && dev.DUID != "" {
		duid = dev.DUID
	}

	for _, msg := range msgs {
		if h.broker.telemetry != nil {
			h.broker.telemetry.RecordDecodedMessage(did, duid, topic, msg)
		}
		if msg.Protocol == ProtoOTAFirmwareReport && len(msg.Payload) > 0 {
			var obj map[string]any
			if json.Unmarshal(msg.Payload, &obj) == nil {
				if fv := extractFirmwareVersion(obj); fv != "" {
					_, _ = h.broker.creds.EnsureDevice(DeviceCredential{
						DID:             did,
						FirmwareVersion: fv,
						LastMQTTAt:      time.Now().UTC().Format(time.RFC3339),
					})
				}
			}
		}
		if msg.Protocol == ProtoRPCResponse && h.onRPCResp != nil && len(msg.Payload) > 0 {
			var obj map[string]any
			if json.Unmarshal(msg.Payload, &obj) == nil {
				if dps, ok := obj["dps"].(map[string]any); ok {
					if rpcResp, ok := extractRPCFromDPS(dps, "102"); ok {
						if idInt, ok := anyToInt(rpcResp["id"]); ok {
							h.onRPCResp(did, int64(idInt), rpcResp)
						}
					}
				}
			}
		}
	}
}

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (int, error) { return len(p), nil }
