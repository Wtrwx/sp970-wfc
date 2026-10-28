// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sgp22 "github.com/damonto/euicc-go/v2"
)

func TestAdminProxyImportPreservesPinAndEncodedCredential(t *testing.T) {
	pin := strings.Repeat("ab", 32)
	uri := "hy2://user%3Akey%40word@192.0.2.5:8443?insecure=1&sni=uk.example.com&pinSHA256=" + pin
	node, ip, err := parseProxyURI(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "192.0.2.5" || node["auth"] != "user:key@word" || node["server"] != "192.0.2.5:8443" {
		t.Fatal("node import changed credentials or address")
	}
	tls := node["tls"].(map[string]any)
	if tls["pinSHA256"] != pin || tls["sni"] != "uk.example.com" {
		t.Fatal("TLS identity lost")
	}
	for _, key := range []string{"socks5", "http"} {
		if !strings.HasPrefix(node[key].(map[string]any)["listen"].(string), "127.0.0.1:") {
			t.Fatal("non-loopback listener")
		}
	}
	rebuilt := reconstructHy2URI(node)
	node2, ip2, err := parseProxyURI(context.Background(), rebuilt)
	if err != nil {
		t.Fatalf("reconstructed URI %q failed to parse: %v", rebuilt, err)
	}
	if ip2 != ip || node2["auth"] != node["auth"] || node2["server"] != node["server"] {
		t.Fatalf("reconstructed URI %q lost fields", rebuilt)
	}
}

func TestAdminProxyRejectsUnsafeOrAmbiguousImports(t *testing.T) {
	for _, uri := range []string{
		"https://192.0.2.5", "hy2://@192.0.2.5", "hy2://user:pass@192.0.2.5",
		"hy2://x@192.0.2.5?insecure=1", "hy2://x@192.0.2.5?pinSHA256=bad",
		"hy2://x@192.0.2.5?insecure=0&insecure=1", "hy2://x@192.0.2.5?command=sh",
		"hy2://x@[2001:db8::1]", "hy2://x@192.0.2.5:0", "hy2://x@192.0.2.5?obfs=other",
		"socks5://192.0.2.5", "ss://rc4-md5:pass@192.0.2.5:8388",
	} {
		if _, err := parseAnyProxyURI(context.Background(), uri); err == nil {
			t.Errorf("accepted %q", uri)
		}
	}
}

func TestAdminMultiProtocolProxyParsing(t *testing.T) {
	s5, err := parseAnyProxyURI(context.Background(), "socks5://alice:s3cr3t@198.51.100.10:1080")
	if err != nil {
		t.Fatal(err)
	}
	if s5.Protocol != "socks5" || s5.ServerIP != "198.51.100.10" || s5.ClientConfig.Username != "alice" || s5.ClientConfig.Password != "s3cr3t" {
		t.Fatalf("unexpected socks5 parse: %+v", s5)
	}

	cred := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:my-secret-key"))
	ss1, err := parseAnyProxyURI(context.Background(), "ss://"+cred+"@203.0.113.7:8388#UK-Node")
	if err != nil {
		t.Fatal(err)
	}
	if ss1.Protocol != "ss" || ss1.ClientConfig.Method != "aes-256-gcm" || ss1.ClientConfig.Password != "my-secret-key" || ss1.ClientConfig.Server != "203.0.113.7:8388" {
		t.Fatalf("unexpected ss SIP002 parse: %+v", ss1)
	}

	legacy := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:legacy-pw@203.0.113.9:9000"))
	ss2, err := parseAnyProxyURI(context.Background(), "ss://"+legacy)
	if err != nil {
		t.Fatal(err)
	}
	if ss2.Protocol != "ss" || ss2.ClientConfig.Method != "aes-128-gcm" || ss2.ClientConfig.Password != "legacy-pw" || ss2.ClientConfig.Server != "203.0.113.9:9000" {
		t.Fatalf("unexpected ss legacy parse: %+v", ss2)
	}
}

func TestSocks5UDPAssociateAndLocalAddrType(t *testing.T) {
	// Fake target ePDG UDP server
	epdgUDP, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer epdgUDP.Close()
	go func() {
		buf := make([]byte, 512)
		n, cli, e := epdgUDP.ReadFromUDP(buf)
		if e == nil {
			_, _ = epdgUDP.WriteToUDP(append([]byte("pong:"), buf[:n]...), cli)
		}
	}()

	// Fake SOCKS5 UDP relay
	relayUDP, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer relayUDP.Close()
	go func() {
		buf := make([]byte, 1024)
		n, clientAddr, e := relayUDP.ReadFromUDP(buf)
		if e != nil || n < 10 {
			return
		}
		dstIP := net.IP(buf[4:8])
		dstPort := int(binary.BigEndian.Uint16(buf[8:10]))
		outConn, e := net.DialUDP("udp4", nil, &net.UDPAddr{IP: dstIP, Port: dstPort})
		if e != nil {
			return
		}
		defer outConn.Close()
		_, _ = outConn.Write(buf[10:n])
		rb := make([]byte, 512)
		rn, e := outConn.Read(rb)
		if e != nil {
			return
		}
		resp := append([]byte{0, 0, 0, 1, 127, 0, 0, 1, 0, 0}, rb[:rn]...)
		_, _ = relayUDP.WriteToUDP(resp, clientAddr)
	}()

	// Fake SOCKS5 TCP control server with user/pass auth
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		var greet [4]byte
		_, _ = io.ReadFull(c, greet[:])
		_, _ = c.Write([]byte{0x05, 0x02})
		var ulen [2]byte
		_, _ = io.ReadFull(c, ulen[:])
		ubuf := make([]byte, ulen[1]+1)
		_, _ = io.ReadFull(c, ubuf)
		pbuf := make([]byte, ubuf[len(ubuf)-1])
		_, _ = io.ReadFull(c, pbuf)
		_, _ = c.Write([]byte{0x01, 0x00})

		var req [10]byte
		_, _ = io.ReadFull(c, req[:])
		var rep [10]byte
		rep[0], rep[1], rep[3] = 0x05, 0x00, 0x01
		copy(rep[4:8], []byte{0, 0, 0, 0}) // 0.0.0.0 BND.ADDR test
		binary.BigEndian.PutUint16(rep[8:10], uint16(relayUDP.LocalAddr().(*net.UDPAddr).Port))
		_, _ = c.Write(rep[:])
		time.Sleep(2 * time.Second)
	}()

	conn, err := dialSocks5UDP(context.Background(), proxyClientConfig{
		Protocol: "socks5",
		Server:   ln.Addr().String(),
		Username: "u",
		Password: "p",
	}, epdgUDP.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, ok := conn.LocalAddr().(*net.UDPAddr); !ok {
		t.Fatalf("LocalAddr %T is not *net.UDPAddr", conn.LocalAddr())
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("ike-init")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 64)
	n, err := conn.Read(got)
	if err != nil || !bytes.Equal(got[:n], []byte("pong:ike-init")) {
		t.Fatalf("unexpected UDP reply: %q, err=%v", got[:n], err)
	}
}

func TestEuiccProfileViewPreservesFullICCID(t *testing.T) {
	iccid, err := sgp22.NewICCID("8944501201209348240")
	if err != nil {
		t.Fatal(err)
	}
	v := profileView(&sgp22.ProfileInfo{
		ISDPAID:             []byte{0xa0, 0x00, 0x00, 0x05, 0x59},
		ICCID:               iccid,
		ProfileNickname:     "UK Travel",
		ServiceProviderName: "Lebara",
		ProfileState:        sgp22.ProfileEnabled,
	})
	if strings.Contains(v.ICCID, "*") || v.ICCID != "8944501201209348240" || v.Nickname != "UK Travel" {
		t.Fatalf("ICCID or nickname altered: %+v", v)
	}
}

func TestEuiccActivationParsing(t *testing.T) {
	ac, err := parseEuiccActivation(euiccDownloadRequest{
		Code:             "LPA:1$rsp.truphone.com$ABCD-1234",
		ConfirmationCode: "9988",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if ac.SMDP.Host != "rsp.truphone.com" || ac.MatchingID != "ABCD-1234" || ac.ConfirmationCode != "9988" || ac.IMEI != fallbackEuiccIMEI {
		t.Fatalf("unexpected LPA parse: %+v", ac)
	}

	ac2, err := parseEuiccActivation(euiccDownloadRequest{
		SMDP:       "smdp.example.com",
		MatchingID: "MATCH-9",
	}, "356938035643809")
	if err != nil {
		t.Fatal(err)
	}
	if ac2.SMDP.Host != "smdp.example.com" || ac2.MatchingID != "MATCH-9" || ac2.IMEI != "356938035643809" {
		t.Fatalf("unexpected manual SM-DP+ parse: %+v", ac2)
	}

	if _, err := parseEuiccActivation(euiccDownloadRequest{Code: "not-an-lpa-code"}, ""); err == nil {
		t.Fatal("accepted invalid LPA code")
	}
}

func TestAdminRejectsUntrustedMutationBeforeDeviceCalls(t *testing.T) {
	host := &adminHost{}
	for _, path := range []string{
		"/flight", "/wifi", "/wifi/connect", "/wifi/disconnect", "/wifi/forget",
		"/hotspot", "/hotspot/config", "/usb-tether", "/mobile-data", "/data-roaming",
		"/wfc", "/network", "/euicc/enable", "/euicc/disable", "/euicc/nickname", "/euicc/delete", "/euicc/download",
		"/tailscale", "/tailscale/login", "/tailscale/logout",
	} {
		r := httptest.NewRequest("POST", "http://device.local"+path, strings.NewReader(`{"enabled":false}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		host.api(w, r)
		if w.Code != 401 {
			t.Errorf("%s: expected 401, got %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://device.local/flight", strings.NewReader(`{"enabled":false}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://untrusted.example")
	w := httptest.NewRecorder()
	host.api(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
}

func TestIMSDoesNotAcceptRevokedMemorySession(t *testing.T) {
	token := strings.Repeat("a", 64)
	host := &imsSMSHost{sessions: map[string]time.Time{token: time.Now().Add(time.Hour)}}
	request := httptest.NewRequest("GET", "http://device.local/status", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if host.authorized(request) {
		t.Fatal("revoked shared session accepted from process cache")
	}
}
