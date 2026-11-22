//go:build gmp

package roborock

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestGMPModulusRecovererRSA2048(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 16s RSA-2048 GMP recovery test in -short mode")
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	msgs := []string{
		"did=1001&pid=roborock.vacuum.a125&ts=1700000001&nonce=aa11",
		"did=1001&pid=roborock.vacuum.a125&ts=1700000002&nonce=bb22",
	}
	var samples []SignatureSample
	for _, m := range msgs {
		h := sha256.Sum256([]byte(m))
		sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h[:])
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, SignatureSample{
			Canonical:    m,
			SignatureB64: base64.StdEncoding.EncodeToString(sig),
		})
	}
	rec := &GMPModulusRecoverer{}
	res, err := rec.RecoverModulus(t.Context(), samples, "sha256")
	if err != nil {
		t.Fatalf("RecoverModulus failed: %v", err)
	}
	if res.Modulus.Cmp(priv.N) != 0 {
		t.Fatalf("recovered modulus mismatch: got %d bits, want exact N", res.Modulus.BitLen())
	}
}
