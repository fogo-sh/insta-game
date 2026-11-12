package wake

import "bytes"

// Discard holding replies, including late arrivals after readiness. Only the
// backend supplies the Steam challenge and authenticates the client.
func goldsrcHoldingReply(packet []byte) bool {
	if !bytes.HasPrefix(packet, []byte("\xff\xff\xff\xffconnect ")) {
		return false
	}
	_, rest, ok := bytes.Cut(packet, []byte{'"'})
	if !ok {
		return false
	}
	info, _, ok := bytes.Cut(rest, []byte{'"'})
	if !ok {
		return false
	}
	fields := bytes.Split(info, []byte{'\\'})
	for i := 1; i+1 < len(fields); i += 2 {
		if bytes.Equal(fields[i], []byte("prot")) {
			return bytes.Equal(fields[i+1], []byte("2"))
		}
	}
	return false
}
