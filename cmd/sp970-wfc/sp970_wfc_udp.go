// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
)

// IKE and ESP must share the exact NAT-T UDP endpoint. A single reader owns
// the socket and demultiplexes the RFC 3948 non-ESP marker, never two readers.
type udpIKE struct {
	conn           net.Conn
	marker         bool
	lastInit       []byte
	keys           *ikev2.IKEKeys
	readerOnce     sync.Once
	exchangeMu     sync.Mutex
	writeMu        sync.Mutex
	ikePackets     chan []byte
	espPackets     chan []byte
	controlPackets chan []byte
	readDone       chan struct{}
	readError      error
}

func (u *udpIKE) startReader() {
	u.readerOnce.Do(func() {
		u.ikePackets = make(chan []byte, 32)
		u.espPackets = make(chan []byte, 256)
		u.controlPackets = make(chan []byte, 16)
		u.readDone = make(chan struct{})
		go func() {
			defer close(u.readDone)
			buf := make([]byte, 65535)
			for {
				n, err := u.conn.Read(buf)
				if err != nil {
					u.readError = err
					return
				}
				packet := bytes.Clone(buf[:n])
				if u.marker {
					if len(packet) == 1 && packet[0] == 255 {
						continue
					}
					if len(packet) < 4 {
						continue
					}
					if !bytes.Equal(packet[:4], []byte{0, 0, 0, 0}) {
						if len(packet) < 8 {
							continue
						}
						select {
						case u.espPackets <- packet:
						default:
							u.readError = errors.New("ESP receive queue overflow")
							return
						}
						continue
					}
					packet = packet[4:]
				}
				if len(packet) < 28 {
					continue
				}
				queue := u.ikePackets
				if packet[19]&ikev2.FlagResponse == 0 {
					queue = u.controlPackets
				}
				select {
				case queue <- packet:
				default:
					u.readError = errors.New("IKE receive queue overflow")
					return
				}
			}
		}()
	})
}

func (u *udpIKE) writeWire(ctx context.Context, packet []byte) error {
	u.writeMu.Lock()
	defer u.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(7 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if err := u.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	n, err := u.conn.Write(packet)
	if err == nil && n != len(packet) {
		return errors.New("short UDP write")
	}
	return err
}

type ikeTimeout struct{}

func (ikeTimeout) Error() string   { return "IKE response timeout" }
func (ikeTimeout) Timeout() bool   { return true }
func (ikeTimeout) Temporary() bool { return true }

func (u *udpIKE) receiveIKE(ctx context.Context, request []byte) ([]byte, error) {
	u.startReader()
	if len(request) < 28 {
		return nil, errors.New("short IKE request")
	}
	timer := time.NewTimer(7 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, ikeTimeout{}
		case <-u.readDone:
			return nil, u.readError
		case response := <-u.ikePackets:
			// Ignore delayed retransmission responses from a previous exchange.
			if !bytes.Equal(response[:8], request[:8]) || response[18] != request[18] || binary.BigEndian.Uint32(response[20:24]) != binary.BigEndian.Uint32(request[20:24]) {
				continue
			}
			return response, nil
		}
	}
}

func (u *udpIKE) SendESPPacket(ctx context.Context, packet []byte) error {
	if !u.marker || len(packet) < 8 || binary.BigEndian.Uint32(packet[:4]) == 0 {
		return errors.New("invalid NAT-T ESP packet")
	}
	return u.writeWire(ctx, packet)
}

func (u *udpIKE) ReadESPPacket(ctx context.Context) ([]byte, error) {
	u.startReader()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-u.readDone:
		return nil, u.readError
	case packet := <-u.espPackets:
		return packet, nil
	}
}

func (u *udpIKE) Close(context.Context) error { return u.conn.Close() }

func (u *udpIKE) keepNATAlive(ctx context.Context) {
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := u.writeWire(ctx, []byte{255}); err != nil {
				event("nat_keepalive_error", map[string]any{"message": err.Error()})
				return
			}
		}
	}
}

func (u *udpIKE) answerControl(ctx context.Context, init ikev2.InitResult) {
	u.startReader()
	for {
		select {
		case <-ctx.Done():
			return
		case <-u.readDone:
			return
		case raw := <-u.controlPackets:
			m, inner, err := ikev2.UnprotectMessage(raw, init.Keys, false)
			if err != nil || m.Header.InitiatorSPI != init.InitiatorSPI || m.Header.ResponderSPI != init.ResponderSPI || m.Header.ExchangeType != ikev2.ExchangeINFORMATIONAL || m.Header.Flags&(ikev2.FlagInitiator|ikev2.FlagResponse) != 0 {
				continue
			}
			if len(inner) != 0 {
				// Do not silently accept deletes or other unsupported state changes.
				event("ike_control", map[string]any{"payload_count": len(inner), "message_id": m.Header.MessageID})
				continue
			}
			h := m.Header
			h.Flags, h.NextPayload, h.Length = ikev2.FlagInitiator|ikev2.FlagResponse, 0, 0
			_, response, err := ikev2.ProtectMessage(h, init.Keys, true, nil, nil)
			if err == nil {
				err = u.writeWire(ctx, append([]byte{0, 0, 0, 0}, response...))
			}
			if err != nil {
				event("ike_control_error", map[string]any{"message": fmt.Sprint(err)})
			}
		}
	}
}
