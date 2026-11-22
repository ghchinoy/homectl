package roborock

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func TestSplitSignedQueryAndV2Header(t *testing.T) {
	sig := base64.StdEncoding.EncodeToString(make([]byte, 256))
	q := "did=1103811971559&pid=roborock.vacuum.a125&ts=1712345678&nonce=abc123&signature=" + strings.ReplaceAll(sig, "+", "%2B")
	s, ok := SplitSignedQuery(q)
	if !ok {
		t.Fatalf("SplitSignedQuery returned ok=false")
	}
	wantCanon := "did=1103811971559&pid=roborock.vacuum.a125&ts=1712345678&nonce=abc123"
	if s.Canonical != wantCanon {
		t.Fatalf("canonical: got: %q, want: %q", s.Canonical, wantCanon)
	}

	v2Sig := base64.StdEncoding.EncodeToString(make([]byte, 512))
	hs := HeaderSample{
		Version:      "v2",
		Method:       "GET",
		Path:         "/.roborock.com/region",
		Query:        "did=999&pid=roborock.vacuum.a144",
		Nonce:        "n1",
		TS:           "1700000000",
		SignatureB64: v2Sig,
	}
	v2, ok := SplitV2RegionSample(hs)
	if !ok {
		t.Fatalf("SplitV2RegionSample returned ok=false")
	}
	wantV2 := "did=999&pid=roborock.vacuum.a144:n1:1700000000"
	if v2.Canonical != wantV2 {
		t.Fatalf("v2 canonical: got: %q, want: %q", v2.Canonical, wantV2)
	}
}

func TestEncryptJSONOAEPAndRRCheckGrant(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	// Test multi-chunk OAEP-SHA1 round-trip (>214 bytes)
	payload := map[string]string{
		"apiUrl":  "https://api-rr.example.com",
		"mqttUrl": "ssl://api-rr.example.com:8881",
		"padding": strings.Repeat("x", 240),
	}
	encB64, err := EncryptJSONOAEPSHA1B64(&priv.PublicKey, payload)
	if err != nil {
		t.Fatalf("EncryptJSONOAEPSHA1B64: %v", err)
	}
	encBytes, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(encBytes) != 512 {
		t.Fatalf("encrypted length: got: %d, want: 512 (2 RSA-2048 blocks)", len(encBytes))
	}
	var plain []byte
	for off := 0; off < len(encBytes); off += 256 {
		chunk, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, priv, encBytes[off:off+256], nil)
		if err != nil {
			t.Fatalf("DecryptOAEP block %d: %v", off/256, err)
		}
		plain = append(plain, chunk...)
	}
	var decoded map[string]string
	if err := json.Unmarshal(plain, &decoded); err != nil {
		t.Fatalf("unmarshal decrypted payload: %v", err)
	}
	if decoded["apiUrl"] != payload["apiUrl"] || decoded["mqttUrl"] != payload["mqttUrl"] {
		t.Fatalf("decoded mismatch: got: %+v, want: %+v", decoded, payload)
	}

	// Test BuildRRCheckGrant ({k, d}) full decrypt verification
	grant, err := BuildRRCheckGrant(&priv.PublicKey, "us", 1712345678)
	if err != nil {
		t.Fatalf("BuildRRCheckGrant: %v", err)
	}
	kRaw, err := base64.StdEncoding.DecodeString(grant.K)
	if err != nil {
		t.Fatal(err)
	}
	aesKey, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, priv, kRaw, nil)
	if err != nil {
		t.Fatalf("decrypt rrcheck k: %v", err)
	}
	dRaw, err := base64.StdEncoding.DecodeString(grant.D)
	if err != nil {
		t.Fatal(err)
	}
	innerPlain, err := decryptAESECB(dRaw, aesKey)
	if err != nil {
		t.Fatalf("decrypt rrcheck d: %v", err)
	}
	var inner struct {
		F string `json:"f"`
		C string `json:"c"`
		T int64  `json:"t"`
	}
	if err := json.Unmarshal(innerPlain, &inner); err != nil {
		t.Fatal(err)
	}
	if inner.F != "t" || inner.C != "US" || inner.T != 1712345678 {
		t.Fatalf("rrcheck inner: got: %+v, want: {F:t C:US T:1712345678}", inner)
	}
}

func TestVerifyDeviceInfoSignatureAndCofactorStrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	form := "did=12345&featureset=123456&newfeatureset=00abcdef&pid=roborock.vacuum.a144&sn=RR000000000001"
	digest := sha256.Sum256([]byte(form))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	if err := VerifyDeviceInfoSignature(&priv.PublicKey, form, sigB64); err != nil {
		t.Fatalf("VerifyDeviceInfoSignature failed: %v", err)
	}
	if err := VerifyDeviceInfoSignature(&priv.PublicKey, form+"x", sigB64); err == nil {
		t.Fatalf("VerifyDeviceInfoSignature should have failed on tampered form")
	}

	// Cofactor stripping: 6 * N should reduce back to exact N
	em, err := EMSAPKCS1v15(form, 256, "sha256")
	if err != nil {
		t.Fatal(err)
	}
	withCofactor := new(big.Int).Mul(priv.N, big.NewInt(6))
	got := StripSmallCofactorsAndVerify(withCofactor, 2048, [][2]*big.Int{{new(big.Int).SetBytes(sig), em}})
	if got == nil || got.Cmp(priv.N) != 0 {
		t.Fatalf("StripSmallCofactorsAndVerify: got: %v, want exact N", got)
	}
}
