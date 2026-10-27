// SPDX-License-Identifier: AGPL-3.0-or-later
// IMS AKA with TS 33.203 transport SAs. Keys stay on the device; the Netlink
// helper reads sensitive input from stdin, never argv or a saved file.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iniwex5/vowifi-go/engine/sim"
	"github.com/iniwex5/vowifi-go/runtimehost/voiceclient"
)

type imsAKAKeys struct {
	base *uimAKA
	ik   []byte
}

func (a *imsAKAKeys) CalculateAKA(r, n []byte) (sim.AKAResult, error) {
	clear(a.ik)
	a.ik = nil
	result, err := a.base.CalculateAKA(r, n)
	if err == nil {
		a.ik = bytes.Clone(result.IK)
	}
	return result, err
}

type imsXFRM struct{ undo [][]string }

// No shell is involved. Stderr is redacted before it can reach diagnostics.
func runIMSBatch(ctx context.Context, args []string, key string) error {
	get := func(name string) string {
		for i := 3; i+1 < len(args); i++ {
			if args[i] == name {
				return args[i+1]
			}
		}
		return "0"
	}
	operation := args[1] + "-" + args[2]
	line := ""
	if args[1] == "state" {
		line = get("src") + " " + get("dst") + " " + get("spi") + " " + get("reqid")
		if args[2] == "add" {
			line += " " + strings.TrimPrefix(key, "0x")
		}
	} else {
		line = get("dir") + " " + get("src") + " " + get("dst") + " " + get("sport") + " " + get("dport") + " " + get("spi") + " " + get("reqid")
	}
	input := []byte(line + "\n")
	defer clear(input)
	cmd := exec.CommandContext(ctx, "/data/adb/sp970-wfc/bin/sp970_ims_xfrm", operation)
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.CombinedOutput()
	defer clear(out)
	if err != nil {
		message := strings.TrimSpace(string(out))
		if key != "" {
			message = strings.ReplaceAll(message, key, "[IMS key]")
		}
		return fmt.Errorf("IMS IPsec %s %s: %w: %s", args[1], args[2], err, message)
	}
	return nil
}

func (x *imsXFRM) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := len(x.undo) - 1; i >= 0; i-- {
		if err := runIMSBatch(ctx, x.undo[i], ""); err != nil {
			event("ims_ipsec_cleanup_error", map[string]any{"error": err.Error()})
		}
	}
	x.undo = nil
}

func validIMSAgreement(a voiceclient.SecurityAgreement) bool {
	return a.Protocol == "ipsec-3gpp" && a.Algorithm == "hmac-sha-1-96" && a.EncryptionAlgorithm == "null" &&
		a.SPIClient > 255 && a.SPIServer > 255 && a.SPIClient != a.SPIServer &&
		a.PortClient > 0 && a.PortClient <= 65535 && a.PortServer > 0 && a.PortServer <= 65535
}

func installIMSXFRM(ctx context.Context, local, peer net.IP, ue, server voiceclient.SecurityAgreement, ik []byte) (*imsXFRM, error) {
	if local == nil || peer == nil || local.IsUnspecified() || peer.IsUnspecified() || local.IsMulticast() || peer.IsMulticast() ||
		(local.To4() == nil) != (peer.To4() == nil) || !validIMSAgreement(ue) || !validIMSAgreement(server) || len(ik) != 16 {
		return nil, errors.New("invalid or unsupported carrier IMS IPsec agreement")
	}
	var id [4]byte
	if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
		return nil, err
	}
	reqid := strconv.FormatUint(uint64(0x53000000|binary.BigEndian.Uint32(id[:])&0x00ffffff), 10)
	key := "0x" + hex.EncodeToString(ik)
	x := &imsXFRM{}
	ok := false
	defer func() {
		if !ok {
			x.close()
		}
	}()
	add := func(args, undo []string) error {
		if err := runIMSBatch(ctx, args, key); err != nil {
			return err
		}
		x.undo = append(x.undo, undo)
		return nil
	}
	// Each endpoint chooses the SPI for packets that it will receive.
	states := []struct {
		src, dst net.IP
		spi      uint32
	}{
		{local, peer, server.SPIServer}, {local, peer, server.SPIClient},
		{peer, local, ue.SPIClient}, {peer, local, ue.SPIServer},
	}
	for _, s := range states {
		selector := []string{"src", s.src.String(), "dst", s.dst.String(), "proto", "esp", "spi", strconv.FormatUint(uint64(s.spi), 10)}
		args := append([]string{"xfrm", "state", "add"}, selector...)
		args = append(args, "mode", "transport", "reqid", reqid, "replay-window", "32", "auth-trunc", "hmac(sha1)", key, "96", "enc", "ecb(cipher_null)", "0x")
		if err := add(args, append([]string{"xfrm", "state", "delete"}, selector...)); err != nil {
			return nil, err
		}
	}
	policies := []struct {
		src, dst     net.IP
		sport, dport int
		direction    string
		spi          uint32
	}{
		{local, peer, ue.PortClient, server.PortServer, "out", server.SPIServer},
		{local, peer, ue.PortServer, server.PortClient, "out", server.SPIClient},
		{peer, local, server.PortServer, ue.PortClient, "in", ue.SPIClient},
		{peer, local, server.PortClient, ue.PortServer, "in", ue.SPIServer},
	}
	for _, p := range policies {
		selector := []string{"dir", p.direction, "src", hostPrefix(p.src), "dst", hostPrefix(p.dst), "proto", "tcp", "sport", strconv.Itoa(p.sport), "dport", strconv.Itoa(p.dport)}
		args := append([]string{"xfrm", "policy", "add"}, selector...)
		args = append(args, "priority", "20970", "tmpl", "src", p.src.String(), "dst", p.dst.String(), "proto", "esp", "spi", strconv.FormatUint(uint64(p.spi), 10), "mode", "transport", "reqid", reqid, "level", "required")
		if err := add(args, append([]string{"xfrm", "policy", "delete"}, selector...)); err != nil {
			return nil, err
		}
	}
	ok = true
	event("ims_ipsec_ready", map[string]any{"mode": "transport", "algorithm": "hmac-sha-1-96", "encryption": "null", "states": 4, "policies": 4})
	return x, nil
}

type protectedIMSRegister struct {
	initial     voiceclient.RegisterResponse
	initialUsed bool
	aka         *imsAKAKeys
	local, peer net.IP
	client      voiceclient.SecurityAgreement
	headers     map[string]string
	xfrm        *imsXFRM
	flow        *imsWireFlow
	listener    net.Listener
	sms         atomic.Pointer[imsSMSHost]
}

func (p *protectedIMSRegister) close() {
	if p.listener != nil {
		_ = p.listener.Close()
	}
	if p.flow != nil {
		_ = p.flow.Close()
	}
	if p.xfrm != nil {
		p.xfrm.close()
	}
	clear(p.aka.ik)
}

func (p *protectedIMSRegister) listen(ctx context.Context) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(p.local.String(), strconv.Itoa(p.client.PortServer)))
	if err != nil {
		return err
	}
	p.listener = listener
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if address, ok := conn.RemoteAddr().(*net.TCPAddr); !ok || !address.IP.Equal(p.peer) {
				_ = conn.Close()
				continue
			}
			event("ims_inbound_connection", map[string]any{"protected_server_port": p.client.PortServer})
			_ = adoptIMSWireFlow(ctx, conn, p.handleIncoming)
		}
	}()
	return nil
}

func (p *protectedIMSRegister) handleIncoming(ctx context.Context, msg voiceclient.SIPIncomingRequest) (int, string) {
	switch msg.Method {
	case "MESSAGE":
		if host := p.sms.Load(); host != nil {
			return host.receive(ctx, msg)
		}
		return 480, "Temporarily Unavailable"
	case "OPTIONS", "NOTIFY":
		return 200, "OK"
	case "INVITE":
		return 486, "Busy Here"
	default:
		return 405, "Method Not Allowed"
	}
}

func (p *protectedIMSRegister) RoundTripRegister(ctx context.Context, msg voiceclient.RegisterMessage) (voiceclient.RegisterResponse, error) {
	if !p.initialUsed {
		p.initialUsed = true
		return p.initial, nil
	}
	for _, name := range []string{"Contact", "Supported", "User-Agent", "P-Access-Network-Info", "Proxy-Require", "Security-Client"} {
		if value := p.headers[name]; value != "" {
			msg.Headers[name] = value
		}
	}
	delete(msg.Headers, "P-Preferred-Identity")
	localPort, peerPort := 5064, 5060
	if len(p.aka.ik) == 16 {
		server, ok := voiceclient.SelectSecurityAgreement([]string{msg.Headers["Security-Verify"]}, p.client)
		if !ok {
			return voiceclient.RegisterResponse{}, errors.New("carrier supplied no compatible IMS security verification")
		}
		var err error
		if p.xfrm == nil {
			p.xfrm, err = installIMSXFRM(ctx, p.local, p.peer, p.client, server, p.aka.ik)
			if err != nil {
				return voiceclient.RegisterResponse{}, err
			}
			if err := p.listen(ctx); err != nil {
				return voiceclient.RegisterResponse{}, fmt.Errorf("IMS protected listener: %w", err)
			}
		}
		localPort, peerPort = p.client.PortClient, server.PortServer
		msg.Headers["Contact"] = strings.Replace(p.headers["Contact"], net.JoinHostPort(p.local.String(), "5060"), net.JoinHostPort(p.local.String(), strconv.Itoa(p.client.PortServer)), 1)
	}
	var branch [12]byte
	if _, err := rand.Read(branch[:]); err != nil {
		return voiceclient.RegisterResponse{}, err
	}
	msg.Headers["Via"] = "SIP/2.0/TCP " + net.JoinHostPort(p.local.String(), strconv.Itoa(localPort)) + ";branch=z9hG4bK" + hex.EncodeToString(branch[:]) + ";rport;keep"
	if p.flow == nil {
		var err error
		p.flow, err = newIMSWireFlow(ctx, net.JoinHostPort(p.local.String(), strconv.Itoa(localPort)), net.JoinHostPort(p.peer.String(), strconv.Itoa(peerPort)), p.handleIncoming)
		if err != nil {
			return voiceclient.RegisterResponse{}, err
		}
	}
	response, err := p.flow.RoundTripRegister(ctx, msg)
	if err == nil {
		details := map[string][]string{}
		for name, values := range response.Headers {
			for _, safe := range []string{"Warning", "Reason", "Retry-After", "Server", "Unsupported", "Require"} {
				if strings.EqualFold(name, safe) {
					for _, value := range values {
						details[safe] = append(details[safe], strings.ReplaceAll(value, p.aka.base.imsi, "[IMSI]"))
					}
				}
			}
		}
		event("ims_protected_response", map[string]any{"status": response.StatusCode, "reason": response.Reason, "details": details})
	}
	return response, err
}

func authenticateIMS(ctx context.Context, first voiceclient.RegisterResponse, headers map[string]string, profile voiceclient.IMSProfile, local, pcscf net.IP, uim *uimAKA) error {
	clientOffers := voiceclient.ParseSecurityAgreements([]string{headers["Security-Client"]})
	if len(clientOffers) != 1 {
		return errors.New("invalid local IMS security offer")
	}
	aka := &imsAKAKeys{base: uim}
	transport := &protectedIMSRegister{initial: first, aka: aka, local: local, peer: pcscf, client: clientOffers[0], headers: headers}
	defer transport.close()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	contact := "sip:" + strings.SplitN(profile.IMPI, "@", 2)[0] + "@" + net.JoinHostPort(local.String(), strconv.Itoa(clientOffers[0].PortServer)) + ";transport=tcp"
	session := voiceclient.RegisterSession{Transport: transport, AKAProvider: aka, Profile: profile, RegistrarURI: "sip:" + profile.Domain, ContactURI: contact, CallID: headers["Call-ID"], CNonce: hex.EncodeToString(nonce[:]), Expires: 600000, SecurityClient: clientOffers[0]}
	result, err := session.Register(ctx)
	fields := map[string]any{"status": result.StatusCode, "reason": result.Reason, "attempts": result.Attempts, "wfc_registered": result.Registered && err == nil && transport.xfrm != nil, "sms_submitted": false}
	if result.Registered {
		fields["expires"] = result.Binding.Expires
		fields["associated_identity_count"] = len(result.Binding.AssociatedURIs)
	}
	event("ims_registration", fields)
	if err != nil {
		return fmt.Errorf("IMS registration: %w", err)
	}
	if result.Registered && *wfcHold > 0 {
		duration := *wfcHold
		if result.Binding.Expires > 0 {
			if beforeExpiry := time.Duration(result.Binding.Expires) * time.Second * 3 / 4; beforeExpiry < duration {
				duration = beforeExpiry
			}
		}
		err := keepIMSSMS(ctx, transport, profile, result, duration, *wfcSelfTest)
		if err == nil && ctx.Err() == nil {
			end, cancel := context.WithTimeout(ctx, 15*time.Second)
			dereg, e := session.Deregister(end, voiceclient.DeregisterRequest{Binding: result.Binding, CallID: headers["Call-ID"], CSeq: result.NextCSeq, AuthState: result.AuthState, AuthHeader: result.AuthHeader, AuthHeaderName: result.AuthHeaderName})
			cancel()
			event("ims_deregistered", map[string]any{"status": dereg.StatusCode, "confirmed": e == nil && dereg.Deregistered})
		}
		return err
	}
	return nil
}
