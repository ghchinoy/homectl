//go:build !gmp

package roborock

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os/exec"
	"strings"
	"time"
)

// ExternalModulusRecoverer invokes a configured external helper (e.g. a binary built with -tags gmp
// or a python3/gmpy2 one-liner) when homectl is compiled as a pure-Go CGO_ENABLED=0 binary.
type ExternalModulusRecoverer struct {
	HelperCmd string
}

// NewDefaultModulusRecoverer returns the pure-Go external helper recoverer.
func NewDefaultModulusRecoverer(helperCmd string) ModulusRecoverer {
	return &ExternalModulusRecoverer{HelperCmd: strings.TrimSpace(helperCmd)}
}

type externalRecoveryRequest struct {
	HashName string            `json:"hash_name"`
	Exponent int               `json:"e"`
	Samples  []SignatureSample `json:"samples"`
}

type externalRecoveryResponse struct {
	ModulusHex string `json:"modulus_hex"`
	Error      string `json:"error,omitempty"`
}

const pyGmpy2Helper = `
import base64, hashlib, json, sys, gmpy2
req = json.load(sys.stdin)
e = int(req.get("e", 65537))
h = req.get("hash_name", "sha256")
prefixes = {"sha256": "3031300d060960864801650304020105000420", "sha384": "3041300d060960864801650304020205000430"}
xs = []
for s in req["samples"][:3 if h == "sha384" else len(req["samples"])]:
    sig = base64.b64decode(s["signature_b64"])
    kb = len(sig)
    d = hashlib.new(h, s["canonical"].encode()).digest()
    di = bytes.fromhex(prefixes[h]) + d
    em = b"\x00\x01" + b"\xff"*(kb - len(di) - 3) + b"\x00" + di
    xs.append(abs(gmpy2.mpz(int.from_bytes(sig, "big"))**e - gmpy2.mpz(int.from_bytes(em, "big"))))
acc = xs[0]
for v in xs[1:]:
    acc = gmpy2.gcd(acc, v)
print(json.dumps({"modulus_hex": format(int(acc), "x")}))
`

// RecoverModulus delegates the 134M/268M-bit GCD to an external helper and verifies the result in pure Go.
func (r *ExternalModulusRecoverer) RecoverModulus(ctx context.Context, samples []SignatureSample, hashName string) (*RecoveryResult, error) {
	if len(samples) < 2 {
		return nil, errors.New("roborock: need at least 2 unique signature samples for RSA recovery")
	}
	started := time.Now()
	payload, err := json.Marshal(externalRecoveryRequest{
		HashName: hashName,
		Exponent: DefaultRSAExponent,
		Samples:  samples,
	})
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	backend := "external"
	if r.HelperCmd != "" {
		cmd = exec.CommandContext(ctx, r.HelperCmd)
	} else if pyPath, err := exec.LookPath("python3"); err == nil {
		cmd = exec.CommandContext(ctx, pyPath, "-c", pyGmpy2Helper)
		backend = "python3-gmpy2"
	} else {
		return nil, errors.New("roborock: pure-Go build has no libgmp; rebuild with '-tags gmp' (make build-gmp), configure key_recover_cmd, or import device_keys.json")
	}

	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("roborock: key recovery helper (%s) failed: %w (stderr: %s)", backend, err, strings.TrimSpace(stderr.String()))
	}

	var resp externalRecoveryResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("roborock: decode key recovery helper output: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("roborock: key recovery helper error: %s", resp.Error)
	}
	gcdVal, ok := new(big.Int).SetString(strings.TrimSpace(resp.ModulusHex), 16)
	if !ok || gcdVal.Sign() <= 0 {
		return nil, errors.New("roborock: helper returned invalid modulus_hex")
	}

	// Verify in pure Go and strip any small prime cofactors.
	var keyBytes int
	var verifiers [][2]*big.Int
	for _, s := range samples {
		raw, err := decodeBase64Strict(s.SignatureB64)
		if err != nil {
			continue
		}
		if keyBytes == 0 {
			keyBytes = len(raw)
		}
		if len(raw) != keyBytes {
			continue
		}
		em, err := EMSAPKCS1v15(s.Canonical, keyBytes, hashName)
		if err != nil {
			return nil, err
		}
		verifiers = append(verifiers, [2]*big.Int{new(big.Int).SetBytes(raw), em})
	}
	modulus := StripSmallCofactorsAndVerify(gcdVal, keyBytes*8, verifiers)
	if modulus == nil {
		return nil, fmt.Errorf("roborock: helper GCD did not verify as a %d-bit RSA modulus", keyBytes*8)
	}
	return &RecoveryResult{
		Modulus:  modulus,
		Duration: time.Since(started),
		Backend:  backend,
	}, nil
}

func decodeBase64Strict(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
}
