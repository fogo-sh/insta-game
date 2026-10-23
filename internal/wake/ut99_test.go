package wake

import (
	"encoding/binary"
	"testing"
)

func TestUT99RejectionClosesInitialReliableControlChannel(t *testing.T) {
	packet := ut99Rejection()
	// Decode the header independently as a little-endian bit field. The
	// payload starts at bit54, so strings are not byte aligned on the wire.
	header := binary.LittleEndian.Uint64(packet)
	fields := []struct {
		name               string
		shift, width, want uint
	}{
		{"packet", 0, 14, 0}, {"bunch", 14, 1, 0},
		{"control", 15, 1, 1}, {"open", 16, 1, 0},
		{"close", 17, 1, 1}, {"reliable", 18, 1, 1},
		{"channel", 19, 10, 0}, {"sequence", 29, 10, 1},
		{"type", 39, 3, 1}, {"payload bits", 42, 12, 72},
	}
	for _, field := range fields {
		if got := (header >> field.shift) & ((1 << field.width) - 1); got != uint64(field.want) {
			t.Errorf("%s = %d, want %d", field.name, got, field.want)
		}
	}
	payload := make([]byte, 9)
	for i := range payload {
		bit := 54 + i*8
		payload[i] = packet[bit/8]>>6 | packet[bit/8+1]<<2
	}
	if string(payload) != "\x08FAILURE\x00" {
		t.Fatalf("control FString = %q", payload)
	}
	if len(packet) != 16 || packet[15] != 0x40 {
		t.Fatal("missing packet terminator after the control payload")
	}
}
