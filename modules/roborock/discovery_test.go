package roborock

import (
	"errors"
	"testing"
)

func TestBroadcastPacketRoundTripV1AndL01(t *testing.T) {
	cases := []BroadcastDevice{
		{DUID: "duid-q5-max", IP: "192.0.2.10", Version: Version10},
		{DUID: "duid-saros-10r", IP: "192.0.2.11", Version: VersionL01},
	}

	for _, want := range cases {
		t.Run(want.Version, func(t *testing.T) {
			raw, err := EncodeBroadcastPacket(want)
			if err != nil {
				t.Fatalf("EncodeBroadcastPacket: %v", err)
			}
			got, err := DecodeBroadcastPacket(raw)
			if err != nil {
				t.Fatalf("DecodeBroadcastPacket: %v", err)
			}
			if *got != want {
				t.Fatalf("decoded broadcast mismatch: got: %+v, want: %+v", *got, want)
			}

			// Corrupt trailing CRC byte and assert ErrChecksum.
			raw[len(raw)-1] ^= 0xFF
			if _, err := DecodeBroadcastPacket(raw); !errors.Is(err, ErrChecksum) {
				t.Fatalf("corrupted CRC error: got: %v, want: ErrChecksum", err)
			}
		})
	}
}
