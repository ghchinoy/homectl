package roborock

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultRSAExponent is the standard RSA public exponent used by Roborock firmware.
	DefaultRSAExponent = 65537
	// MinRSASignatureBytes rejects short HMAC tags from RSA recovery.
	MinRSASignatureBytes = 128
	// MaxRRCheckInnerBytes matches the 128-byte firmware stack buffer with safety margin.
	MaxRRCheckInnerBytes = 120
	// MaxBootstrapSingleBlockOAEP is the single-block RSA-2048 OAEP-SHA1 plaintext limit (256 - 42 = 214).
	MaxBootstrapSingleBlockOAEP = 214
)

// SignatureSample holds a canonical signed message and its base64 PKCS#1 v1.5 signature.
type SignatureSample struct {
	Canonical    string `json:"canonical"`
	SignatureB64 string `json:"signature_b64"`
}

// HeaderSample holds a captured V2 HTTP header-signed request sample (RSA-4096 / SHA-384).
type HeaderSample struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Query        string `json:"query"`
	Nonce        string `json:"nonce"`
	TS           string `json:"ts"`
	SignatureB64 string `json:"signature_b64"`
	BodySHA256   string `json:"body_sha256,omitempty"`
	SignatureLen string `json:"signature_len,omitempty"`
	Version      string `json:"version,omitempty"`
}

// RecoveryResult holds the outcome of RSA modulus recovery.
type RecoveryResult struct {
	Modulus  *big.Int
	Duration time.Duration
	Backend  string
}

// ModulusRecoverer recovers an RSA modulus N from >=2 deterministic PKCS#1 v1.5 signatures.
type ModulusRecoverer interface {
	RecoverModulus(ctx context.Context, samples []SignatureSample, hashName string) (*RecoveryResult, error)
}

// RRCheckGrant is the {k, d} payload returned by POST /f/c and GET /devices/{did}/sh.
type RRCheckGrant struct {
	K string `json:"k"`
	D string `json:"d"`
}

// SplitSignedQuery extracts the canonical query (without signature=) and base64 signature from a V1 request.
func SplitSignedQuery(rawQuery string) (SignatureSample, bool) {
	if rawQuery == "" || !strings.Contains(rawQuery, "signature=") {
		return SignatureSample{}, false
	}
	parts := strings.Split(rawQuery, "&")
	canonicalParts := make([]string, 0, len(parts))
	var sigRaw string
	for _, p := range parts {
		if strings.HasPrefix(p, "signature=") && sigRaw == "" {
			sigRaw = strings.TrimPrefix(p, "signature=")
			continue
		}
		canonicalParts = append(canonicalParts, p)
	}
	if sigRaw == "" {
		return SignatureSample{}, false
	}
	sigB64, err := url.QueryUnescape(sigRaw)
	if err != nil {
		return SignatureSample{}, false
	}
	if _, err := base64.StdEncoding.DecodeString(sigB64); err != nil {
		return SignatureSample{}, false
	}
	return SignatureSample{
		Canonical:    strings.Join(canonicalParts, "&"),
		SignatureB64: sigB64,
	}, true
}

// SplitV2RegionSample extracts the verified RSA-4096/SHA-384 canonical payload from a V2 GET /region sample.
func SplitV2RegionSample(sample HeaderSample) (SignatureSample, bool) {
	if sample.Version != "v2" || !strings.EqualFold(sample.Method, "GET") {
		return SignatureSample{}, false
	}
	if sample.Path != "/region" && sample.Path != "/.roborock.com/region" {
		return SignatureSample{}, false
	}
	rawSig, err := base64.StdEncoding.DecodeString(sample.SignatureB64)
	if err != nil || len(rawSig) != 512 {
		return SignatureSample{}, false
	}
	query := sample.Query
	if idx := strings.Index(query, "&signature="); idx >= 0 {
		query = query[:idx]
	}
	if query == "" || sample.Nonce == "" || sample.TS == "" {
		return SignatureSample{}, false
	}
	return SignatureSample{
		Canonical:    fmt.Sprintf("%s:%s:%s", query, sample.Nonce, sample.TS),
		SignatureB64: sample.SignatureB64,
	}, true
}

// EMSAPKCS1v15 computes the deterministic EMSA-PKCS1-v1.5 encoded integer for SHA-256 or SHA-384.
func EMSAPKCS1v15(msg string, keyBytes int, hashName string) (*big.Int, error) {
	var prefixHex string
	var digest []byte
	switch strings.ToLower(hashName) {
	case "sha256":
		prefixHex = "3031300d060960864801650304020105000420"
		h := sha256.Sum256([]byte(msg))
		digest = h[:]
	case "sha384":
		prefixHex = "3041300d060960864801650304020205000430"
		h := sha512.Sum384([]byte(msg))
		digest = h[:]
	default:
		return nil, fmt.Errorf("roborock: unsupported RSA recovery hash %q", hashName)
	}
	prefix, err := hex.DecodeString(prefixHex)
	if err != nil {
		return nil, err
	}
	digestInfo := append(prefix, digest...)
	if keyBytes < len(digestInfo)+11 {
		return nil, fmt.Errorf("roborock: key size %d too small for PKCS#1 v1.5", keyBytes)
	}
	psLen := keyBytes - len(digestInfo) - 3
	em := make([]byte, 0, keyBytes)
	em = append(em, 0x00, 0x01)
	for i := 0; i < psLen; i++ {
		em = append(em, 0xff)
	}
	em = append(em, 0x00)
	em = append(em, digestInfo...)
	return new(big.Int).SetBytes(em), nil
}

// StripSmallCofactorsAndVerify reduces a candidate GCD by small prime factors (2..31)
// and verifies sig^e mod candidate == EM across all samples.
func StripSmallCofactorsAndVerify(gcd *big.Int, targetBits int, verifiers [][2]*big.Int) *big.Int {
	if gcd == nil || gcd.Sign() <= 0 {
		return nil
	}
	e := big.NewInt(DefaultRSAExponent)
	verify := func(cand *big.Int) bool {
		if cand.BitLen() != targetBits {
			return false
		}
		tmp := new(big.Int)
		for _, pair := range verifiers {
			tmp.Exp(pair[0], e, cand)
			if tmp.Cmp(pair[1]) != 0 {
				return false
			}
		}
		return true
	}

	if verify(gcd) {
		return new(big.Int).Set(gcd)
	}

	smallPrimes := []int64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31}
	type primePow struct {
		p     int64
		power int
	}
	rem := new(big.Int).Set(gcd)
	mod := new(big.Int)
	var factors []primePow
	for _, p := range smallPrimes {
		bp := big.NewInt(p)
		pow := 0
		for {
			mod.Mod(rem, bp)
			if mod.Sign() != 0 {
				break
			}
			rem.Quo(rem, bp)
			pow++
		}
		if pow > 0 {
			factors = append(factors, primePow{p: p, power: pow})
		}
	}
	if len(factors) == 0 {
		return nil
	}

	divisors := []*big.Int{big.NewInt(1)}
	for _, f := range factors {
		bp := big.NewInt(f.p)
		next := make([]*big.Int, 0, len(divisors)*(f.power+1))
		pp := big.NewInt(1)
		for k := 0; k <= f.power; k++ {
			for _, d := range divisors {
				next = append(next, new(big.Int).Mul(d, pp))
			}
			pp = new(big.Int).Mul(pp, bp)
		}
		divisors = next
		if len(divisors) > 4096 {
			break
		}
	}

	sort.Slice(divisors, func(i, j int) bool {
		return divisors[i].Cmp(divisors[j]) < 0
	})
	trial := new(big.Int)
	for _, div := range divisors {
		trial.Quo(gcd, div)
		if verify(trial) {
			return new(big.Int).Set(trial)
		}
	}
	return nil
}

// EncryptJSONOAEPSHA1B64 encrypts JSON (or raw string) in chunks using RSA-OAEP-SHA1,
// matching Roborock's /region and /nc bootstrap encryption contract.
func EncryptJSONOAEPSHA1B64(pub *rsa.PublicKey, payload any) (string, error) {
	if pub == nil {
		return "", errors.New("roborock: nil rsa public key")
	}
	var data []byte
	switch v := payload.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return "", err
		}
	}

	keyBytes := pub.Size()
	chunkMax := keyBytes - 2*sha1.Size - 2
	if chunkMax <= 0 {
		return "", fmt.Errorf("roborock: rsa key too small (%d bytes)", keyBytes)
	}
	var out []byte
	for off := 0; off < len(data); off += chunkMax {
		end := off + chunkMax
		if end > len(data) {
			end = len(data)
		}
		enc, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, data[off:end], nil)
		if err != nil {
			return "", err
		}
		out = append(out, enc...)
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// BuildRRCheckGrant forges the {k, d} acceptance grant for POST /f/c and GET /devices/{did}/sh
// so vacuums never hit the ~48h "-10002 rrcheck access denied" lockout when WAN-blocked.
func BuildRRCheckGrant(pub *rsa.PublicKey, country string, nowUnix int64) (RRCheckGrant, error) {
	if pub == nil {
		return RRCheckGrant{}, errors.New("roborock: nil rsa public key")
	}
	if country == "" {
		country = "US"
	}
	inner, err := json.Marshal(struct {
		F string `json:"f"`
		C string `json:"c"`
		T int64  `json:"t"`
	}{
		F: "t",
		C: strings.ToUpper(country),
		T: nowUnix,
	})
	if err != nil {
		return RRCheckGrant{}, err
	}
	if len(inner) >= MaxRRCheckInnerBytes {
		return RRCheckGrant{}, fmt.Errorf("roborock: rrcheck inner payload too large (%d >= %d)", len(inner), MaxRRCheckInnerBytes)
	}

	aesKey := make([]byte, aes.BlockSize)
	if _, err := rand.Read(aesKey); err != nil {
		return RRCheckGrant{}, err
	}
	ciphertext, err := encryptAESECB(inner, aesKey)
	if err != nil {
		return RRCheckGrant{}, err
	}
	wrappedKey, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return RRCheckGrant{}, err
	}
	return RRCheckGrant{
		K: base64.StdEncoding.EncodeToString(wrappedKey),
		D: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// VerifyDeviceInfoSignature verifies the PKCS#1 v1.5 SHA-256 signature on POST /devices/{did}/info.
func VerifyDeviceInfoSignature(pub *rsa.PublicKey, canonicalForm, sigB64 string) error {
	if pub == nil {
		return errors.New("roborock: missing device public key")
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if err != nil {
		return fmt.Errorf("roborock: invalid base64 signature: %w", err)
	}
	digest := sha256.Sum256([]byte(canonicalForm))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

// PublicKeyFromModulusHex parses a hex RSA modulus string into an *rsa.PublicKey.
func PublicKeyFromModulusHex(modHex string) (*rsa.PublicKey, error) {
	cleaned := strings.TrimSpace(modHex)
	if cleaned == "" {
		return nil, errors.New("roborock: empty modulus")
	}
	n, ok := new(big.Int).SetString(cleaned, 16)
	if !ok || n.Sign() <= 0 {
		return nil, errors.New("roborock: invalid hex modulus")
	}
	return &rsa.PublicKey{N: n, E: DefaultRSAExponent}, nil
}
