package wake

import (
	"encoding/binary"
	"fmt"
	"io"
)

const bzVersion = "BZFS0221"

// BZFlag 2.4 BZFS0221 opcodes, upstream commit
// 0946efd9e405aa6e708341f6d0a7b8c187f5a4ad (include/Protocol.h).
const (
	bzMsgAccept             = 0x6163
	bzMsgAddPlayer          = 0x6170
	bzMsgAdminInfo          = 0x6169
	bzMsgAlive              = 0x616c
	bzMsgAutoPilot          = 0x6175
	bzMsgCacheURL           = 0x6375
	bzMsgCaptureFlag        = 0x6366
	bzMsgCustomSound        = 0x6373
	bzMsgDropFlag           = 0x6466
	bzMsgEnter              = 0x656e
	bzMsgExit               = 0x6578
	bzMsgFetchResources     = 0x6672
	bzMsgFlagType           = 0x6674
	bzMsgFlagUpdate         = 0x6675
	bzMsgGMUpdate           = 0x676d
	bzMsgGameSettings       = 0x6773
	bzMsgGameTime           = 0x6774
	bzMsgGetWorld           = 0x6777
	bzMsgGrabFlag           = 0x6766
	bzMsgHandicap           = 0x6863
	bzMsgKilled             = 0x6b6c
	bzMsgLagPing            = 0x7069
	bzMsgMessage            = 0x6d67
	bzMsgNearFlag           = 0x4e66
	bzMsgNegotiateFlags     = 0x6e66
	bzMsgNewRabbit          = 0x6e52
	bzMsgPause              = 0x7061
	bzMsgPlayerInfo         = 0x7062
	bzMsgPlayerUpdate       = 0x7075
	bzMsgPlayerUpdateSmall  = 0x7073
	bzMsgQueryGame          = 0x7167
	bzMsgQueryPlayers       = 0x7170
	bzMsgReject             = 0x726a
	bzMsgRemovePlayer       = 0x7270
	bzMsgReplayReset        = 0x7272
	bzMsgScore              = 0x7363
	bzMsgScoreOver          = 0x736f
	bzMsgSetVar             = 0x7376
	bzMsgShotBegin          = 0x7362
	bzMsgShotEnd            = 0x7365
	bzMsgSuperKill          = 0x736b
	bzMsgTeamUpdate         = 0x7475
	bzMsgTeleport           = 0x7470
	bzMsgTimeUpdate         = 0x746f
	bzMsgTransferFlag       = 0x7466
	bzMsgUDPLinkEstablished = 0x6f67
	bzMsgUDPLinkRequest     = 0x6f66
	bzMsgWantSettings       = 0x7773
	bzMsgWantWHash          = 0x7768
)

func bzPacket(code uint16, payload []byte) []byte {
	p := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint16(p, uint16(len(payload)))
	binary.BigEndian.PutUint16(p[2:], code)
	copy(p[4:], payload)
	return p
}

func bzRead(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(header[:]))
	if n > 1020 {
		return nil, fmt.Errorf("BZFlag packet exceeds 1024 bytes")
	}
	p := make([]byte, n+4)
	copy(p, header[:])
	_, err := io.ReadFull(r, p[4:])
	return p, err
}

func bzTranslateDatagram(packet []byte, virtual, actual byte) error {
	if len(packet) == 0 || len(packet) > 1024 {
		return fmt.Errorf("invalid BZFlag datagram")
	}
	for len(packet) > 0 {
		if len(packet) < 4 {
			return fmt.Errorf("truncated BZFlag datagram")
		}
		n := 4 + int(binary.BigEndian.Uint16(packet))
		if n > len(packet) {
			return fmt.Errorf("truncated BZFlag datagram message")
		}
		if err := bzTranslate(packet[:n], virtual, actual, true); err != nil {
			return err
		}
		packet = packet[n:]
	}
	return nil
}

// BZFS0221 assigns the local player ID before flag negotiation. Swapping that
// ID with the backend ID preserves every other player, including one already
// occupying the virtual slot. Offsets follow BZFlag 2.4's Protocol.h,
// playing.cxx, ServerLink.cxx, Flag.cxx and ShotUpdate.cxx.
func bzTranslate(packet []byte, virtual, actual byte, fromServer bool) error {
	if len(packet) < 4 || len(packet) > 1024 || int(binary.BigEndian.Uint16(packet))+4 != len(packet) {
		return fmt.Errorf("invalid BZFlag packet")
	}
	p := packet[4:]
	code := binary.BigEndian.Uint16(packet[2:])
	var offsets []int
	if fromServer {
		switch code {
		case bzMsgAccept, bzMsgScoreOver, bzMsgAddPlayer, bzMsgRemovePlayer, bzMsgAlive, bzMsgAutoPilot, bzMsgPause, bzMsgCaptureFlag, bzMsgNewRabbit, bzMsgShotEnd, bzMsgTeleport:
			offsets = []int{0}
		case bzMsgKilled, bzMsgMessage:
			offsets = []int{0, 1}
		case bzMsgGrabFlag, bzMsgDropFlag:
			offsets = []int{0, 9} // player, embedded Flag.owner
		case bzMsgTransferFlag:
			offsets = []int{0, 1, 10}
		case bzMsgFlagUpdate:
			if len(p) < 2 || len(p) != 2+57*int(binary.BigEndian.Uint16(p)) {
				return fmt.Errorf("invalid BZFlag flag list")
			}
			for i := 2; i < len(p); i += 57 {
				offsets = append(offsets, i+8)
			}
		case bzMsgHandicap, bzMsgScore, bzMsgPlayerInfo:
			stride := map[uint16]int{bzMsgHandicap: 3, bzMsgScore: 7, bzMsgPlayerInfo: 2}[code]
			if len(p) < 1 || len(p) != 1+int(p[0])*stride {
				return fmt.Errorf("invalid BZFlag player list")
			}
			for i := 1; i < len(p); i += stride {
				offsets = append(offsets, i)
			}
		case bzMsgAdminInfo:
			if len(p) < 1 {
				return fmt.Errorf("invalid BZFlag address list")
			}
			i := 1
			for count := 0; count < int(p[0]); count++ {
				if i >= len(p) || p[i] < 2 || i+2+int(p[i]) > len(p) {
					return fmt.Errorf("invalid BZFlag address entry")
				}
				offsets = append(offsets, i+1)
				i += 2 + int(p[i])
			}
			if i != len(p) {
				return fmt.Errorf("invalid BZFlag address list length")
			}
		case bzMsgPlayerUpdate, bzMsgPlayerUpdateSmall, bzMsgShotBegin:
			offsets = []int{4}
		case bzMsgGMUpdate:
			offsets = []int{0, 33}
		case bzMsgCustomSound, bzMsgCacheURL, bzMsgFlagType, bzMsgFetchResources, bzMsgGetWorld, bzMsgGameSettings, bzMsgGameTime,
			bzMsgNearFlag, bzMsgNegotiateFlags, bzMsgQueryGame, bzMsgQueryPlayers, bzMsgReject, bzMsgSuperKill, bzMsgSetVar,
			bzMsgTimeUpdate, bzMsgTeamUpdate, bzMsgWantWHash, bzMsgUDPLinkRequest, bzMsgUDPLinkEstablished, bzMsgLagPing:
		default:
			// Replay reset encodes an ID range, not individual IDs, and cannot
			// be translated by a permutation. Unknown extensions also fail closed.
			return fmt.Errorf("unsupported BZFlag server opcode %#04x", code)
		}
	} else {
		switch code {
		case bzMsgKilled, bzMsgShotEnd, bzMsgMessage, bzMsgUDPLinkRequest:
			offsets = []int{0}
		case bzMsgTransferFlag:
			offsets = []int{0, 1}
		case bzMsgPlayerUpdate, bzMsgPlayerUpdateSmall, bzMsgShotBegin:
			offsets = []int{4}
		case bzMsgGMUpdate:
			offsets = []int{0, 33}
		case bzMsgEnter, bzMsgExit, bzMsgNegotiateFlags, bzMsgGetWorld, bzMsgWantSettings, bzMsgWantWHash, bzMsgQueryGame, bzMsgQueryPlayers,
			bzMsgAlive, bzMsgGrabFlag, bzMsgDropFlag, bzMsgCaptureFlag, bzMsgTeleport, bzMsgUDPLinkEstablished, bzMsgNewRabbit, bzMsgPause, bzMsgAutoPilot, bzMsgLagPing:
		default:
			return fmt.Errorf("unsupported BZFlag client opcode %#04x", code)
		}
	}
	for _, offset := range offsets {
		if offset >= len(p) {
			return fmt.Errorf("truncated BZFlag opcode %#04x", code)
		}
	}
	for _, offset := range offsets {
		switch p[offset] {
		case virtual:
			p[offset] = actual
		case actual:
			p[offset] = virtual
		}
	}
	return nil
}
