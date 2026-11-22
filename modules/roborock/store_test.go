package roborock

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

type stubRecoverer struct {
	modulus *rsa.PublicKey
}

func (s *stubRecoverer) RecoverModulus(_ context.Context, _ []SignatureSample, _ string) (*RecoveryResult, error) {
	return &RecoveryResult{Modulus: s.modulus.N, Duration: 5 * time.Millisecond, Backend: "stub"}, nil
}

func TestCatalogLookupTargetFleet(t *testing.T) {
	cases := []struct {
		model     string
		wantName  string
		wantCam   CameraBootstrapMode
	}{
		{"roborock.vacuum.a125", "Roborock Q5 Max+", CameraBootstrapBoth},
		{"a144", "Roborock Saros 10R", CameraBootstrapCountryDomain},
		{"roborock.vacuum.a288", "Roborock Saros 20", CameraBootstrapTokenR},
	}
	for _, tc := range cases {
		meta, ok := LookupProduct(tc.model)
		if !ok {
			t.Fatalf("LookupProduct(%q): got ok=false, want true", tc.model)
		}
		if meta.ProductName != tc.wantName {
			t.Fatalf("LookupProduct(%q).ProductName: got: %q, want: %q", tc.model, meta.ProductName, tc.wantName)
		}
		if cam := CameraBootstrapForModel(tc.model); cam != tc.wantCam {
			t.Fatalf("CameraBootstrapForModel(%q): got: %q, want: %q", tc.model, cam, tc.wantCam)
		}
	}
}

func TestKeyStoreAsyncRecoveryAndPersistence(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	storage := core.NewDirStorage(dir)
	ks, err := NewKeyStore(storage, &stubRecoverer{modulus: &priv.PublicKey}, core.NewNoOpLogger())
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}

	sig := base64.StdEncoding.EncodeToString(make([]byte, 256))
	added1, err := ks.AddSignedQuery("did-a125", "did=did-a125&ts=1&nonce=a&signature="+sig, "roborock.vacuum.a125")
	if err != nil || !added1 {
		t.Fatalf("AddSignedQuery #1: added=%v err=%v", added1, err)
	}
	added2, err := ks.AddSignedQuery("did-a125", "did=did-a125&ts=2&nonce=b&signature="+sig, "roborock.vacuum.a125")
	if err != nil || !added2 {
		t.Fatalf("AddSignedQuery #2: added=%v err=%v", added2, err)
	}

	ks.MaybeRecoverAsync(t.Context(), "did-a125")
	ks.WaitRecoveries()

	pub := ks.GetPublicKey("did-a125")
	if pub == nil || pub.N.Cmp(priv.N) != 0 {
		t.Fatalf("GetPublicKey after recovery: got: %v, want exact N", pub)
	}

	// Reload from disk and assert modulus survived persistence
	ks2, err := NewKeyStore(storage, nil, nil)
	if err != nil {
		t.Fatalf("reload KeyStore: %v", err)
	}
	if pub2 := ks2.GetPublicKey("did-a125"); pub2 == nil || pub2.N.Cmp(priv.N) != 0 {
		t.Fatalf("reloaded GetPublicKey: got: %v, want exact N", pub2)
	}
}

func TestCredentialStoreAndUpstreamImport(t *testing.T) {
	storage := core.NewDirStorage(t.TempDir())
	cs, err := NewCredentialStore(storage)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := cs.EnsureDevice(DeviceCredential{
		DID:            "10001",
		Model:          "roborock.vacuum.a144",
		DeviceMQTTUser: "vac_user_1",
		DeviceMQTTPass: "vac_pass_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dev.LocalKey) != 16 {
		t.Fatalf("minted localKey length: got: %d, want: 16", len(dev.LocalKey))
	}
	if dev.Name != "Roborock Saros 10R" {
		t.Fatalf("default device name: got: %q, want: %q", dev.Name, "Roborock Saros 10R")
	}
	if ok, reason, _ := cs.VerifyDeviceMQTT("vac_user_1", "vac_pass_1"); !ok || reason != "device_mqtt_user" {
		t.Fatalf("VerifyDeviceMQTT: got: ok=%v reason=%q, want: ok=true reason=device_mqtt_user", ok, reason)
	}
	if lk := cs.LocalKeyForTopic("rr/d/i/10001/vac_user_1"); lk != dev.LocalKey {
		t.Fatalf("LocalKeyForTopic: got: %q, want: %q", lk, dev.LocalKey)
	}

	// Test upstream import
	upDir := t.TempDir()
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	keysJSON := `{"devices":{"20002":{"samples":[],"header_samples":[],"modulus_hex":"` + priv.N.Text(16) + `"}}}`
	if err := os.WriteFile(filepath.Join(upDir, DefaultFileDeviceKeys), []byte(keysJSON), 0600); err != nil {
		t.Fatal(err)
	}
	invJSON := `{"devices":[{"duid":"duid-288","did":"20002","name":"Saros 20","model":"roborock.vacuum.a288","localKey":"1122334455667788"}]}`
	if err := os.WriteFile(filepath.Join(upDir, DefaultFileInventory), []byte(invJSON), 0600); err != nil {
		t.Fatal(err)
	}
	ks, _ := NewKeyStore(storage, nil, nil)
	kCnt, dCnt, err := ImportUpstreamDir(upDir, ks, cs)
	if err != nil {
		t.Fatalf("ImportUpstreamDir: %v", err)
	}
	if kCnt != 1 || dCnt != 1 {
		t.Fatalf("ImportUpstreamDir counts: got: keys=%d devs=%d, want: 1, 1", kCnt, dCnt)
	}
	if ks.GetPublicKey("20002") == nil {
		t.Fatalf("imported public key not found")
	}
	if gotDev, ok := cs.ResolveByNameOrID("a288"); !ok || gotDev.LocalKey != "1122334455667788" {
		t.Fatalf("imported device lookup: got: %+v ok=%v", gotDev, ok)
	}
}
