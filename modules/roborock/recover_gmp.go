//go:build gmp

package roborock

/*
#cgo darwin CFLAGS: -I/opt/homebrew/opt/gmp/include -I/usr/local/opt/gmp/include
#cgo darwin LDFLAGS: -L/opt/homebrew/opt/gmp/lib -L/usr/local/opt/gmp/lib -lgmp
#cgo linux LDFLAGS: -lgmp
#include <gmp.h>
#include <stdlib.h>

static char* rr_recover_gcd_hex(const char **sigs, const char **ems, int count, unsigned long e) {
    if (count < 2) {
        return NULL;
    }
    mpz_t s, em, x, acc;
    mpz_inits(s, em, x, acc, NULL);
    for (int i = 0; i < count; i++) {
        if (mpz_set_str(s, sigs[i], 16) != 0 || mpz_set_str(em, ems[i], 16) != 0) {
            mpz_clears(s, em, x, acc, NULL);
            return NULL;
        }
        mpz_pow_ui(x, s, e);
        mpz_sub(x, x, em);
        mpz_abs(x, x);
        if (i == 0) {
            mpz_set(acc, x);
        } else {
            mpz_gcd(acc, acc, x);
        }
    }
    char *out = mpz_get_str(NULL, 16, acc);
    mpz_clears(s, em, x, acc, NULL);
    return out;
}
*/
import "C"
import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unsafe"
)

// GMPModulusRecoverer recovers RSA public moduli using native libgmp (~16s RSA-2048, ~38s RSA-4096).
type GMPModulusRecoverer struct{}

// NewDefaultModulusRecoverer returns the native libgmp recoverer when built with -tags gmp.
func NewDefaultModulusRecoverer(_ string) ModulusRecoverer {
	return &GMPModulusRecoverer{}
}

// RecoverModulus computes GCD(sig_i^65537 - EM_i) via libgmp and verifies the resulting modulus.
func (r *GMPModulusRecoverer) RecoverModulus(ctx context.Context, samples []SignatureSample, hashName string) (*RecoveryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	started := time.Now()
	dedup := make(map[string]string, len(samples))
	var unique []SignatureSample
	for _, s := range samples {
		if s.Canonical == "" || s.SignatureB64 == "" {
			continue
		}
		if _, exists := dedup[s.Canonical]; exists {
			continue
		}
		dedup[s.Canonical] = s.SignatureB64
		unique = append(unique, s)
	}
	if len(unique) < 2 {
		return nil, errors.New("roborock: need at least 2 unique signature samples for RSA recovery")
	}

	keyBytes := 0
	type decodedSample struct {
		sig *big.Int
		em  *big.Int
	}
	var allDecoded []decodedSample
	for _, s := range unique {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s.SignatureB64))
		if err != nil {
			continue
		}
		if len(raw) > keyBytes {
			keyBytes = len(raw)
		}
	}
	if keyBytes < MinRSASignatureBytes {
		return nil, fmt.Errorf("roborock: signatures too short (%d bytes) for RSA recovery", keyBytes)
	}
	for _, s := range unique {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s.SignatureB64))
		if err != nil || len(raw) != keyBytes {
			continue
		}
		em, err := EMSAPKCS1v15(s.Canonical, keyBytes, hashName)
		if err != nil {
			return nil, err
		}
		allDecoded = append(allDecoded, decodedSample{
			sig: new(big.Int).SetBytes(raw),
			em:  em,
		})
	}
	if len(allDecoded) < 2 {
		return nil, fmt.Errorf("roborock: need >=2 signatures of length %d bytes", keyBytes)
	}

	// Cap GCD inputs at 3 for RSA-4096/SHA-384 (matching upstream), verify against all samples.
	gcdCount := len(allDecoded)
	if strings.EqualFold(hashName, "sha384") && gcdCount > 3 {
		gcdCount = 3
	}

	cSigs := make([]*C.char, gcdCount)
	cEMs := make([]*C.char, gcdCount)
	for i := 0; i < gcdCount; i++ {
		cSigs[i] = C.CString(allDecoded[i].sig.Text(16))
		cEMs[i] = C.CString(allDecoded[i].em.Text(16))
	}
	defer func() {
		for i := 0; i < gcdCount; i++ {
			C.free(unsafe.Pointer(cSigs[i]))
			C.free(unsafe.Pointer(cEMs[i]))
		}
	}()

	outPtr := C.rr_recover_gcd_hex(&cSigs[0], &cEMs[0], C.int(gcdCount), C.ulong(DefaultRSAExponent))
	if outPtr == nil {
		return nil, errors.New("roborock: libgmp modulus recovery returned null")
	}
	gcdHex := C.GoString(outPtr)
	C.free(unsafe.Pointer(outPtr))

	gcdVal, ok := new(big.Int).SetString(gcdHex, 16)
	if !ok || gcdVal.Sign() <= 0 {
		return nil, errors.New("roborock: invalid hex returned from libgmp")
	}

	verifiers := make([][2]*big.Int, len(allDecoded))
	for i, d := range allDecoded {
		verifiers[i] = [2]*big.Int{d.sig, d.em}
	}
	modulus := StripSmallCofactorsAndVerify(gcdVal, keyBytes*8, verifiers)
	if modulus == nil {
		return nil, fmt.Errorf("roborock: recovered GCD (%d bits) did not yield a verifying %d-bit RSA modulus", gcdVal.BitLen(), keyBytes*8)
	}
	return &RecoveryResult{
		Modulus:  modulus,
		Duration: time.Since(started),
		Backend:  "gmp",
	}, nil
}
