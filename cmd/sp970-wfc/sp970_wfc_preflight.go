// SPDX-License-Identifier: AGPL-3.0-or-later
// SP970-B SMS-over-IMS runtime using the pinned vowifi-go SWu library.
// SIM identity and AKA are read-only; radio and profile control are excluded.
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/iniwex5/vowifi-go/engine/sim"
	"github.com/iniwex5/vowifi-go/engine/swu"
	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
)

const helper = "/data/adb/sp970-wfc/bin/sp970_uim_bridge"

var buildVersion = "dev"
var versionOnly = flag.Bool("version", false, "print the toolkit version")
var checkSIMOnly = flag.Bool("check-sim", false, "read and validate SIM policy; print only the prefix, never full identities")

var wfcHold = flag.Duration("hold", 0, "keep a temporary IMS SMS session online, at most 25m; no boot service")
var wfcSelfTest = flag.Bool("self-test", false, "send one SMS only to this SIM's carrier-confirmed own number")
var wfcRecoverSMS = flag.Bool("recover-sms", false, "import preserved incoming IMS SMS into Android's native inbox")
var runtimeOnly = flag.Bool("runtime", false, "report runtime only")
var dns = flag.String("dns", "1.1.1.1", "DNS server reachable through the selected network")
var port = flag.Int("port", 4500, "ePDG UDP port, 500 or 4500")
var ep = flag.String("epdg-ip", "", "one of the two ePDG IPv4 addresses already observed from carrier DNS")
var dh = flag.Int("dh-group", 31, "temporary compatibility choice: 14 or 31")
var akaProbe = flag.Bool("aka-probe", false, "verify real SIM AKA and carrier MAC, then stop before registration")
var imsProbe = flag.Bool("ims-probe", false, "register with the carrier, then stop unless --hold or --serve is set")

func event(stage string, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["stage"] = stage
	_ = json.NewEncoder(os.Stdout).Encode(fields)
}

func bridge(ctx context.Context, operation, input string) ([]string, error) {
	cmd := exec.CommandContext(ctx, helper, operation)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH=/system/vendor/lib:/system/lib")
	cmd.Stdin = strings.NewReader(input)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	out, err := cmd.Output()
	defer clear(out)
	if err != nil {
		return nil, fmt.Errorf("UIM %s failed: %s", operation, strings.TrimSpace(diagnostic.String()))
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "OK" {
		return nil, fmt.Errorf("UIM %s returned an invalid response", operation)
	}
	return fields[1:], nil
}

type uimAKA struct {
	imsi             string
	calls, successes int
}

func (u *uimAKA) CalculateAKA(rand16, autn16 []byte) (sim.AKAResult, error) {
	u.calls++
	if len(rand16) != 16 || len(autn16) != 16 {
		return sim.AKAResult{}, errors.New("invalid network challenge length")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
	defer cancel()
	fields, err := bridge(ctx, "auth", fmt.Sprintf("%s %x %x\n", u.imsi, rand16, autn16))
	if err != nil {
		return sim.AKAResult{}, err
	}
	if len(fields) != 1 {
		return sim.AKAResult{}, errors.New("invalid UIM authentication response")
	}
	raw, err := hex.DecodeString(fields[0])
	if err != nil || len(raw) < 2 {
		return sim.AKAResult{}, errors.New("invalid UIM authentication encoding")
	}
	defer clear(raw)
	if raw[0] == 0xdc && int(raw[1])+2 == len(raw) {
		event("aka_sync", map[string]any{"auts_length": int(raw[1])})
		return sim.AKAResult{AUTS: bytes.Clone(raw[2:])}, sim.ErrSyncFailure
	}
	if raw[0] != 0xdb {
		return sim.AKAResult{}, sim.ErrAuthFailure
	}
	pos := 1
	next := func() ([]byte, error) {
		if pos >= len(raw) {
			return nil, errors.New("truncated UIM authentication response")
		}
		n := int(raw[pos])
		pos++
		if n == 0 || pos+n > len(raw) {
			return nil, errors.New("invalid UIM authentication field")
		}
		v := bytes.Clone(raw[pos : pos+n])
		pos += n
		return v, nil
	}
	res, err := next()
	if err != nil {
		return sim.AKAResult{}, err
	}
	ck, err := next()
	if err != nil {
		return sim.AKAResult{}, err
	}
	ik, err := next()
	if err != nil {
		return sim.AKAResult{}, err
	}
	if len(res) < 4 || len(res) > 16 || len(ck) != 16 || len(ik) != 16 {
		return sim.AKAResult{}, errors.New("invalid AKA result lengths")
	}
	u.successes++
	event("aka_success", map[string]any{"res_length": len(res), "ck_length": len(ck), "ik_length": len(ik)})
	return sim.AKAResult{RES: res, CK: ck, IK: ik}, nil
}

func (u *udpIKE) ExchangeIKE(ctx context.Context, request []byte) ([]byte, error) {
	u.exchangeMu.Lock()
	defer u.exchangeMu.Unlock()
	u.startReader()
	current := request
	for attempt := 0; attempt < 3; attempt++ {
		wire := current
		if u.marker {
			wire = append([]byte{0, 0, 0, 0}, current...)
		}
		if err := u.writeWire(ctx, wire); err != nil {
			return nil, err
		}
		if len(current) >= 28 && current[18] == ikev2.ExchangeIKE_SA_INIT {
			u.lastInit = bytes.Clone(current)
		}
		packet, err := u.receiveIKE(ctx, current)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && ctx.Err() == nil && attempt < 2 {
				event("ike_retransmit", map[string]any{"attempt": attempt + 1})
				continue
			}
			return nil, err
		}
		if len(packet) < 28 {
			return nil, errors.New("truncated IKE packet")
		}
		if u.keys != nil && packet[18] == ikev2.ExchangeIKE_AUTH {
			m, inner, e := ikev2.UnprotectMessage(packet, *u.keys, false)
			if e != nil {
				event("ike_auth_decode", map[string]any{"error": e.Error()})
			} else {
				for _, p := range inner {
					if p.Type == ikev2.PayloadEAP && len(p.Body) >= 4 {
						meta := map[string]any{"message_id": m.Header.MessageID, "code": p.Body[0], "wire_length": len(p.Body), "declared_length": binary.BigEndian.Uint16(p.Body[2:4])}
						if len(p.Body) >= 5 {
							meta["method"] = p.Body[4]
						}
						if len(p.Body) >= 8 && (p.Body[4] == 23 || p.Body[4] == 50) {
							meta["subtype"] = p.Body[5]
						}
						event("eap_received", meta)
					}
					if p.Type == ikev2.PayloadNotify {
						n, e := ikev2.ParseNotify(p.Body)
						if e == nil {
							event("ike_auth_notify", map[string]any{"notify_type": n.NotifyType})
						}
					}
				}
			}
		}
		if msg, err := ikev2.ParseMessage(packet); err == nil && msg.Header.ExchangeType == ikev2.ExchangeIKE_SA_INIT {
			notifies := []uint16{}
			var requestedGroup uint16
			var cookie []byte
			for _, p := range msg.Payloads {
				if p.Type != ikev2.PayloadNotify {
					continue
				}
				n, err := ikev2.ParseNotify(p.Body)
				if err != nil {
					continue
				}
				notifies = append(notifies, n.NotifyType)
				if n.NotifyType == 16390 {
					cookie = n.NotificationData
				}
				if n.NotifyType == 17 && len(n.NotificationData) == 2 {
					requestedGroup = binary.BigEndian.Uint16(n.NotificationData)
				}
			}
			event("ike_init_response", map[string]any{"length": len(packet), "responder_spi_nonzero": msg.Header.ResponderSPI != 0, "notify_types": notifies, "requested_dh_group": requestedGroup})
			if len(cookie) > 0 {
				if len(cookie) > 64 {
					return nil, errors.New("invalid IKE cookie length")
				}
				current, err = withCookie(current, cookie)
				if err != nil {
					return nil, err
				}
				event("ike_cookie_retry", map[string]any{"attempt": attempt + 1, "cookie_length": len(cookie)})
				continue
			}
		}
		return bytes.Clone(packet), nil
	}
	return nil, errors.New("IKE cookie retry limit reached")
}

// RFC 7296 section 2.6 requires the received cookie first and all original
// IKE_SA_INIT payloads unchanged, including the initiator SPI, KE and nonce.
func withCookie(request, cookie []byte) ([]byte, error) {
	m, err := ikev2.ParseMessage(request)
	if err != nil || m.Header.ExchangeType != ikev2.ExchangeIKE_SA_INIT || len(cookie) < 1 || len(cookie) > 64 {
		return nil, errors.New("invalid IKE cookie retry input")
	}
	p, err := ikev2.NotifyPayload(ikev2.Notify{NotifyType: 16390, NotificationData: bytes.Clone(cookie)})
	if err != nil {
		return nil, err
	}
	ps := []ikev2.Payload{p}
	for _, old := range m.Payloads {
		if old.Type == ikev2.PayloadNotify {
			n, e := ikev2.ParseNotify(old.Body)
			if e == nil && n.NotifyType == 16390 {
				continue
			}
		}
		ps = append(ps, old)
	}
	m.Payloads = ps
	return m.MarshalBinary()
}

func run(parent context.Context) error {
	event("runtime", map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH})
	if *runtimeOnly {
		return nil
	}
	if net.ParseIP(*dns) == nil || (*port != 500 && *port != 4500) {
		return errors.New("invalid DNS or IKE port")
	}
	if *wfcHold < 0 || *wfcHold > 25*time.Minute || (*wfcSelfTest && *wfcHold == 0) {
		return errors.New("invalid temporary IMS session duration")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute+*wfcHold)
	defer cancel()
	fields, err := bridge(ctx, "identity", "")
	if err != nil {
		return err
	}
	if !validSIMPolicy(fields) {
		return errors.New("SIM policy guard failed")
	}
	imsi := fields[0]
	event("sim", map[string]any{"imsi_prefix": imsi[:5], "imsi_length": len(imsi), "mnc_length": 2, "flight_mode": true})
	if *checkSIMOnly {
		return nil
	}
	if *wfcRecoverSMS {
		return recoverIMSSMS(ctx)
	}
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, net.JoinHostPort(*dns, "53"))
	}}
	host := "epdg.epc.mnc087.mcc234.pub.3gppnetwork.org"
	var ips []net.IP
	if *ep != "" {
		if *ep != "84.246.113.151" && *ep != "84.246.115.151" {
			return errors.New("unverified ePDG endpoint")
		}
		ips = []net.IP{net.ParseIP(*ep)}
	} else {
		ips, err = resolver.LookupIP(ctx, "ip4", host)
		if err != nil {
			return fmt.Errorf("ePDG DNS: %w", err)
		}
	}
	if len(ips) == 0 {
		return errors.New("ePDG DNS returned no IPv4 address")
	}
	source := "live_dns"
	if *ep != "" {
		source = "previously_observed_dns"
	}
	event("epdg_dns", map[string]any{"host": host, "addresses": ips, "dns": *dns, "source": source})
	remote := net.JoinHostPort(ips[0].String(), strconv.Itoa(*port))
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "udp4", remote)
	if err != nil {
		return err
	}
	defer conn.Close()
	transport := &udpIKE{conn: conn, marker: *port == 4500}
	uim := &uimAKA{imsi: imsi}
	defer func() { event("summary", map[string]any{"aka_calls": uim.calls, "aka_successes": uim.successes}) }()
	initRunner := func(ctx context.Context, cfg ikev2.InitConfig) (ikev2.InitResult, error) {
		event("ike_init_start", map[string]any{"remote": remote})
		addr := conn.LocalAddr().(*net.UDPAddr)
		cfg.LocalIP, cfg.LocalPort = addr.IP, uint16(addr.Port)
		runner := ikev2.RunIKE_SA_INIT
		if *dh == 14 {
			runner = runMODP14Init
		} else if *dh != 31 {
			return ikev2.InitResult{}, errors.New("unsupported preflight DH group")
		}
		r, err := runner(ctx, cfg)
		if err == nil {
			// Later AUTH calculations must cover the request actually sent.
			r.RequestBytes = bytes.Clone(transport.lastInit)
			r.Request, _ = ikev2.ParseMessage(r.RequestBytes)
			transport.keys = &r.Keys
			event("ike_init_success", map[string]any{"nat_detected": r.NATDetected})
		}
		return r, err
	}
	managerConfig := swu.IKEPacketTunnelManagerConfig{
		SIM: uim, Transport: transport, ESPTransport: transport, InitRunner: initRunner,
		RemotePort: uint16(*port), RemoteIP: ips[0], UseNonESPMarker: *port == 4500,
		EAPIdentity: "0" + imsi + "@nai.epc.mnc087.mcc234.3gppnetwork.org",
	}
	var child *ikev2.ChildSAResult
	var establishedInit ikev2.InitResult
	if *akaProbe {
		managerConfig.AuthRunner = runRealAKAProbe
	} else {
		managerConfig.AuthRunner = func(ctx context.Context, cfg ikev2.FullAuthConfig) (ikev2.FullAuthResult, error) {
			r, err := runRealAKAFull(ctx, cfg)
			if err == nil {
				child, establishedInit = r.ChildSA, cfg.Init
			}
			if err == nil && r.ChildSA != nil && r.ChildSA.Configuration != nil {
				attributes := []map[string]any{}
				for _, a := range r.ChildSA.Configuration.Attributes {
					m := map[string]any{"type": a.Type, "length": len(a.Value)}
					if (a.Type == 1 || a.Type == 3 || a.Type == 20) && len(a.Value) == 4 {
						m["address"] = net.IP(a.Value).String()
					}
					if (a.Type == 10 || a.Type == 21) && len(a.Value) == 16 {
						m["address"] = net.IP(a.Value).String()
					}
					if a.Type == 8 && len(a.Value) == 17 {
						m["address"] = net.IP(a.Value[:16]).String()
						m["prefix"] = a.Value[16]
					}
					attributes = append(attributes, m)
				}
				event("ike_configuration", map[string]any{"attributes": attributes})
			}
			return r, err
		}
	}
	manager := swu.NewIKEPacketTunnelManager(managerConfig)
	sess, err := manager.EstablishTunnel(ctx, swu.TunnelConfig{
		DeviceID: "sp970b-slot1", Mode: swu.DataplaneModeUserspace,
		EPDGAddress: ips[0].String(), IMSI: imsi, MCC: "234", MNC: "87",
	})
	if err != nil {
		if errors.Is(err, errAKAReady) {
			event("preflight_complete", map[string]any{"sim_aka": true, "carrier_eap_mac": true, "wfc_registered": false})
			return nil
		}
		return errors.New(strings.ReplaceAll(strings.ReplaceAll(err.Error(), imsi, "[IMSI]"), fields[1], "[ICCID]"))
	}
	result := sess.Result()
	event("tunnel", map[string]any{"ready": result.IsReady(), "ike": result.IKEEstablished, "ipsec": result.IPsecEstablished})
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = sess.Close(closeCtx)
	}()
	if *imsProbe || *wfcHold > 0 {
		return inspectIMS(ctx, sess, transport, establishedInit, child, imsi, uim)
	}
	return nil
}

func validSIMPolicy(fields []string) bool {
	if len(fields) != 4 || len(fields[0]) != 15 || !strings.HasPrefix(fields[0], "23487") || fields[2] != "2" {
		return false
	}
	for _, digit := range fields[0] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func main() {
	flag.Parse()
	if *versionOnly {
		fmt.Println(buildVersion)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *adminMode {
		if err := serveAdmin(ctx); err != nil && ctx.Err() == nil {
			event("admin_error", map[string]any{"message": err.Error()})
			os.Exit(1)
		}
		return
	}
	if err := run(ctx); err != nil && ctx.Err() == nil {
		event("error", map[string]any{"message": err.Error()})
		os.Exit(1)
	}
}
