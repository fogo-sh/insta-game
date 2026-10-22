package wake

import "encoding/binary"

const (
	flagControl    = 0x80000000
	flagData       = 0x00010000
	flagAck        = 0x00020000
	flagNak        = 0x00040000
	flagEOM        = 0x00080000
	flagUnreliable = 0x00100000
)

var backendConnect = []byte{0x80, 0, 0, 12, 1, 'Q', 'U', 'A', 'K', 'E', 0, 3}

func controlRequest(packet []byte, command byte) bool {
	return len(packet) >= 12 && len(packet) <= 1024 && binary.BigEndian.Uint32(packet[:4]) == flagControl|uint32(len(packet)) && packet[4] == command && string(packet[5:11]) == "QUAKE\x00" && packet[11] == 3
}

func controlPacket(command byte, payload []byte) []byte {
	packet := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(packet, flagControl|uint32(len(packet)))
	packet[4] = command
	copy(packet[5:], payload)
	return packet
}

func accept(port int) []byte {
	var payload [4]byte
	binary.LittleEndian.PutUint32(payload[:], uint32(port))
	return controlPacket(0x81, payload[:])
}

func rejection(message string) []byte { return controlPacket(0x82, append([]byte(message), 0)) }

func dataPacket(sequence uint32, payload []byte) []byte {
	packet := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(packet, flagData|flagEOM|uint32(len(packet)))
	binary.BigEndian.PutUint32(packet[4:], sequence)
	copy(packet[8:], payload)
	return packet
}

func frame(packet []byte) (flags, sequence uint32, ok bool) {
	if len(packet) < 8 {
		return 0, 0, false
	}
	header := binary.BigEndian.Uint32(packet)
	if header&flagControl != 0 || int(header&0xffff) != len(packet) {
		return 0, 0, false
	}
	return header & 0xffff0000, binary.BigEndian.Uint32(packet[4:]), true
}

func printMessage(text string) []byte { return append(append([]byte{8}, []byte(text)...), 0) }
