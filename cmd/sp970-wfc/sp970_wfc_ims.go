// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iniwex5/vowifi-go/engine/swu"
	"github.com/iniwex5/vowifi-go/engine/swu/ikev2"
	"github.com/iniwex5/vowifi-go/runtimehost/voiceclient"
)

type imsAddresses struct {
	local, pcscf, dns []net.IP
}

func equipmentInstanceFromParcel(raw []byte) (string, error) {
	rows := regexp.MustCompile(`(?m)^\s*0x[0-9a-fA-F]{8}:\s*((?:[0-9a-fA-F]{8}[ \t]*){1,4})`).FindAllSubmatch(raw, -1)
	var data []byte
	defer func() { clear(data) }()
	for _, row := range rows {
		for _, word := range strings.Fields(string(row[1])) {
			value, err := strconv.ParseUint(word, 16, 32)
			if err != nil {
				return "", errors.New("invalid equipment identity parcel")
			}
			data = binary.LittleEndian.AppendUint32(data, uint32(value))
		}
	}
	if len(data) < 38 || binary.LittleEndian.Uint32(data[:4]) != 0 || binary.LittleEndian.Uint32(data[4:8]) != 15 {
		return "", errors.New("equipment identity unavailable")
	}
	var imei [15]byte
	defer clear(imei[:])
	for i := range imei {
		digit := binary.LittleEndian.Uint16(data[8+i*2 : 10+i*2])
		if digit < '0' || digit > '9' {
			return "", errors.New("invalid equipment identity")
		}
		imei[i] = byte(digit)
	}
	// RFC 7254: the transmitted spare digit is zero, not the stored check digit.
	return "urn:gsma:imei:" + string(imei[:8]) + "-" + string(imei[8:14]) + "-0", nil
}

func parseIMSAddresses(cp *ikev2.Configuration) (imsAddresses, error) {
	var out imsAddresses
	if cp == nil {
		return out, errors.New("ePDG did not assign IMS configuration")
	}
	for _, a := range cp.Attributes {
		var target *[]net.IP
		length := 0
		switch a.Type {
		case 1, 8:
			target = &out.local
		case 3, 10:
			target = &out.dns
		case 20, 21:
			target = &out.pcscf
		default:
			continue
		}
		if a.Type == 1 || a.Type == 3 || a.Type == 20 {
			length = 4
		} else {
			length = 16
		}
		if a.Type == 8 {
			if len(a.Value) != 17 || a.Value[16] > 128 {
				return out, errors.New("invalid IMS IPv6 address assignment")
			}
		} else if len(a.Value) != length {
			return out, fmt.Errorf("invalid IMS address attribute type %d", a.Type)
		}
		ip := net.IP(append([]byte(nil), a.Value[:length]...))
		if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() {
			return out, errors.New("invalid IMS unicast address")
		}
		duplicate := false
		for _, existing := range *target {
			duplicate = duplicate || existing.Equal(ip)
		}
		if !duplicate {
			*target = append(*target, ip)
		}
	}
	if len(out.local) == 0 || len(out.pcscf) == 0 {
		return out, errors.New("ePDG did not supply an inner address and P-CSCF")
	}
	return out, nil
}

func hostPrefix(ip net.IP) string {
	if ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ip.String() + "/128"
}

func runIP(ctx context.Context, args ...string) error {
	output, err := exec.CommandContext(ctx, "/system/bin/ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("temporary IMS interface: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Only carrier-supplied IMS destinations receive host routes. The WAN default
// and Android's saved network configuration are never replaced. Closing the
// nonpersistent TUN removes these interface-owned routes, including on exit.
func inspectIMS(ctx context.Context, session swu.TunnelSession, transport *udpIKE, init ikev2.InitResult, child *ikev2.ChildSAResult, imsi string, uim *uimAKA) error {
	if child == nil {
		return errors.New("IMS probe requires an authenticated CHILD_SA")
	}
	addresses, err := parseIMSAddresses(child.Configuration)
	if err != nil {
		return err
	}
	packetSession, ok := session.(swu.PacketTunnelReadSession)
	if !ok {
		return errors.New("tunnel cannot receive inner packets")
	}
	const iface = "spwfc0"
	if _, err := os.Stat("/sys/class/net/" + iface); !os.IsNotExist(err) {
		return errors.New("temporary IMS interface already exists or cannot be checked")
	}
	tun, err := swu.OpenTUNDevice(swu.TUNDeviceConfig{Name: iface, Path: "/dev/tun"})
	if err != nil {
		return err
	}
	defer tun.Close(context.Background())
	for _, ip := range addresses.local {
		family := "-6"
		if ip.To4() != nil {
			family = "-4"
		}
		if err := runIP(ctx, family, "addr", "add", hostPrefix(ip), "dev", iface); err != nil {
			return err
		}
	}
	if err := runIP(ctx, "link", "set", "dev", iface, "mtu", "1280", "up"); err != nil {
		return err
	}
	var local, pcscf net.IP
	for _, peer := range addresses.pcscf {
		for _, ip := range addresses.local {
			if (peer.To4() != nil) == (ip.To4() != nil) {
				local, pcscf = ip, peer
				break
			}
		}
		if pcscf != nil {
			break
		}
	}
	if pcscf == nil {
		return errors.New("IMS P-CSCF and assigned address have no common IP family")
	}
	seen := map[string]bool{}
	for _, peer := range append(append([]net.IP{}, addresses.pcscf...), addresses.dns...) {
		if seen[peer.String()] {
			continue
		}
		seen[peer.String()] = true
		family := "-6"
		if peer.To4() != nil {
			family = "-4"
		}
		if err := runIP(ctx, family, "route", "add", hostPrefix(peer), "dev", iface); err != nil {
			return err
		}
	}
	pump, err := swu.NewPacketPump(swu.PacketPumpConfig{Session: packetSession, Device: tun, OnError: func(direction swu.PacketPumpDirection, e error) {
		event("ims_dataplane_error", map[string]any{"direction": direction, "error": e.Error()})
	}})
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := pump.Start(probeCtx); err != nil {
		return err
	}
	defer func() {
		end, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = pump.Close(end)
	}()
	go transport.keepNATAlive(probeCtx)
	go transport.answerControl(probeCtx, init)
	event("ims_dataplane", map[string]any{"interface": iface, "pcscf": pcscf.String(), "wan_default_changed": false})

	// The first response establishes the carrier's actual transport and
	// authentication requirements before implementing its next exchange.
	realm := "ims.mnc087.mcc234.3gppnetwork.org"
	profile := voiceclient.IMSProfile{IMPI: imsi + "@" + realm, IMPU: "sip:" + imsi + "@" + realm, Domain: realm, LocalIP: local.String(), UserAgent: "SP970-WFC/0.1"}
	// The IMS REGISTER exceeds the 1280-byte inner MTU. RFC 3261 section
	// 18.1.1 and TS 24.229 R.3.1.6 select TCP to avoid IPv6/ESP fragmentation.
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return err
	}
	contact := "sip:" + imsi + "@" + net.JoinHostPort(local.String(), "5060") + ";transport=tcp"
	headers := voiceclient.BuildRegisterHeaders(profile, contact, hex.EncodeToString(randomID)+"@sp970-wfc", strconv.Itoa(1))
	delete(headers, "P-Preferred-Identity")
	headers["Expires"] = "600000"
	headers["Proxy-Require"] = "sec-agree"
	// TS 24.229 initial IMS registration identifies the private user even
	// before an AKA challenge: nonce and response are deliberately empty.
	headers["Authorization"] = fmt.Sprintf(`Digest username="%s", realm="%s", nonce="", uri="sip:%s", response="", algorithm=AKAv1-MD5`, profile.IMPI, realm, realm)
	headers["Security-Client"] += ";prot=esp;mod=trans"
	// Use a valid stable-for-this-registration instance URI rather than the
	// dependency's placeholder UUID, and advertise the SMS capability.
	randomID[6] = (randomID[6] & 0x0f) | 0x40
	randomID[8] = (randomID[8] & 0x3f) | 0x80
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", randomID[:4], randomID[4:6], randomID[6:8], randomID[8:10], randomID[10:])
	instance := "urn:uuid:" + uuid
	if raw, e := exec.CommandContext(ctx, "/system/bin/service", "call", "iphonesubinfo", "1").Output(); e == nil {
		if value, e := equipmentInstanceFromParcel(raw); e == nil {
			instance = value
		}
		clear(raw)
	}
	headers["Contact"] = fmt.Sprintf(`<%s>;+sip.instance="<%s>";+g.3gpp.smsip;reg-id=1`, contact, instance)
	headers["Via"] = "SIP/2.0/TCP " + net.JoinHostPort(local.String(), "5060") + ";branch=z9hG4bK" + hex.EncodeToString(randomID) + ";rport;keep"
	if status, e := exec.CommandContext(ctx, "/system/bin/wpa_cli", "-p", "/data/misc/wifi/sockets", "-i", "wlan0", "status").Output(); e == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "bssid=") {
				if mac, e := net.ParseMAC(strings.TrimSpace(strings.TrimPrefix(line, "bssid="))); e == nil && len(mac) == 6 {
					headers["P-Access-Network-Info"] = "IEEE-802.11;i-wlan-node-id=" + hex.EncodeToString(mac)
				}
			}
		}
	}
	var response voiceclient.RegisterResponse
	for _, peer := range addresses.pcscf {
		if (peer.To4() != nil) != (local.To4() != nil) {
			continue
		}
		flow := &voiceclient.WireSIPFlow{Network: "tcp", ServerAddr: net.JoinHostPort(peer.String(), "5060"), LocalAddr: net.JoinHostPort(local.String(), "5060"), Timeout: 40 * time.Second}
		response, err = flow.RoundTripRegister(ctx, voiceclient.RegisterMessage{URI: "sip:" + realm, Headers: headers})
		_ = flow.Close()
		if err == nil {
			pcscf = peer
			break
		}
		event("ims_register_transport_error", map[string]any{"pcscf": peer.String(), "transport": "tcp", "error": err.Error()})
		if ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("first IMS REGISTER: %w", err)
	}
	// Challenges and subscriber identifiers are deliberately not logged.
	schemes := []string{}
	for name, values := range response.Headers {
		if strings.EqualFold(name, "Security-Server") {
			for _, value := range values {
				schemes = append(schemes, strings.SplitN(value, ";", 2)[0])
			}
		}
	}
	options := map[string][]string{}
	for name, values := range response.Headers {
		for _, allowed := range []string{"Require", "Proxy-Require", "Unsupported", "Supported", "Warning", "Retry-After", "Server", "Reason"} {
			if strings.EqualFold(name, allowed) {
				for _, value := range values {
					options[allowed] = append(options[allowed], strings.ReplaceAll(value, imsi, "[IMSI]"))
				}
			}
		}
	}
	fields := map[string]any{"status": response.StatusCode, "reason": response.Reason, "transport": "tcp", "extension_headers": options, "security_mechanisms": schemes, "wfc_registered": false, "sms_submitted": false}
	if challenge, err := voiceclient.SelectDigestChallenge(response.Headers, "WWW-Authenticate"); err == nil {
		fields["authentication_algorithm"] = challenge.Algorithm
		fields["authentication_realm"] = challenge.Realm
	}
	event("ims_register_first_response", fields)
	if response.StatusCode == 401 || response.StatusCode == 407 {
		return authenticateIMS(ctx, response, headers, profile, local, pcscf, uim)
	}
	return nil
}
