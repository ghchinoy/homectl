package roborock

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

const (
	// DefaultFileDeviceKeys is schema-compatible with Python local_roborock_server's device_keys.json.
	DefaultFileDeviceKeys = "device_keys.json"
	// DefaultFileCredentials stores per-device local keys, DID/DUID links, and MQTT credentials.
	DefaultFileCredentials = "runtime_credentials.json"
	// DefaultFileInventory stores home, room, and device inventory metadata.
	DefaultFileInventory = "web_api_inventory.json"

	maxQuerySamples  = 16
	maxHeaderSamples = 64
)

// RecoveryMeta tracks the async RSA public-key recovery lifecycle for a device.
type RecoveryMeta struct {
	State      string `json:"state,omitempty"` // missing | collecting | recovering | recovered | failed | blocked
	Note       string `json:"note,omitempty"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// DeviceKeyRecord persists signature samples and recovered RSA modulus for one DID.
type DeviceKeyRecord struct {
	Samples       []SignatureSample `json:"samples"`
	HeaderSamples []HeaderSample    `json:"header_samples"`
	ModulusHex    string            `json:"modulus_hex,omitempty"`
	Model         string            `json:"model,omitempty"`
	Recovery      *RecoveryMeta     `json:"recovery,omitempty"`
}

type deviceKeysFile struct {
	Devices map[string]*DeviceKeyRecord `json:"devices"`
}

// KeyStore manages persistent signature samples, RSA public keys, and background recovery.
type KeyStore struct {
	mu         sync.RWMutex
	storage    core.Storage
	filename   string
	recoverer  ModulusRecoverer
	logger     core.Logger
	devices    map[string]*DeviceKeyRecord
	pubkeys    map[string]*rsa.PublicKey
	recovering map[string]bool
	wg         sync.WaitGroup
}

// NewKeyStore initializes and loads a KeyStore from storage.
func NewKeyStore(storage core.Storage, recoverer ModulusRecoverer, logger core.Logger) (*KeyStore, error) {
	if storage == nil {
		storage = core.NewMemoryStorage()
	}
	if recoverer == nil {
		recoverer = NewDefaultModulusRecoverer("")
	}
	if logger == nil {
		logger = core.NewNoOpLogger()
	}
	ks := &KeyStore{
		storage:    storage,
		filename:   DefaultFileDeviceKeys,
		recoverer:  recoverer,
		logger:     logger,
		devices:    make(map[string]*DeviceKeyRecord),
		pubkeys:    make(map[string]*rsa.PublicKey),
		recovering: make(map[string]bool),
	}
	if err := ks.load(); err != nil {
		return nil, err
	}
	return ks, nil
}

func (ks *KeyStore) load() error {
	raw, err := ks.storage.ReadFile(ks.filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("roborock: read %s: %w", ks.filename, err)
	}
	var parsed deviceKeysFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("roborock: parse %s: %w", ks.filename, err)
	}
	for did, rec := range parsed.Devices {
		did = strings.TrimSpace(did)
		if did == "" || rec == nil {
			continue
		}
		if rec.Samples == nil {
			rec.Samples = []SignatureSample{}
		}
		if rec.HeaderSamples == nil {
			rec.HeaderSamples = []HeaderSample{}
		}
		ks.devices[did] = rec
		if rec.ModulusHex != "" {
			if pub, err := PublicKeyFromModulusHex(rec.ModulusHex); err == nil {
				ks.pubkeys[did] = pub
				if rec.Recovery == nil {
					rec.Recovery = &RecoveryMeta{State: "recovered", Note: "Public key is available."}
				} else if rec.Recovery.State != "recovered" {
					rec.Recovery.State = "recovered"
				}
			}
		}
	}
	return nil
}

func (ks *KeyStore) saveLocked() error {
	payload := deviceKeysFile{Devices: ks.devices}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(ks.storage, ks.filename, data, 0600)
}

// GetPublicKey returns the recovered RSA public key for did, or nil if not yet recovered.
func (ks *KeyStore) GetPublicKey(did string) *rsa.PublicKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.pubkeys[strings.TrimSpace(did)]
}

// SetModulus stores a known or recovered RSA modulus for did.
func (ks *KeyStore) SetModulus(did string, modulus *big.Int, note string) error {
	did = strings.TrimSpace(did)
	if did == "" || modulus == nil || modulus.Sign() <= 0 {
		return errors.New("roborock: invalid did or modulus")
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	rec := ks.ensureLocked(did)
	rec.ModulusHex = modulus.Text(16)
	ks.pubkeys[did] = &rsa.PublicKey{N: new(big.Int).Set(modulus), E: DefaultRSAExponent}
	if note == "" {
		note = fmt.Sprintf("Recovered %d-bit RSA public key.", modulus.BitLen())
	}
	if rec.Recovery == nil {
		rec.Recovery = &RecoveryMeta{}
	}
	rec.Recovery.State = "recovered"
	rec.Recovery.Note = note
	rec.Recovery.Error = ""
	rec.Recovery.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return ks.saveLocked()
}

// RecordSnapshot returns a deep copy of all stored key records.
func (ks *KeyStore) RecordSnapshot() map[string]DeviceKeyRecord {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	out := make(map[string]DeviceKeyRecord, len(ks.devices))
	for did, rec := range ks.devices {
		cp := *rec
		cp.Samples = append([]SignatureSample(nil), rec.Samples...)
		cp.HeaderSamples = append([]HeaderSample(nil), rec.HeaderSamples...)
		if rec.Recovery != nil {
			rm := *rec.Recovery
			cp.Recovery = &rm
		}
		out[did] = cp
	}
	return out
}

// KnownDIDs returns sorted DIDs present in the KeyStore.
func (ks *KeyStore) KnownDIDs() []string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	dids := make([]string, 0, len(ks.devices))
	for did := range ks.devices {
		dids = append(dids, did)
	}
	sort.Strings(dids)
	return dids
}

func (ks *KeyStore) ensureLocked(did string) *DeviceKeyRecord {
	rec, ok := ks.devices[did]
	if !ok {
		rec = &DeviceKeyRecord{
			Samples:       []SignatureSample{},
			HeaderSamples: []HeaderSample{},
		}
		ks.devices[did] = rec
	}
	return rec
}

// AddSignedQuery records a V1 query-signed sample for did. Returns true if newly added.
func (ks *KeyStore) AddSignedQuery(did, rawQuery, model string) (bool, error) {
	did = strings.TrimSpace(did)
	if did == "" {
		return false, nil
	}
	sample, ok := SplitSignedQuery(rawQuery)
	if !ok {
		return false, nil
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	rec := ks.ensureLocked(did)
	if model != "" && rec.Model == "" {
		rec.Model = strings.TrimSpace(model)
	}
	for _, existing := range rec.Samples {
		if existing == sample {
			return false, nil
		}
	}
	rec.Samples = append(rec.Samples, sample)
	if len(rec.Samples) > maxQuerySamples {
		rec.Samples = rec.Samples[len(rec.Samples)-maxQuerySamples:]
	}
	if ks.pubkeys[did] == nil {
		rec.Recovery = &RecoveryMeta{
			State: "collecting",
			Note:  fmt.Sprintf("Captured %d query signature sample(s).", len(rec.Samples)),
		}
	}
	return true, ks.saveLocked()
}

// AddHeaderSignature records a V2 header-signed sample for did. Returns true if newly added.
func (ks *KeyStore) AddHeaderSignature(did string, sample HeaderSample, model string) (bool, error) {
	did = strings.TrimSpace(did)
	sample.SignatureB64 = strings.TrimSpace(sample.SignatureB64)
	if did == "" || sample.SignatureB64 == "" {
		return false, nil
	}
	raw, err := base64.StdEncoding.DecodeString(sample.SignatureB64)
	if err != nil {
		return false, nil
	}
	sample.Method = strings.ToUpper(strings.TrimSpace(sample.Method))
	sample.Path = strings.TrimSpace(sample.Path)
	sample.Nonce = strings.TrimSpace(sample.Nonce)
	sample.TS = strings.TrimSpace(sample.TS)
	sample.Version = strings.TrimSpace(sample.Version)
	sample.SignatureLen = strconv.Itoa(len(raw))

	ks.mu.Lock()
	defer ks.mu.Unlock()
	rec := ks.ensureLocked(did)
	if model != "" && rec.Model == "" {
		rec.Model = strings.TrimSpace(model)
	}
	for _, existing := range rec.HeaderSamples {
		if existing == sample {
			return false, nil
		}
	}
	rec.HeaderSamples = append(rec.HeaderSamples, sample)
	if len(rec.HeaderSamples) > maxHeaderSamples {
		rec.HeaderSamples = rec.HeaderSamples[len(rec.HeaderSamples)-maxHeaderSamples:]
	}
	if ks.pubkeys[did] == nil && len(rec.Samples) == 0 {
		rec.Recovery = &RecoveryMeta{
			State: "collecting",
			Note:  fmt.Sprintf("Captured %d header signature sample(s).", len(rec.HeaderSamples)),
		}
	}
	return true, ks.saveLocked()
}

func (ks *KeyStore) recoverySamplesLocked(did string) ([]SignatureSample, string, string) {
	rec := ks.devices[did]
	if rec == nil {
		return nil, "sha256", "query"
	}
	var v2 []SignatureSample
	seen := make(map[string]bool)
	for _, hs := range rec.HeaderSamples {
		if pair, ok := SplitV2RegionSample(hs); ok && !seen[pair.Canonical] {
			seen[pair.Canonical] = true
			v2 = append(v2, pair)
		}
	}
	if len(v2) > 0 {
		return v2, "sha384", "v2 region header"
	}
	return append([]SignatureSample(nil), rec.Samples...), "sha256", "query"
}

// MaybeRecoverAsync triggers background RSA modulus recovery if >=2 valid samples exist and no key is known yet.
func (ks *KeyStore) MaybeRecoverAsync(ctx context.Context, did string) {
	did = strings.TrimSpace(did)
	if did == "" {
		return
	}
	ks.mu.Lock()
	if ks.pubkeys[did] != nil {
		ks.mu.Unlock()
		return
	}
	samples, hashName, source := ks.recoverySamplesLocked(did)
	rec := ks.ensureLocked(did)
	if len(samples) < 2 {
		rec.Recovery = &RecoveryMeta{
			State: "collecting",
			Note:  fmt.Sprintf("Need at least 2 %s signature samples (%d captured).", source, len(samples)),
		}
		_ = ks.saveLocked()
		ks.mu.Unlock()
		return
	}
	if ks.recovering[did] {
		ks.mu.Unlock()
		return
	}
	ks.recovering[did] = true
	rec.Recovery = &RecoveryMeta{
		State:     "recovering",
		Note:      fmt.Sprintf("Recovering RSA modulus from %d %s signatures (%s).", len(samples), source, hashName),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_ = ks.saveLocked()
	ks.wg.Add(1)
	ks.mu.Unlock()

	go func() {
		defer ks.wg.Done()
		defer func() {
			ks.mu.Lock()
			delete(ks.recovering, did)
			ks.mu.Unlock()
		}()

		res, err := ks.recoverer.RecoverModulus(ctx, samples, hashName)
		ks.mu.Lock()
		defer ks.mu.Unlock()
		r := ks.ensureLocked(did)
		nowISO := time.Now().UTC().Format(time.RFC3339)
		if err != nil || res == nil || res.Modulus == nil {
			errMsg := "no modulus recovered"
			if err != nil {
				errMsg = err.Error()
			}
			r.Recovery = &RecoveryMeta{
				State:      "failed",
				Note:       "RSA modulus recovery failed.",
				Error:      errMsg,
				StartedAt:  r.Recovery.StartedAt,
				FinishedAt: nowISO,
			}
			_ = ks.saveLocked()
			ks.logger.Printf("roborock: modulus recovery failed for did=%s: %s", did, errMsg)
			return
		}
		r.ModulusHex = res.Modulus.Text(16)
		ks.pubkeys[did] = &rsa.PublicKey{N: new(big.Int).Set(res.Modulus), E: DefaultRSAExponent}
		r.Recovery = &RecoveryMeta{
			State:      "recovered",
			Note:       fmt.Sprintf("Recovered %d-bit public key in %.1fs (%s).", res.Modulus.BitLen(), res.Duration.Seconds(), res.Backend),
			StartedAt:  r.Recovery.StartedAt,
			FinishedAt: nowISO,
		}
		_ = ks.saveLocked()
		ks.logger.Printf("roborock: recovered %d-bit RSA public key for did=%s in %s (%s)", res.Modulus.BitLen(), did, res.Duration.Round(time.Millisecond), res.Backend)
	}()
}

// WaitRecoveries blocks until all active background recovery goroutines finish (useful in tests/shutdown).
func (ks *KeyStore) WaitRecoveries() {
	ks.wg.Wait()
}

// DeviceCredential persists per-vacuum runtime identities, localKey, and MQTT auth state.
type DeviceCredential struct {
	DID              string `json:"did,omitempty"`
	DUID             string `json:"duid,omitempty"`
	Name             string `json:"name,omitempty"`
	Model            string `json:"model,omitempty"`
	ProductID        string `json:"product_id,omitempty"`
	LocalKey         string `json:"localkey,omitempty"`
	LocalKeySource   string `json:"local_key_source,omitempty"`
	DeviceMQTTUser   string `json:"device_mqtt_usr,omitempty"`
	DeviceMQTTPass   string `json:"device_mqtt_pass,omitempty"`
	DeviceMQTTTopic  string `json:"device_mqtt_topic,omitempty"`
	FirmwareVersion  string `json:"fv,omitempty"`
	SerialNumber     string `json:"sn,omitempty"`
	FeatureSet       string `json:"feature_set,omitempty"`
	NewFeatureSet    string `json:"new_feature_set,omitempty"`
	LastNCAt         string `json:"last_nc_at,omitempty"`
	LastMQTTAt       string `json:"last_mqtt_at,omitempty"`
	LastIP           string `json:"last_ip,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type credentialsFile struct {
	Bootstrap struct {
		MQTTUser     string `json:"mqtt_usr,omitempty"`
		MQTTPass     string `json:"mqtt_passwd,omitempty"`
		MQTTClientID string `json:"mqtt_clientid,omitempty"`
		LocalKey     string `json:"localkey,omitempty"`
	} `json:"bootstrap"`
	Devices []*DeviceCredential `json:"devices"`
}

// CredentialStore manages persistent per-device local keys, DID<->DUID bindings, and MQTT credentials.
type CredentialStore struct {
	mu       sync.RWMutex
	storage  core.Storage
	filename string
	data     credentialsFile
}

// NewCredentialStore loads or initializes a CredentialStore.
func NewCredentialStore(storage core.Storage) (*CredentialStore, error) {
	if storage == nil {
		storage = core.NewMemoryStorage()
	}
	cs := &CredentialStore{
		storage:  storage,
		filename: DefaultFileCredentials,
	}
	if err := cs.load(); err != nil {
		return nil, err
	}
	if cs.data.Bootstrap.MQTTUser == "" {
		cs.data.Bootstrap.MQTTUser = "rr_" + randomHex(6)
		cs.data.Bootstrap.MQTTPass = randomHex(12)
		cs.data.Bootstrap.MQTTClientID = "rr_local_" + randomHex(4)
		cs.data.Bootstrap.LocalKey = randomHex(8)
		_ = cs.saveLocked()
	}
	return cs, nil
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (cs *CredentialStore) load() error {
	raw, err := cs.storage.ReadFile(cs.filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("roborock: read %s: %w", cs.filename, err)
	}
	if err := json.Unmarshal(raw, &cs.data); err != nil {
		return fmt.Errorf("roborock: parse %s: %w", cs.filename, err)
	}
	return nil
}

func (cs *CredentialStore) saveLocked() error {
	raw, err := json.MarshalIndent(cs.data, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(cs.storage, cs.filename, raw, 0600)
}

// BootstrapCredentials returns the stack's bootstrap MQTT user, password, and client ID.
func (cs *CredentialStore) BootstrapCredentials() (user, pass, clientID string) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data.Bootstrap.MQTTUser, cs.data.Bootstrap.MQTTPass, cs.data.Bootstrap.MQTTClientID
}

// DefaultLocalKey returns the fallback local key.
func (cs *CredentialStore) DefaultLocalKey() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data.Bootstrap.LocalKey
}

// ListDevices returns snapshots of all known devices.
func (cs *CredentialStore) ListDevices() []DeviceCredential {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make([]DeviceCredential, 0, len(cs.data.Devices))
	for _, d := range cs.data.Devices {
		if d != nil {
			out = append(out, *d)
		}
	}
	return out
}

func (cs *CredentialStore) findLocked(did, duid, mqttUser string) *DeviceCredential {
	did = strings.TrimSpace(did)
	duid = strings.TrimSpace(duid)
	mqttUser = strings.TrimSpace(mqttUser)
	for _, d := range cs.data.Devices {
		if d == nil {
			continue
		}
		if did != "" && (d.DID == did || d.DUID == did) {
			return d
		}
		if duid != "" && (d.DUID == duid || d.DID == duid) {
			return d
		}
		if mqttUser != "" && d.DeviceMQTTUser == mqttUser {
			return d
		}
	}
	return nil
}

// ResolveDevice looks up a device by DID, DUID, or MQTT username.
func (cs *CredentialStore) ResolveDevice(did, duid, mqttUser string) (DeviceCredential, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	d := cs.findLocked(did, duid, mqttUser)
	if d == nil {
		return DeviceCredential{}, false
	}
	return *d, true
}

// ResolveByNameOrID looks up a device by DID, DUID, Name (case-insensitive), or Model suffix (e.g. "a144").
func (cs *CredentialStore) ResolveByNameOrID(query string) (DeviceCredential, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return DeviceCredential{}, false
	}
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, d := range cs.data.Devices {
		if d == nil {
			continue
		}
		if strings.ToLower(d.DID) == q || strings.ToLower(d.DUID) == q ||
			strings.ToLower(d.Name) == q || strings.ToLower(d.Model) == q ||
			strings.HasSuffix(strings.ToLower(d.Model), "."+q) {
			return *d, true
		}
	}
	return DeviceCredential{}, false
}

// EnsureDevice creates or updates a device credential record, minting a 16-char localKey if missing.
func (cs *CredentialStore) EnsureDevice(update DeviceCredential) (DeviceCredential, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	d := cs.findLocked(update.DID, update.DUID, update.DeviceMQTTUser)
	if d == nil {
		d = &DeviceCredential{}
		cs.data.Devices = append(cs.data.Devices, d)
	}
	if update.DID != "" {
		d.DID = strings.TrimSpace(update.DID)
	}
	if update.DUID != "" {
		d.DUID = strings.TrimSpace(update.DUID)
	}
	if d.DUID == "" && d.DID != "" {
		d.DUID = d.DID
	}
	if update.Model != "" {
		d.Model = strings.TrimSpace(update.Model)
	}
	if update.Name != "" {
		d.Name = strings.TrimSpace(update.Name)
	} else if d.Name == "" && d.Model != "" {
		d.Name = DefaultProductName(d.Model)
	}
	if update.ProductID != "" {
		d.ProductID = strings.TrimSpace(update.ProductID)
	} else if d.ProductID == "" && d.Model != "" {
		if meta, ok := LookupProduct(d.Model); ok {
			d.ProductID = meta.ProductID
		}
	}
	if update.LocalKey != "" {
		d.LocalKey = strings.TrimSpace(update.LocalKey)
	} else if d.LocalKey == "" {
		d.LocalKey = randomHex(8) // 16 ASCII hex chars (16 bytes for AES-128)
		if update.LocalKeySource == "" {
			update.LocalKeySource = "minted"
		}
	}
	if update.LocalKeySource != "" {
		d.LocalKeySource = update.LocalKeySource
	}
	if update.DeviceMQTTUser != "" {
		d.DeviceMQTTUser = strings.TrimSpace(update.DeviceMQTTUser)
	}
	if update.DeviceMQTTPass != "" {
		d.DeviceMQTTPass = strings.TrimSpace(update.DeviceMQTTPass)
	}
	if update.DeviceMQTTTopic != "" {
		d.DeviceMQTTTopic = strings.TrimSpace(update.DeviceMQTTTopic)
	}
	if update.FirmwareVersion != "" {
		d.FirmwareVersion = strings.TrimSpace(update.FirmwareVersion)
	}
	if update.SerialNumber != "" {
		d.SerialNumber = strings.TrimSpace(update.SerialNumber)
	}
	if update.FeatureSet != "" {
		d.FeatureSet = strings.TrimSpace(update.FeatureSet)
	}
	if update.NewFeatureSet != "" {
		d.NewFeatureSet = strings.TrimSpace(update.NewFeatureSet)
	}
	if update.LastNCAt != "" {
		d.LastNCAt = update.LastNCAt
	}
	if update.LastMQTTAt != "" {
		d.LastMQTTAt = update.LastMQTTAt
	}
	if update.LastIP != "" {
		d.LastIP = update.LastIP
	}
	d.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := cs.saveLocked(); err != nil {
		return *d, err
	}
	return *d, nil
}

// VerifyDeviceMQTT authenticates a vacuum MQTT username/password pair.
// Returns (ok, reason, device).
func (cs *CredentialStore) VerifyDeviceMQTT(username, password string) (bool, string, DeviceCredential) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if username == "" || password == "" {
		return false, "missing_mqtt_credentials", DeviceCredential{}
	}
	if username == cs.data.Bootstrap.MQTTUser && password == cs.data.Bootstrap.MQTTPass {
		return true, "bootstrap", DeviceCredential{}
	}
	for _, d := range cs.data.Devices {
		if d == nil || d.DeviceMQTTUser == "" {
			continue
		}
		if d.DeviceMQTTUser != username {
			continue
		}
		if d.DeviceMQTTPass == "" {
			return false, "device_mqtt_password_missing", *d
		}
		if d.DeviceMQTTPass == password {
			return true, "device_mqtt_user", *d
		}
		return false, "invalid_device_mqtt_password", *d
	}
	return false, "unknown_device_mqtt_username", DeviceCredential{}
}

// LocalKeyForTopic resolves the 16-byte localKey for a device or cloud MQTT topic.
func (cs *CredentialStore) LocalKeyForTopic(topic string) string {
	parts := strings.Split(topic, "/")
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if len(parts) >= 5 && parts[0] == "rr" && parts[1] == "d" {
		did, usr := parts[3], parts[4]
		if d := cs.findLocked(did, "", usr); d != nil && d.LocalKey != "" {
			return d.LocalKey
		}
	}
	if len(parts) >= 6 && parts[0] == "rr" && parts[1] == "m" {
		duid := parts[5]
		if d := cs.findLocked("", duid, ""); d != nil && d.LocalKey != "" {
			return d.LocalKey
		}
	}
	return cs.data.Bootstrap.LocalKey
}

// ImportUpstreamDir imports device_keys.json and runtime_credentials.json (or web_api_inventory.json)
// from an existing Python local_roborock_server data directory into storage.
func ImportUpstreamDir(srcDir string, ks *KeyStore, cs *CredentialStore) (importedKeys, importedDevices int, err error) {
	keysPath := filepath.Join(srcDir, DefaultFileDeviceKeys)
	if raw, readErr := os.ReadFile(keysPath); readErr == nil {
		var parsed deviceKeysFile
		if jErr := json.Unmarshal(raw, &parsed); jErr != nil {
			return 0, 0, fmt.Errorf("roborock: parse upstream %s: %w", keysPath, jErr)
		}
		for did, rec := range parsed.Devices {
			if rec == nil || rec.ModulusHex == "" {
				continue
			}
			n, ok := new(big.Int).SetString(strings.TrimSpace(rec.ModulusHex), 16)
			if !ok {
				continue
			}
			if setErr := ks.SetModulus(did, n, "Imported from local_roborock_server."); setErr != nil {
				return importedKeys, importedDevices, setErr
			}
			importedKeys++
		}
	}

	credsPath := filepath.Join(srcDir, DefaultFileCredentials)
	if raw, readErr := os.ReadFile(credsPath); readErr == nil {
		var parsed credentialsFile
		if jErr := json.Unmarshal(raw, &parsed); jErr != nil {
			return importedKeys, 0, fmt.Errorf("roborock: parse upstream %s: %w", credsPath, jErr)
		}
		for _, dev := range parsed.Devices {
			if dev == nil {
				continue
			}
			if _, eErr := cs.EnsureDevice(*dev); eErr != nil {
				return importedKeys, importedDevices, eErr
			}
			importedDevices++
		}
	}

	invPath := filepath.Join(srcDir, DefaultFileInventory)
	if raw, readErr := os.ReadFile(invPath); readErr == nil {
		var inv struct {
			Devices []struct {
				DUID      string `json:"duid"`
				DID       string `json:"did"`
				Name      string `json:"name"`
				Model     string `json:"model"`
				ProductID string `json:"productId"`
				LocalKey  string `json:"localKey"`
				FV        string `json:"fv"`
				SN        string `json:"sn"`
			} `json:"devices"`
		}
		if jErr := json.Unmarshal(raw, &inv); jErr == nil {
			for _, d := range inv.Devices {
				if d.DUID == "" && d.DID == "" {
					continue
				}
				if _, eErr := cs.EnsureDevice(DeviceCredential{
					DID:             d.DID,
					DUID:            d.DUID,
					Name:            d.Name,
					Model:           d.Model,
					ProductID:       d.ProductID,
					LocalKey:        d.LocalKey,
					LocalKeySource:  "imported_inventory",
					FirmwareVersion: d.FV,
					SerialNumber:    d.SN,
				}); eErr != nil {
					return importedKeys, importedDevices, eErr
				}
				importedDevices++
			}
		}
	}

	return importedKeys, importedDevices, nil
}

// writeFileAtomic performs an atomic temp-file + rename write on disk storages, falling back to WriteFile for MemoryStorage.
func writeFileAtomic(storage core.Storage, filename string, data []byte, perm os.FileMode) error {
	if _, isMem := storage.(*core.MemoryStorage); isMem {
		return storage.WriteFile(filename, data, perm)
	}
	if err := storage.EnsureDir(); err != nil {
		return err
	}
	target := storage.Path(filename)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return storage.WriteFile(filename, data, perm)
	}
	return nil
}
