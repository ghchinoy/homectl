package roborock

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestEncodeTimestamp(t *testing.T) {
	// Python: f"{0x65a0b1c2:x}".zfill(8) -> "65a0b1c2"
	// Indices [5,6,3,7,1,2,0,4] -> '1','c','0','2','5','a','6','b' => "1c025a6b"
	got := string(EncodeTimestamp(0x65a0b1c2))
	want := "1c025a6b"
	if got != want {
		t.Fatalf("EncodeTimestamp: got: %q, want: %q", got, want)
	}
}

func TestProtocolRoundTripAllVersions(t *testing.T) {
	localKey := "0123456789abcdef"
	ackNonce := uint32(0x11223344)

	tests := []struct {
		name     string
		version  string
		payload  []byte
		prefixed bool
		opts     CodecOptions
	}{
		{
			name:     "v1.0 MQTT RPC",
			version:  Version10,
			payload:  []byte(`{"t":1700000000,"dps":{"101":"{\"id\":42,\"method\":\"get_status\",\"params\":[]}"}}`),
			prefixed: false,
			opts:     CodecOptions{LocalKey: localKey, Prefixed: false},
		},
		{
			name:     "v1.0 Local TCP Prefixed",
			version:  Version10,
			payload:  []byte(`{"dps":{"122":98,"121":8}}`),
			prefixed: true,
			opts:     CodecOptions{LocalKey: localKey, Prefixed: true},
		},
		{
			name:     "A01 CBC 16-byte aligned",
			version:  VersionA01,
			payload:  []byte(`{"dps":{"201":1}}               `)[:32],
			prefixed: false,
			opts:     CodecOptions{LocalKey: localKey},
		},
		{
			name:     "B01 CBC PKCS7",
			version:  VersionB01,
			payload:  []byte(`{"dps":{"121":8,"122":100}}`),
			prefixed: false,
			opts:     CodecOptions{LocalKey: localKey},
		},
		{
			name:     "L01 AES-256-GCM with ackNonce",
			version:  VersionL01,
			payload:  []byte(`{"t":1700000000,"dps":{"101":"{\"id\":7,\"method\":\"app_start\"}"}}`),
			prefixed: true,
			opts: CodecOptions{
				LocalKey:     localKey,
				Prefixed:     true,
				ConnectNonce: 0xdeadbeef,
				AckNonce:     &ackNonce,
			},
		},
		{
			name:     "L01 AES-256-GCM without ackNonce",
			version:  VersionL01,
			payload:  []byte(`{"hello":"saros"}`),
			prefixed: false,
			opts: CodecOptions{
				LocalKey:     localKey,
				Prefixed:     false,
				ConnectNonce: 0x12345678,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := Message{
				Version:   tc.version,
				Seq:       100042,
				Random:    54321,
				Timestamp: 1700000123,
				Protocol:  ProtoRPCRequest,
				Payload:   tc.payload,
			}
			wire, err := EncodeMessage(in, tc.opts)
			if err != nil {
				t.Fatalf("EncodeMessage failed: %v", err)
			}
			if tc.prefixed {
				declared := binary.BigEndian.Uint32(wire[:4])
				if int(declared) != len(wire)-4 {
					t.Fatalf("TCP length prefix: got: %d, want: %d", declared, len(wire)-4)
				}
			}
			msgs, rem, err := DecodeMessages(wire, tc.opts)
			if err != nil {
				t.Fatalf("DecodeMessages failed: %v", err)
			}
			if len(rem) != 0 {
				t.Fatalf("remaining bytes: got: %d, want: 0", len(rem))
			}
			if len(msgs) != 1 {
				t.Fatalf("decoded count: got: %d, want: 1", len(msgs))
			}
			got := msgs[0]
			if got.Version != in.Version || got.Seq != in.Seq || got.Random != in.Random ||
				got.Timestamp != in.Timestamp || got.Protocol != in.Protocol {
				t.Fatalf("header mismatch: got: %+v, want: %+v", got, in)
			}
			if !bytes.Equal(got.Payload, in.Payload) {
				t.Fatalf("payload mismatch: got: %q, want: %q", got.Payload, in.Payload)
			}
		})
	}
}

func TestDecodeMessagesConcatenatedNoiseAndCRC(t *testing.T) {
	opts := CodecOptions{LocalKey: "fedcba9876543210", Prefixed: false}
	m1 := Message{Version: Version10, Seq: 1, Random: 2, Timestamp: 1700000001, Protocol: ProtoRPCRequest, Payload: []byte(`{"id":1}`)}
	m2 := Message{Version: Version10, Seq: 2, Random: 3, Timestamp: 1700000002, Protocol: ProtoRPCResponse, Payload: []byte(`{"id":1,"result":["ok"]}`)}

	b1, err := EncodeMessage(m1, opts)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := EncodeMessage(m2, opts)
	if err != nil {
		t.Fatal(err)
	}

	// Leading noise + frame1 + frame2 + truncated partial frame
	stream := append([]byte{0x00, 0x00, 0x00, 0x99}, b1...)
	stream = append(stream, b2...)
	stream = append(stream, b1[:10]...)

	msgs, rem, err := DecodeMessages(stream, opts)
	if err != nil {
		t.Fatalf("DecodeMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("count: got: %d, want: 2", len(msgs))
	}
	if len(rem) != 10 {
		t.Fatalf("remaining: got: %d, want: 10", len(rem))
	}

	// Corrupt CRC32
	corrupt := append([]byte(nil), b1...)
	corrupt[len(corrupt)-1] ^= 0xFF
	if _, _, err := DecodeMessages(corrupt, opts); !errors.Is(err, ErrChecksum) {
		t.Fatalf("corrupt crc error: got: %v, want: %v", err, ErrChecksum)
	}

	// MQTT frame without trailing 4-byte CRC32 should still decode (raw+crc32 compatibility)
	noCRC := b1[:len(b1)-4]
	msgs, _, err = DecodeMessages(noCRC, opts)
	if err != nil {
		t.Fatalf("crc-less mqtt decode failed: %v", err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != `{"id":1}` {
		t.Fatalf("crc-less mqtt payload: got: %+v", msgs)
	}
}

func TestPayloadlessHelloFrame(t *testing.T) {
	opts := CodecOptions{LocalKey: "0123456789abcdef", Prefixed: true}
	hello := Message{Version: VersionL01, Seq: 1, Random: 99, Timestamp: 1700000000, Protocol: ProtoHelloRequest}
	wire, err := EncodeMessage(hello, opts)
	if err != nil {
		t.Fatalf("EncodeMessage hello failed: %v", err)
	}
	msgs, rem, err := DecodeMessages(wire, opts)
	if err != nil {
		t.Fatalf("DecodeMessages hello failed: %v", err)
	}
	if len(msgs) != 1 || len(rem) != 0 || msgs[0].Protocol != ProtoHelloRequest {
		t.Fatalf("hello decode: got: %+v rem=%d, want 1 HELLO_REQUEST", msgs, len(rem))
	}
}
