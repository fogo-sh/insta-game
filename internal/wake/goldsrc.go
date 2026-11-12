package wake

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/bits"
	"time"
)

const goldsrcReliable uint32 = 1 << 31

// GoldSrc scrambles complete four-byte payload words using the packet sequence.
func goldsrcPacket(sequence, ack uint32, payload []byte) []byte {
	packet := make([]byte, max(16, 8+len(payload)))
	binary.LittleEndian.PutUint32(packet, sequence)
	binary.LittleEndian.PutUint32(packet[4:], ack)
	copy(packet[8:], payload)
	for i := 8 + len(payload); i < len(packet); i++ {
		packet[i] = 1
	}
	table := [...]byte{0x05, 0x61, 0x7a, 0xed, 0x1b, 0xca, 0x0d, 0x9b, 0x4a, 0xf1, 0x64, 0xc7, 0xb5, 0x8e, 0xdf, 0xa0}
	mask := [...]byte{0xa5, 0xa7, 0xaf, 0xbf}
	key := uint32(byte(sequence))
	for i := 8; i+4 <= len(packet); i += 4 {
		word := bits.ReverseBytes32(binary.LittleEndian.Uint32(packet[i:]) ^ ^key)
		for j := range 4 {
			word ^= uint32(mask[j]|table[((i-8)/4+j)&15]) << (8 * j)
		}
		binary.LittleEndian.PutUint32(packet[i:], word^key)
	}
	return packet
}

// Hold only the loading channel. The real server authenticates the fresh retry;
// no Steam ticket or game sign-on is accepted by the gateway.
func (s *session) goldsrc(ctx context.Context, first []byte) error {
	s.gateway.mu.Lock()
	state := s.gateway.games[s.listener.instanceID]
	warm := state != nil && state.err == nil && state.address != "" && time.Now().Before(state.expires)
	s.gateway.mu.Unlock()
	if warm {
		return s.relay(ctx, first)
	}

	log.Printf("wake %s %s: holding GoldSrc loading channel", s.listener.instanceID, s.address)
	boot, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()
	ready := make(chan error, 1)
	s.gateway.work.Go(func() {
		_, err := s.gateway.resolve(boot, ctx, s.listener.instanceID)
		ready <- err
	})
	accepted := append([]byte{255, 255, 255, 255}, []byte(fmt.Sprintf("B 0 %q 0 8684\n\x00", s.address.String()))...)
	if err := s.send(accepted); err != nil {
		return err
	}
	var incoming, incomingReliable, reliable uint32
	sequence := uint32(1)
	var pending []byte
	var retryAt time.Time
	lastSeen, noticeAt := time.Now(), time.Time{}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	fail := func(message string) {
		payload := append(printMessage(message+"\n"), 2)
		payload = append(payload, []byte(message)...)
		payload = append(payload, 0)
		packet := goldsrcPacket(sequence, incoming|incomingReliable, payload)
		for range 3 {
			s.send(packet)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-boot.Done():
			if ctx.Err() != nil {
				return nil
			}
			fail("Server did not start in time. Please try again.")
			return boot.Err()
		case err := <-ready:
			if err != nil {
				fail("Unable to start server. Please try again.")
				return err
			}
			log.Printf("wake %s %s: backend ready; requesting fresh Steam handshake", s.listener.instanceID, s.address)
			retryAt = time.Now().Add(10 * time.Second)
		case packet := <-s.input:
			if connectionRequest("goldsrc", packet) {
				lastSeen = time.Now()
				if !retryAt.IsZero() {
					return s.relay(ctx, packet)
				}
				if err := s.send(accepted); err != nil {
					return err
				}
				continue
			}
			if len(packet) < 8 || binary.LittleEndian.Uint32(packet) == 0xffffffff {
				continue
			}
			value := binary.LittleEndian.Uint32(packet)
			current := value & 0x3fffffff
			if current <= incoming {
				continue
			}
			lastSeen = time.Now()
			incoming = current
			if value&goldsrcReliable != 0 {
				incomingReliable ^= goldsrcReliable
			}
			ack := binary.LittleEndian.Uint32(packet[4:])
			if pending != nil && ack&0x3fffffff >= sequence-1 && ack&goldsrcReliable == reliable {
				pending = nil
			}
		case now := <-ticker.C:
			if now.Sub(lastSeen) > clientTimeout {
				return errors.New("client inactive while waiting for server")
			}
			if !retryAt.IsZero() && !now.Before(retryAt) {
				fail("Server is ready. Please retry this address.")
				return errors.New("client did not retry after loading")
			}
			if pending == nil && incoming > 0 {
				payload := []byte{1}
				if !retryAt.IsZero() {
					payload = append([]byte{9}, []byte("retry\n\x00")...)
				} else if !now.Before(noticeAt) {
					payload = printMessage("Starting server... Please wait.\n")
					noticeAt = now.Add(5 * time.Second)
				}
				reliable ^= goldsrcReliable
				pending = goldsrcPacket(sequence|goldsrcReliable, incoming|incomingReliable, payload)
				sequence++
			}
			if pending != nil {
				if err := s.send(pending); err != nil {
					return err
				}
			}
		}
	}
}
