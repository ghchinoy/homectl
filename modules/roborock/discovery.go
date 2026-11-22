package roborock

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	// DefaultBroadcastPort is the UDP port where Roborock vacuums periodically broadcast their DUID and LAN IP.
	DefaultBroadcastPort = 58866
	// BroadcastToken is the fixed key used by Roborock firmware to encrypt local UDP :58866 broadcasts.
	BroadcastToken = "qWKYcdQWrbm9hPqe"
)

// BroadcastDevice represents one vacuum discovered via UDP port 58866 LAN broadcast.
type BroadcastDevice struct {
	DUID    string `json:"duid"`
	IP      string `json:"ip"`
	Version string `json:"version"` // "1.0" or "L01"
}

// DecodeBroadcastPacket decrypts and verifies a single Roborock UDP :58866 broadcast frame.
// Supports both legacy "1.0" (AES-128-ECB) and modern "L01" (AES-256-GCM) broadcast formats.
func DecodeBroadcastPacket(data []byte) (*BroadcastDevice, error) {
	if len(data) < 15 {
		return nil, fmt.Errorf("roborock discovery: packet too short (%d bytes)", len(data))
	}
	version := string(data[:3])
	if !isValidVersionHeader(data[:3]) {
		return nil, fmt.Errorf("roborock discovery: unsupported broadcast version %q", version)
	}

	// Layout:
	//   [0:3]   version ("1.0" or "L01")
	//   [3:7]   seq / field1 (4 bytes)
	//   [7:9]   protocol / field2 (2 bytes)
	//   [9:11]  uint16 big-endian encrypted payload length
	//   [11:11+payloadLen] encrypted payload
	//   [11+payloadLen:15+payloadLen] uint32 big-endian IEEE CRC32 over [0:11+payloadLen]
	payloadLen := int(binary.BigEndian.Uint16(data[9:11]))
	frameWithoutCRC := 11 + payloadLen
	frameWithCRC := frameWithoutCRC + 4
	if payloadLen == 0 || len(data) < frameWithCRC {
		return nil, fmt.Errorf("roborock discovery: truncated broadcast packet (len=%d, want>=%d)", len(data), frameWithCRC)
	}

	expectedCRC := crc32.ChecksumIEEE(data[:frameWithoutCRC])
	gotCRC := binary.BigEndian.Uint32(data[frameWithoutCRC:frameWithCRC])
	if gotCRC != expectedCRC {
		return nil, fmt.Errorf("%w: got 0x%08x, want 0x%08x", ErrChecksum, gotCRC, expectedCRC)
	}

	encrypted := data[11:frameWithoutCRC]
	var plain []byte
	var err error
	if version == VersionL01 {
		if len(encrypted) < 16 {
			return nil, errors.New("roborock discovery: L01 broadcast payload shorter than GCM tag")
		}
		key := sha256.Sum256([]byte(BroadcastToken))
		ivHash := sha256.Sum256(data[:9])
		block, cErr := aes.NewCipher(key[:])
		if cErr != nil {
			return nil, cErr
		}
		gcm, gErr := cipher.NewGCM(block)
		if gErr != nil {
			return nil, gErr
		}
		plain, err = gcm.Open(nil, ivHash[:12], encrypted, nil)
		if err != nil {
			return nil, fmt.Errorf("roborock discovery: L01 broadcast decrypt failed: %w", err)
		}
	} else {
		plain, err = decryptAESECB(encrypted, []byte(BroadcastToken))
		if err != nil {
			return nil, fmt.Errorf("roborock discovery: V1 broadcast decrypt failed: %w", err)
		}
	}

	var parsed struct {
		DUID string `json:"duid"`
		IP   string `json:"ip"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(plain), &parsed); err != nil {
		return nil, fmt.Errorf("roborock discovery: parse broadcast JSON: %w", err)
	}
	parsed.DUID = strings.TrimSpace(parsed.DUID)
	parsed.IP = strings.TrimSpace(parsed.IP)
	if parsed.DUID == "" || parsed.IP == "" {
		return nil, errors.New("roborock discovery: broadcast JSON missing duid or ip")
	}
	return &BroadcastDevice{
		DUID:    parsed.DUID,
		IP:      parsed.IP,
		Version: version,
	}, nil
}

// EncodeBroadcastPacket builds a valid UDP :58866 broadcast frame (primarily for testing).
func EncodeBroadcastPacket(dev BroadcastDevice) ([]byte, error) {
	ver := dev.Version
	if ver == "" {
		ver = Version10
	}
	if len(ver) != 3 {
		return nil, fmt.Errorf("roborock discovery: invalid version %q", ver)
	}
	payload, err := json.Marshal(map[string]string{
		"duid": dev.DUID,
		"ip":   dev.IP,
	})
	if err != nil {
		return nil, err
	}

	header := make([]byte, 9)
	copy(header[0:3], ver)
	binary.BigEndian.PutUint32(header[3:7], 1)
	binary.BigEndian.PutUint16(header[7:9], 1)

	var encrypted []byte
	if ver == VersionL01 {
		key := sha256.Sum256([]byte(BroadcastToken))
		ivHash := sha256.Sum256(header)
		block, err := aes.NewCipher(key[:])
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		encrypted = gcm.Seal(nil, ivHash[:12], payload, nil)
	} else {
		encrypted, err = encryptAESECB(payload, []byte(BroadcastToken))
		if err != nil {
			return nil, err
		}
	}

	out := make([]byte, 0, 9+2+len(encrypted)+4)
	out = append(out, header...)
	var b2 [2]byte
	binary.BigEndian.PutUint16(b2[:], uint16(len(encrypted)))
	out = append(out, b2[:]...)
	out = append(out, encrypted...)
	var b4 [4]byte
	binary.BigEndian.PutUint32(b4[:], crc32.ChecksumIEEE(out))
	out = append(out, b4[:]...)
	return out, nil
}

// DiscoverLAN listens on UDP port 58866 for Roborock vacuum broadcasts up to timeout.
// Vacuums typically broadcast every ~3–5 seconds, so a 5–6 second window discovers active units.
func DiscoverLAN(ctx context.Context, timeout time.Duration) ([]BroadcastDevice, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			}); err != nil {
				return err
			}
			return sockErr
		},
	}
	pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf("0.0.0.0:%d", DefaultBroadcastPort))
	if err != nil {
		return nil, fmt.Errorf("roborock discovery: listen udp :%d: %w", DefaultBroadcastPort, err)
	}
	defer pc.Close()

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = pc.SetReadDeadline(deadline)

	found := make(map[string]BroadcastDevice)
	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return sortedBroadcastDevices(found), nil
		default:
		}
		n, addr, readErr := pc.ReadFrom(buf)
		if readErr != nil {
			var netErr net.Error
			if errors.As(readErr, &netErr) && netErr.Timeout() {
				break
			}
			if errors.Is(readErr, net.ErrClosed) {
				break
			}
			return sortedBroadcastDevices(found), readErr
		}
		dev, decErr := DecodeBroadcastPacket(buf[:n])
		if decErr != nil {
			continue
		}
		// Fallback to sender IP if packet IP is unset/invalid
		if net.ParseIP(dev.IP) == nil {
			if udpAddr, ok := addr.(*net.UDPAddr); ok && udpAddr.IP != nil {
				dev.IP = udpAddr.IP.String()
			}
		}
		found[dev.DUID] = *dev
	}
	return sortedBroadcastDevices(found), nil
}

func sortedBroadcastDevices(m map[string]BroadcastDevice) []BroadcastDevice {
	out := make([]BroadcastDevice, 0, len(m))
	for _, d := range m {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DUID < out[j].DUID
	})
	return out
}
