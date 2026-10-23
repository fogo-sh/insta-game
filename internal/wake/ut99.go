package wake

// ut99Rejection closes a pending UE1 control channel with its standard FAILURE
// command. It is valid only before any backend packets have been forwarded.
//
// The LSB-first wire fields are packet ID 0 (14 bits), bunch marker 0,
// control/open/close/reliable flags 1/0/1/1, channel 0 (10 bits), reliable
// sequence 1 (10 bits), control type 1 (3 bits), payload size 72 (12 bits),
// the compact FString "\x08FAILURE\x00", and the packet terminator bit.
// These fields follow UT469f ReceivedPacket and UControlChannel::ReceivedBunch.
func ut99Rejection() []byte {
	return []byte{0x00, 0x80, 0x06, 0x20, 0x80, 0x20, 0x01, 0x82, 0x51, 0x50, 0x12, 0x53, 0x95, 0x54, 0x11, 0x40}
}
