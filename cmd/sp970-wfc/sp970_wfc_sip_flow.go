// SPDX-License-Identifier: AGPL-3.0-or-later
// A single-reader TCP SIP flow, including server requests on the same stream.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iniwex5/vowifi-go/runtimehost/voiceclient"
)

type sipReply struct {
	response voiceclient.SIPResponse
	err      error
}

type imsWireFlow struct {
	conn    net.Conn
	ctx     context.Context
	cancel  context.CancelFunc
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan sipReply
	handle  func(context.Context, voiceclient.SIPIncomingRequest) (int, string)
}

func sipHeader(headers map[string][]string, name string) string {
	for k, values := range headers {
		if strings.EqualFold(k, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func sipKey(headers map[string][]string) string {
	return sipHeader(headers, "Call-ID") + "\x00" + sipHeader(headers, "CSeq")
}

func sipBranch() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "z9hG4bK" + hex.EncodeToString(b[:])
}

func newIMSWireFlow(ctx context.Context, local, peer string, handle func(context.Context, voiceclient.SIPIncomingRequest) (int, string)) (*imsWireFlow, error) {
	addr, err := net.ResolveTCPAddr("tcp", local)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{LocalAddr: addr, Timeout: 20 * time.Second, KeepAlive: 20 * time.Second}).DialContext(ctx, "tcp", peer)
	if err != nil {
		return nil, err
	}
	return adoptIMSWireFlow(ctx, conn, handle), nil
}

func adoptIMSWireFlow(ctx context.Context, conn net.Conn, handle func(context.Context, voiceclient.SIPIncomingRequest) (int, string)) *imsWireFlow {
	flowCtx, cancel := context.WithCancel(ctx)
	f := &imsWireFlow{conn: conn, ctx: flowCtx, cancel: cancel, pending: map[string]chan sipReply{}, handle: handle}
	go f.read()
	go func() { <-flowCtx.Done(); _ = f.Close() }()
	return f
}

func (f *imsWireFlow) write(raw []byte) error {
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	if err := f.conn.SetWriteDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	for len(raw) > 0 {
		n, err := f.conn.Write(raw)
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("SIP write made no progress")
		}
		raw = raw[n:]
	}
	return nil
}

func (f *imsWireFlow) read() {
	defer f.Close()
	r := bufio.NewReaderSize(f.conn, 4096)
	for f.ctx.Err() == nil {
		raw, err := voiceclient.ReadSIPStreamMessage(r)
		if err != nil {
			return
		}
		if bytes.HasPrefix(raw, []byte("SIP/2.0 ")) {
			resp, err := voiceclient.ParseSIPResponse(raw)
			clear(raw)
			if err != nil {
				return
			}
			if resp.StatusCode < 200 {
				continue
			}
			f.mu.Lock()
			ch := f.pending[sipKey(resp.Headers)]
			f.mu.Unlock()
			if ch != nil {
				select {
				case ch <- sipReply{response: resp}:
				default:
				}
			}
			continue
		}
		msg, err := voiceclient.ParseSIPRequest(raw)
		clear(raw)
		if err != nil {
			return
		}
		go func() {
			code, reason := 405, "Method Not Allowed"
			if f.handle != nil {
				code, reason = f.handle(f.ctx, msg)
			}
			defer clear(msg.Body)
			if msg.Method == "ACK" {
				return
			}
			headers := map[string]string{}
			to := sipHeader(msg.Headers, "To")
			if !strings.Contains(to, ";tag=") {
				headers["To"] = to + ";tag=sp970-wfc"
			}
			wire, err := voiceclient.BuildSIPResponseWire(msg, code, reason, headers, nil)
			if err == nil {
				_ = f.write(wire)
			}
		}()
	}
}

func sipRequestWire(msg voiceclient.SIPRequestMessage, local net.Addr) ([]byte, error) {
	if strings.ContainsAny(msg.Method+msg.URI, "\r\n") || msg.Method == "" || msg.URI == "" {
		return nil, errors.New("invalid SIP request line")
	}
	headers := make(map[string]string, len(msg.Headers)+2)
	for k, v := range msg.Headers {
		if strings.ContainsAny(k+v, "\r\n") || strings.Contains(k, ":") {
			return nil, errors.New("invalid SIP header")
		}
		if !strings.EqualFold(k, "Content-Length") {
			headers[k] = v
		}
	}
	if headers["Via"] == "" {
		headers["Via"] = "SIP/2.0/TCP " + local.String() + ";branch=" + sipBranch() + ";rport"
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s SIP/2.0\r\n", msg.Method, msg.URI)
	// Via first; all remaining fields have ordinary SIP field semantics.
	fmt.Fprintf(&b, "Via: %s\r\n", headers["Via"])
	delete(headers, "Via")
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if headers[k] != "" {
			fmt.Fprintf(&b, "%s: %s\r\n", k, headers[k])
		}
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(msg.Body))
	b.Write(msg.Body)
	return b.Bytes(), nil
}

func (f *imsWireFlow) RoundTripRegister(ctx context.Context, msg voiceclient.RegisterMessage) (voiceclient.RegisterResponse, error) {
	return f.RoundTripRequest(ctx, voiceclient.SIPRequestMessage{Method: "REGISTER", URI: msg.URI, Headers: msg.Headers, Body: msg.Body})
}

func (f *imsWireFlow) RoundTripRequest(ctx context.Context, msg voiceclient.SIPRequestMessage) (voiceclient.SIPResponse, error) {
	key := msg.Headers["Call-ID"] + "\x00" + msg.Headers["CSeq"]
	if key == "\x00" {
		return voiceclient.SIPResponse{}, errors.New("missing SIP transaction identity")
	}
	ch := make(chan sipReply, 1)
	f.mu.Lock()
	if _, ok := f.pending[key]; ok {
		f.mu.Unlock()
		return voiceclient.SIPResponse{}, errors.New("duplicate SIP transaction")
	}
	f.pending[key] = ch
	f.mu.Unlock()
	defer func() { f.mu.Lock(); delete(f.pending, key); f.mu.Unlock() }()
	wire, err := sipRequestWire(msg, f.conn.LocalAddr())
	if err != nil {
		return voiceclient.SIPResponse{}, err
	}
	defer clear(wire)
	if err = f.write(wire); err != nil {
		return voiceclient.SIPResponse{}, err
	}
	t := time.NewTimer(40 * time.Second)
	defer t.Stop()
	select {
	case r := <-ch:
		return r.response, r.err
	case <-ctx.Done():
		return voiceclient.SIPResponse{}, ctx.Err()
	case <-f.ctx.Done():
		return voiceclient.SIPResponse{}, errors.New("IMS TCP connection closed")
	case <-t.C:
		return voiceclient.SIPResponse{}, errors.New("IMS transaction timed out")
	}
}

func (f *imsWireFlow) SendCRLFKeepalive(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return f.write([]byte("\r\n\r\n"))
}
func (f *imsWireFlow) Close() error { f.cancel(); return f.conn.Close() }

func sipCSeq(headers map[string][]string) int {
	parts := strings.Fields(sipHeader(headers, "CSeq"))
	if len(parts) < 1 {
		return 0
	}
	n, _ := strconv.Atoi(parts[0])
	return n
}
