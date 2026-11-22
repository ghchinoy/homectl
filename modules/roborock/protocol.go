package roborock

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
)

// Protocol versions used on Roborock MQTT and local TCP transports.
const (
	Version10  = "1.0"
	VersionA01 = "A01"
	VersionB01 = "B01"
	VersionL01 = "L01"
)

// Protocol salts matching python-roborock/roborock/protocol.py.
const (
	ProtocolSalt = "TXdfu$jyZ#TZHsg4"
	A01Hash      = "726f626f726f636b2d67a6d6da"
	B01Hash      = "5wwh9ikChRjASpMU8cxg7o1d2E"
)

// MessageProtocol identifies the framing protocol code in a Roborock message header.
type MessageProtocol uint16

const (
	ProtoHelloRequest      MessageProtocol = 0
	ProtoHelloResponse     MessageProtocol = 1
	ProtoPingRequest       MessageProtocol = 2
	ProtoPingResponse      MessageProtocol = 3
	ProtoGeneralRequest    MessageProtocol = 4
	ProtoGeneralResponse   MessageProtocol = 5
	ProtoRPCRequest        MessageProtocol = 101
	ProtoRPCResponse       MessageProtocol = 102
	ProtoOTAFirmwareReport MessageProtocol = 203
	ProtoMapResponsePart   MessageProtocol = 300
	ProtoMapResponse       MessageProtocol = 301
)

// String returns a human-readable protocol name.
func (p MessageProtocol) String() string {
	switch p {
	case ProtoHelloRequest:
		return "HELLO_REQUEST"
	case ProtoHelloResponse:
		return "HELLO_RESPONSE"
	case ProtoPingRequest:
		return "PING_REQUEST"
	case ProtoPingResponse:
		return "PING_RESPONSE"
	case ProtoGeneralRequest:
		return "GENERAL_REQUEST"
	case ProtoGeneralResponse:
		return "GENERAL_RESPONSE"
	case ProtoRPCRequest:
		return "RPC_REQUEST"
	case ProtoRPCResponse:
		return "RPC_RESPONSE"
	case ProtoOTAFirmwareReport:
		return "OTA_FIRMWARE_REPORT"
	case ProtoMapResponsePart:
		return "MAP_RESPONSE_PART"
	case ProtoMapResponse:
		return "MAP_RESPONSE"
	default:
		return fmt.Sprintf("P%d", uint16(p))
	}
}

// DataProtocol identifies standard DPS keys inside V1 MQTT/TCP JSON payloads.
type DataProtocol int

const (
	DPErrorCode           DataProtocol = 120
	DPState               DataProtocol = 121
	DPBattery             DataProtocol = 122
	DPFanPower            DataProtocol = 123
	DPWaterBoxMode        DataProtocol = 124
	DPMainBrushWorkTime   DataProtocol = 125
	DPSideBrushWorkTime   DataProtocol = 126
	DPFilterWorkTime      DataProtocol = 127
	DPAdditionalProps     DataProtocol = 128
	DPTaskComplete        DataProtocol = 130
	DPTaskCancelLowPower  DataProtocol = 131
	DPTaskCancelInMotion  DataProtocol = 132
	DPChargeStatus        DataProtocol = 133
	DPDryingStatus        DataProtocol = 134
	DPOfflineStatus       DataProtocol = 135
)

// Message is a decoded Roborock binary transport frame.
type Message struct {
	Version   string          `json:"version"`
	Seq       uint32          `json:"seq"`
	Random    uint32          `json:"random"`
	Timestamp uint32          `json:"timestamp"`
	Protocol  MessageProtocol `json:"protocol"`
	Payload   []byte          `json:"payload,omitempty"`
}

// CodecOptions provides encryption keys and framing options for Encode/Decode.
type CodecOptions struct {
	LocalKey     string
	Prefixed     bool    // true for local TCP (:58867), false for MQTT
	ConnectNonce uint32  // required for L01
	AckNonce     *uint32 // optional for L01 (set after HELLO_RESPONSE)
}

var (
	ErrInvalidLocalKey = errors.New("roborock: local key must be 16 bytes")
	ErrChecksum        = errors.New("roborock: crc32 checksum mismatch")
	ErrInvalidPadding  = errors.New("roborock: invalid pkcs7 padding")
	ErrTruncatedFrame  = errors.New("roborock: truncated frame")
	ErrUnsupportedVer  = errors.New("roborock: unsupported protocol version")
)

var tsPerm = [8]int{5, 6, 3, 7, 1, 2, 0, 4}

// EncodeTimestamp applies Roborock's timestamp hex-digit permutation used in 1.0 and L01 key derivation.
func EncodeTimestamp(ts uint32) []byte {
	h := fmt.Sprintf("%08x", ts)
	out := make([]byte, 8)
	for i, idx := range tsPerm {
		out[i] = h[idx]
	}
	return out
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - (len(data) % blockSize)
	out := make([]byte, len(data)+padLen)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padLen)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, ErrInvalidPadding
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, ErrInvalidPadding
	}
	for i := len(data) - padLen; i < len(data); i++ {
		if data[i] != byte(padLen) {
			return nil, ErrInvalidPadding
		}
	}
	return data[:len(data)-padLen], nil
}

func encryptAESECB(plaintext, key []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, ErrInvalidLocalKey
	}
	if len(plaintext) == 0 {
		return nil, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plaintext, aes.BlockSize)
	out := make([]byte, len(padded))
	for off := 0; off < len(padded); off += aes.BlockSize {
		block.Encrypt(out[off:off+aes.BlockSize], padded[off:off+aes.BlockSize])
	}
	return out, nil
}

func decryptAESECB(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, ErrInvalidLocalKey
	}
	if len(ciphertext) == 0 {
		return nil, nil
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidPadding
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(ciphertext))
	for off := 0; off < len(ciphertext); off += aes.BlockSize {
		block.Decrypt(out[off:off+aes.BlockSize], ciphertext[off:off+aes.BlockSize])
	}
	return pkcs7Unpad(out, aes.BlockSize)
}

func deriveV1Key(ts uint32, localKey string) []byte {
	h := md5.New()
	h.Write(EncodeTimestamp(ts))
	h.Write([]byte(localKey))
	h.Write([]byte(ProtocolSalt))
	return h.Sum(nil)
}

func deriveA01IV(random uint32) []byte {
	sum := md5.Sum([]byte(fmt.Sprintf("%08x", random) + A01Hash))
	return []byte(hex.EncodeToString(sum[:])[8:24])
}

func deriveB01IV(random uint32) []byte {
	sum := md5.Sum([]byte(fmt.Sprintf("%08x", random) + B01Hash))
	return []byte(hex.EncodeToString(sum[:])[9:25])
}

func deriveL01Key(ts uint32, localKey string) []byte {
	h := sha256.New()
	h.Write(EncodeTimestamp(ts))
	h.Write([]byte(localKey))
	h.Write([]byte(ProtocolSalt))
	return h.Sum(nil)
}

func deriveL01IV(seq, random, ts uint32) []byte {
	var buf [12]byte
	binary.BigEndian.PutUint32(buf[0:4], seq)
	binary.BigEndian.PutUint32(buf[4:8], random)
	binary.BigEndian.PutUint32(buf[8:12], ts)
	sum := sha256.Sum256(buf[:])
	return sum[:12]
}

func deriveL01AAD(seq, connectNonce uint32, ackNonce *uint32, random, ts uint32) []byte {
	size := 16
	if ackNonce != nil {
		size = 20
	}
	aad := make([]byte, 0, size)
	var b4 [4]byte
	binary.BigEndian.PutUint32(b4[:], seq)
	aad = append(aad, b4[:]...)
	binary.BigEndian.PutUint32(b4[:], connectNonce)
	aad = append(aad, b4[:]...)
	if ackNonce != nil {
		binary.BigEndian.PutUint32(b4[:], *ackNonce)
		aad = append(aad, b4[:]...)
	}
	binary.BigEndian.PutUint32(b4[:], random)
	aad = append(aad, b4[:]...)
	binary.BigEndian.PutUint32(b4[:], ts)
	aad = append(aad, b4[:]...)
	return aad
}

func encryptPayload(msg Message, opts CodecOptions) ([]byte, error) {
	if len(msg.Payload) == 0 {
		return nil, nil
	}
	switch msg.Version {
	case Version10:
		return encryptAESECB(msg.Payload, deriveV1Key(msg.Timestamp, opts.LocalKey))
	case VersionA01:
		key := []byte(opts.LocalKey)
		if len(key) != 16 {
			return nil, ErrInvalidLocalKey
		}
		if len(msg.Payload)%aes.BlockSize != 0 {
			return nil, ErrInvalidPadding
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(msg.Payload))
		cipher.NewCBCEncrypter(block, deriveA01IV(msg.Random)).CryptBlocks(out, msg.Payload)
		return out, nil
	case VersionB01:
		key := []byte(opts.LocalKey)
		if len(key) != 16 {
			return nil, ErrInvalidLocalKey
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		padded := pkcs7Pad(msg.Payload, aes.BlockSize)
		out := make([]byte, len(padded))
		cipher.NewCBCEncrypter(block, deriveB01IV(msg.Random)).CryptBlocks(out, padded)
		return out, nil
	case VersionL01:
		block, err := aes.NewCipher(deriveL01Key(msg.Timestamp, opts.LocalKey))
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		iv := deriveL01IV(msg.Seq, msg.Random, msg.Timestamp)
		aad := deriveL01AAD(msg.Seq, opts.ConnectNonce, opts.AckNonce, msg.Random, msg.Timestamp)
		return gcm.Seal(nil, iv, msg.Payload, aad), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedVer, msg.Version)
	}
}

func decryptPayload(msg Message, encrypted []byte, opts CodecOptions) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, nil
	}
	switch msg.Version {
	case Version10:
		return decryptAESECB(encrypted, deriveV1Key(msg.Timestamp, opts.LocalKey))
	case VersionA01:
		key := []byte(opts.LocalKey)
		if len(key) != 16 {
			return nil, ErrInvalidLocalKey
		}
		if len(encrypted)%aes.BlockSize != 0 {
			return nil, ErrInvalidPadding
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(encrypted))
		cipher.NewCBCDecrypter(block, deriveA01IV(msg.Random)).CryptBlocks(out, encrypted)
		return out, nil
	case VersionB01:
		key := []byte(opts.LocalKey)
		if len(key) != 16 {
			return nil, ErrInvalidLocalKey
		}
		if len(encrypted)%aes.BlockSize != 0 {
			return nil, ErrInvalidPadding
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		out := make([]byte, len(encrypted))
		cipher.NewCBCDecrypter(block, deriveB01IV(msg.Random)).CryptBlocks(out, encrypted)
		return pkcs7Unpad(out, aes.BlockSize)
	case VersionL01:
		if len(encrypted) < 16 {
			return nil, fmt.Errorf("roborock: l01 payload shorter than gcm tag")
		}
		block, err := aes.NewCipher(deriveL01Key(msg.Timestamp, opts.LocalKey))
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		iv := deriveL01IV(msg.Seq, msg.Random, msg.Timestamp)
		aad := deriveL01AAD(msg.Seq, opts.ConnectNonce, opts.AckNonce, msg.Random, msg.Timestamp)
		plain, err := gcm.Open(nil, iv, encrypted, aad)
		if err != nil {
			return nil, fmt.Errorf("roborock: l01 gcm verify failed: %w", err)
		}
		return plain, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedVer, msg.Version)
	}
}

// EncodeMessage encodes a single Roborock message frame (with optional 4-byte TCP length prefix).
func EncodeMessage(msg Message, opts CodecOptions) ([]byte, error) {
	if msg.Version == "" {
		msg.Version = Version10
	}
	if len(msg.Version) != 3 {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedVer, msg.Version)
	}
	encPayload, err := encryptPayload(msg, opts)
	if err != nil {
		return nil, err
	}
	if len(encPayload) > 0xFFFF {
		return nil, fmt.Errorf("roborock: encrypted payload exceeds uint16 max (%d)", len(encPayload))
	}

	// Body without optional 4-byte TCP length prefix:
	// 3 (ver) + 4 (seq) + 4 (rand) + 4 (ts) + 2 (proto) + [2 (len) + payload] + 4 (crc)
	// Per python-roborock MessageParser, OptionalChecksum._build always appends the 4-byte
	// IEEE CRC32 even for payloadless HELLO_REQUEST / PING_REQUEST frames (21-byte body).
	bodyCap := 17 + 4
	if len(encPayload) > 0 {
		bodyCap += 2 + len(encPayload)
	}

	body := make([]byte, 0, bodyCap)
	body = append(body, msg.Version...)
	var b4 [4]byte
	var b2 [2]byte
	binary.BigEndian.PutUint32(b4[:], msg.Seq)
	body = append(body, b4[:]...)
	binary.BigEndian.PutUint32(b4[:], msg.Random)
	body = append(body, b4[:]...)
	binary.BigEndian.PutUint32(b4[:], msg.Timestamp)
	body = append(body, b4[:]...)
	binary.BigEndian.PutUint16(b2[:], uint16(msg.Protocol))
	body = append(body, b2[:]...)

	if len(encPayload) > 0 {
		binary.BigEndian.PutUint16(b2[:], uint16(len(encPayload)))
		body = append(body, b2[:]...)
		body = append(body, encPayload...)
	}
	sum := crc32.ChecksumIEEE(body)
	binary.BigEndian.PutUint32(b4[:], sum)
	body = append(body, b4[:]...)

	if !opts.Prefixed {
		return body, nil
	}
	out := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(out[0:4], uint32(len(body)))
	copy(out[4:], body)
	return out, nil
}

var validVersionHeaders = [][]byte{
	[]byte(Version10),
	[]byte(VersionA01),
	[]byte(VersionB01),
	[]byte(VersionL01),
}

func isValidVersionHeader(b []byte) bool {
	if len(b) < 3 {
		return false
	}
	for _, v := range validVersionHeaders {
		if bytes.Equal(b[:3], v) {
			return true
		}
	}
	return false
}

func findVersionHeader(data []byte) int {
	for i := 0; i <= len(data)-3; i++ {
		if isValidVersionHeader(data[i : i+3]) {
			return i
		}
	}
	return -1
}

// DecodeMessages parses one or more Roborock frames from a byte stream, returning
// decoded messages and any trailing incomplete bytes.
func DecodeMessages(data []byte, opts CodecOptions) ([]Message, []byte, error) {
	var messages []Message
	offset := 0

	for offset < len(data) {
		rem := data[offset:]
		if len(rem) < 3 {
			break
		}
		if !isValidVersionHeader(rem[:3]) {
			idx := findVersionHeader(rem)
			if idx < 0 {
				break
			}
			offset += idx
			rem = data[offset:]
		}
		if len(rem) < 17 {
			break
		}

		msg := Message{
			Version:   string(rem[0:3]),
			Seq:       binary.BigEndian.Uint32(rem[3:7]),
			Random:    binary.BigEndian.Uint32(rem[7:11]),
			Timestamp: binary.BigEndian.Uint32(rem[11:15]),
			Protocol:  MessageProtocol(binary.BigEndian.Uint16(rem[15:17])),
		}

		// Check if payload length field is present (header-only frames are 17 bytes,
		// or 19 bytes when length == 0, optionally followed by 4-byte CRC).
		if len(rem) == 17 {
			messages = append(messages, msg)
			offset += 17
			continue
		}
		if len(rem) < 19 {
			break
		}

		// Payloadless frame with 17-byte header + trailing 4-byte CRC32 (21 bytes total,
		// used by HELLO_REQUEST, HELLO_RESPONSE, PING_REQUEST, PING_RESPONSE).
		if len(rem) >= 21 && binary.BigEndian.Uint32(rem[17:21]) == crc32.ChecksumIEEE(rem[:17]) {
			messages = append(messages, msg)
			offset += 21
			continue
		}

		// If the next bytes start a new version header, treat this as a 17-byte payloadless frame.
		if isValidVersionHeader(rem[17:]) {
			messages = append(messages, msg)
			offset += 17
			continue
		}

		payloadLen := int(binary.BigEndian.Uint16(rem[17:19]))
		if payloadLen == 0 {
			// Payloadless frame with explicit 0x0000 length (and optional trailing CRC).
			frameLen := 19
			if len(rem) >= 23 && !isValidVersionHeader(rem[19:22]) {
				expected := crc32.ChecksumIEEE(rem[:19])
				got := binary.BigEndian.Uint32(rem[19:23])
				if got == expected {
					frameLen = 23
				}
			}
			messages = append(messages, msg)
			offset += frameLen
			continue
		}

		frameWithoutCRC := 19 + payloadLen
		frameWithCRC := frameWithoutCRC + 4
		if len(rem) < frameWithoutCRC {
			break
		}

		var encrypted []byte
		if len(rem) >= frameWithCRC {
			expectedCRC := crc32.ChecksumIEEE(rem[:frameWithoutCRC])
			gotCRC := binary.BigEndian.Uint32(rem[frameWithoutCRC:frameWithCRC])
			if gotCRC != expectedCRC {
				return messages, data[offset:], fmt.Errorf("%w: got 0x%08x, want 0x%08x", ErrChecksum, gotCRC, expectedCRC)
			}
			encrypted = rem[19:frameWithoutCRC]
			offset += frameWithCRC
		} else if len(rem) == frameWithoutCRC && !opts.Prefixed {
			// Match upstream mqtt_tls_proxy_server raw+crc32 fallback for MQTT frames
			// that omit the trailing 4-byte CRC32.
			encrypted = rem[19:frameWithoutCRC]
			offset += frameWithoutCRC
		} else {
			break
		}

		plain, err := decryptPayload(msg, encrypted, opts)
		if err != nil {
			return messages, data[offset:], err
		}
		msg.Payload = plain
		messages = append(messages, msg)
	}

	var remaining []byte
	if offset < len(data) {
		remaining = make([]byte, len(data)-offset)
		copy(remaining, data[offset:])
	}
	return messages, remaining, nil
}
