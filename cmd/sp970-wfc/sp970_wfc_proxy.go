// SPDX-License-Identifier: AGPL-3.0-or-later
// Multi-protocol proxy support (SOCKS5 TCP+UDP, Hysteria2 TUN, Shadowsocks/SS2022 TCP+UDP).
package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type proxyClientConfig struct {
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"`
	WFCOnly  bool   `json:"wfc_only,omitempty"`
}

type parsedProxy struct {
	Protocol     string
	CanonicalURI string
	ServerIP     string
	ServerPort   int
	Hy2Config    map[string]any
	ClientConfig proxyClientConfig
}

func resolveNodeIPv4(ctx context.Context, host string) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		resolver := deviceResolver()
		rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		addresses, err := resolver.LookupIP(rctx, "ip4", host)
		if err != nil || len(addresses) == 0 {
			return "", errors.New("无法解析节点 IPv4，请使用 IPv4 节点链接")
		}
		ip = addresses[0]
	}
	if ip.To4() == nil {
		return "", errors.New("此设备的节点需提供 IPv4")
	}
	return ip.To4().String(), nil
}

func parseAnyProxyURI(ctx context.Context, rawURI string) (parsedProxy, error) {
	trimmed := strings.TrimSpace(rawURI)
	if trimmed == "" || len(trimmed) > 4096 || strings.ContainsAny(trimmed, "\r\n\x00") {
		return parsedProxy{}, errors.New("代理节点链接无效")
	}
	scheme, _, ok := strings.Cut(trimmed, "://")
	if !ok {
		return parsedProxy{}, errors.New("代理链接需以 socks5://、hy2:// 或 ss:// 开头")
	}
	switch strings.ToLower(scheme) {
	case "hy2", "hysteria2":
		cfg, ip, err := parseProxyURI(ctx, trimmed)
		if err != nil {
			return parsedProxy{}, err
		}
		_, pStr, _ := net.SplitHostPort(cfg["server"].(string))
		p, _ := strconv.Atoi(pStr)
		return parsedProxy{
			Protocol:     "hysteria2",
			CanonicalURI: trimmed,
			ServerIP:     ip,
			ServerPort:   p,
			Hy2Config:    cfg,
			ClientConfig: proxyClientConfig{Protocol: "hysteria2", Server: cfg["server"].(string)},
		}, nil
	case "socks5", "socks5h", "socks":
		return parseSocks5URI(ctx, trimmed)
	case "ss":
		return parseShadowsocksURI(ctx, trimmed)
	default:
		return parsedProxy{}, fmt.Errorf("不支持的代理协议：%s（支持 socks5://、hy2://、ss://）", scheme)
	}
}

func parseSocks5URI(ctx context.Context, trimmed string) (parsedProxy, error) {
	invalid := errors.New("SOCKS5 链接无效（格式：socks5://[用户名:密码@]IP:端口）")
	u, err := url.Parse(trimmed)
	if err != nil || u.Hostname() == "" || u.RawQuery != "" {
		return parsedProxy{}, invalid
	}
	portStr := u.Port()
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return parsedProxy{}, invalid
	}
	var user, pass string
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
		if len(user) > 255 || len(pass) > 255 {
			return parsedProxy{}, errors.New("SOCKS5 账号或密码长度超出限制")
		}
	}
	ip, err := resolveNodeIPv4(ctx, u.Hostname())
	if err != nil {
		return parsedProxy{}, err
	}
	srv := net.JoinHostPort(ip, strconv.Itoa(port))
	return parsedProxy{
		Protocol:     "socks5",
		CanonicalURI: trimmed,
		ServerIP:     ip,
		ServerPort:   port,
		ClientConfig: proxyClientConfig{
			Protocol: "socks5",
			Server:   srv,
			Username: user,
			Password: pass,
		},
	}, nil
}

func decodeBase64Any(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("invalid base64")
}

func parseShadowsocksURI(ctx context.Context, trimmed string) (parsedProxy, error) {
	invalid := errors.New("Shadowsocks 链接无效")
	body := trimmed[len("ss://"):]
	if idx := strings.IndexByte(body, '#'); idx >= 0 {
		body = body[:idx]
	}
	if idx := strings.IndexByte(body, '?'); idx >= 0 {
		// Reject SIP003 plugins since no external plugin binary exists on device.
		if body[idx+1:] != "" {
			return parsedProxy{}, errors.New("暂不支持带插件参数的 ss:// 链接")
		}
		body = body[:idx]
	}
	body = strings.TrimSuffix(body, "/")
	var method, password, hostPort string
	if at := strings.LastIndexByte(body, '@'); at > 0 {
		credPart := body[:at]
		hostPort = body[at+1:]
		if m, p, ok := strings.Cut(credPart, ":"); ok {
			um, err1 := url.PathUnescape(m)
			up, err2 := url.PathUnescape(p)
			if err1 != nil || err2 != nil {
				return parsedProxy{}, invalid
			}
			method, password = um, up
		} else {
			dec, err := decodeBase64Any(credPart)
			if err != nil {
				return parsedProxy{}, invalid
			}
			m2, p2, ok2 := strings.Cut(string(dec), ":")
			clear(dec)
			if !ok2 {
				return parsedProxy{}, invalid
			}
			method, password = m2, p2
		}
	} else {
		dec, err := decodeBase64Any(body)
		if err != nil {
			return parsedProxy{}, invalid
		}
		full := string(dec)
		clear(dec)
		at2 := strings.LastIndexByte(full, '@')
		if at2 <= 0 {
			return parsedProxy{}, invalid
		}
		m2, p2, ok2 := strings.Cut(full[:at2], ":")
		if !ok2 {
			return parsedProxy{}, invalid
		}
		method, password, hostPort = m2, p2, full[at2+1:]
	}
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "aes-128-gcm", "aes-256-gcm":
	case "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		if _, err := newSS2022Method(proxyClientConfig{Method: method, Password: password}); err != nil {
			return parsedProxy{}, err
		}
	default:
		return parsedProxy{}, errors.New("Shadowsocks 支持 aes-128-gcm、aes-256-gcm 和 SS2022")
	}
	if password == "" {
		return parsedProxy{}, invalid
	}
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil || host == "" {
		return parsedProxy{}, invalid
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return parsedProxy{}, invalid
	}
	ip, err := resolveNodeIPv4(ctx, host)
	if err != nil {
		return parsedProxy{}, err
	}
	srv := net.JoinHostPort(ip, strconv.Itoa(port))
	return parsedProxy{
		Protocol:     "ss",
		CanonicalURI: trimmed,
		ServerIP:     ip,
		ServerPort:   port,
		ClientConfig: proxyClientConfig{
			Protocol: "ss",
			Server:   srv,
			Password: password,
			Method:   method,
		},
	}, nil
}

func reconstructHy2URI(node map[string]any) string {
	if node == nil {
		return ""
	}
	server, _ := node["server"].(string)
	auth, _ := node["auth"].(string)
	if server == "" || auth == "" {
		return ""
	}
	q := url.Values{}
	if tlsMap, ok := node["tls"].(map[string]any); ok {
		if sni, _ := tlsMap["sni"].(string); sni != "" {
			host, _, _ := net.SplitHostPort(server)
			if sni != host {
				q.Set("sni", sni)
			}
		}
		if ins, _ := tlsMap["insecure"].(bool); ins {
			q.Set("insecure", "1")
		}
		if pin, _ := tlsMap["pinSHA256"].(string); pin != "" {
			q.Set("pinSHA256", pin)
		}
	}
	if obfsMap, ok := node["obfs"].(map[string]any); ok {
		if t, _ := obfsMap["type"].(string); t == "salamander" {
			if sal, ok2 := obfsMap["salamander"].(map[string]any); ok2 {
				if pw, _ := sal["password"].(string); pw != "" {
					q.Set("obfs", "salamander")
					q.Set("obfs-password", pw)
				}
			}
		}
	}
	escapedAuth := strings.ReplaceAll(url.UserPassword(auth, "").String(), ":", "")
	// url.User(auth).String() encodes '@', ':', '/', '?' safely without trailing ':'.
	escapedAuth = url.User(auth).String()
	// Note: Go's url.User leaves ':' unescaped in userinfo; parseProxyURI requires %3A if ':' is present.
	escapedAuth = strings.ReplaceAll(escapedAuth, ":", "%3A")
	out := "hy2://" + escapedAuth + "@" + server
	if encoded := q.Encode(); encoded != "" {
		out += "?" + encoded
	}
	return out
}

func loadRuntimeProxyClient() (*proxyClientConfig, string) {
	mode := readSetting("network-mode")
	if mode != "socks5" && mode != "ss" {
		return nil, mode
	}
	raw, err := os.ReadFile(adminBase + "/proxy-client.json")
	if err != nil {
		return nil, mode
	}
	defer clear(raw)
	var cfg proxyClientConfig
	if json.Unmarshal(raw, &cfg) != nil || cfg.Protocol != mode || cfg.Server == "" {
		return nil, mode
	}
	return &cfg, mode
}

// dialEPDGUDP opens the UDP transport to the carrier ePDG, either directly (or via hypsm0 TUN)
// or through SOCKS5 UDP ASSOCIATE / Shadowsocks AEAD UDP.
func dialEPDGUDP(ctx context.Context, remote string) (net.Conn, error) {
	cfg, mode := loadRuntimeProxyClient()
	return dialRuntimeUDP(ctx, cfg, mode, remote)
}

func dialRuntimeUDP(ctx context.Context, cfg *proxyClientConfig, mode, remote string) (net.Conn, error) {
	if cfg != nil && cfg.WFCOnly {
		_, port, err := net.SplitHostPort(remote)
		if err != nil {
			return nil, err
		}
		if port != "500" && port != "4500" {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "udp4", remote)
		}
	}
	if mode == "socks5" {
		if cfg == nil {
			return nil, errors.New("SOCKS5 代理配置无效")
		}
		return dialSocks5UDP(ctx, *cfg, remote)
	}
	if mode == "ss" {
		if cfg == nil {
			return nil, errors.New("Shadowsocks 代理配置无效")
		}
		return dialShadowsocksUDP(ctx, *cfg, remote)
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "udp4", remote)
}

// WFC-only nodes carry IPsec UDP only. HTTPS forwarding and eSIM downloads
// use the local network, since a restricted node rejects application traffic.
func dialForwardTCP(ctx context.Context, resolver *net.Resolver, address string) (net.Conn, error) {
	cfg, mode := loadRuntimeProxyClient()
	return dialRuntimeTCP(ctx, cfg, mode, resolver, address)
}

func dialRuntimeTCP(ctx context.Context, cfg *proxyClientConfig, mode string, resolver *net.Resolver, address string) (net.Conn, error) {
	if cfg != nil && cfg.WFCOnly {
		return (&net.Dialer{Timeout: 8 * time.Second, Resolver: resolver}).DialContext(ctx, "tcp4", address)
	}
	if mode == "socks5" && cfg != nil {
		return dialSocks5TCP(ctx, *cfg, address)
	}
	if mode == "ss" && cfg != nil {
		return dialShadowsocksTCP(ctx, *cfg, address)
	}
	return (&net.Dialer{Timeout: 8 * time.Second, Resolver: resolver}).DialContext(ctx, "tcp4", address)
}

// --- SOCKS5 RFC 1928 / RFC 1929 ---

func socks5Handshake(conn net.Conn, username, password string) error {
	if username != "" || password != "" {
		if _, err := conn.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
			return err
		}
	} else {
		if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
			return err
		}
	}
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return err
	}
	if greeting[0] != 0x05 {
		return errors.New("SOCKS5 协议响应无效")
	}
	switch greeting[1] {
	case 0x00:
		return nil
	case 0x02:
		if len(username) > 255 || len(password) > 255 {
			return errors.New("SOCKS5 认证参数过长")
		}
		authReq := make([]byte, 0, 3+len(username)+len(password))
		authReq = append(authReq, 0x01, byte(len(username)))
		authReq = append(authReq, username...)
		authReq = append(authReq, byte(len(password)))
		authReq = append(authReq, password...)
		if _, err := conn.Write(authReq); err != nil {
			clear(authReq)
			return err
		}
		clear(authReq)
		var authResp [2]byte
		if _, err := io.ReadFull(conn, authResp[:]); err != nil {
			return err
		}
		if authResp[1] != 0x00 {
			return errors.New("SOCKS5 用户名或密码错误")
		}
		return nil
	default:
		return errors.New("SOCKS5 服务器拒绝认证方式")
	}
}

func encodeSocksTarget(address string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("invalid target port")
	}
	var buf []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01)
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 0x04)
			buf = append(buf, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, errors.New("invalid target host")
		}
		buf = append(buf, 0x03, byte(len(host)))
		buf = append(buf, host...)
	}
	var pb [2]byte
	binary.BigEndian.PutUint16(pb[:], uint16(port))
	buf = append(buf, pb[:]...)
	return buf, nil
}

func readSocksReplyAddr(r io.Reader) (net.IP, int, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, 0, err
	}
	if hdr[0] != 0x05 || hdr[1] != 0x00 {
		return nil, 0, fmt.Errorf("SOCKS5 请求失败 (code=%d)", hdr[1])
	}
	var ip net.IP
	switch hdr[3] {
	case 0x01:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, 0, err
		}
		ip = net.IP(b[:])
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return nil, 0, err
		}
		dom := make([]byte, int(l[0]))
		if _, err := io.ReadFull(r, dom); err != nil {
			return nil, 0, err
		}
		ip = net.ParseIP(string(dom))
	case 0x04:
		var b [16]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, 0, err
		}
		ip = net.IP(b[:])
	default:
		return nil, 0, errors.New("SOCKS5 地址类型不支持")
	}
	var pb [2]byte
	if _, err := io.ReadFull(r, pb[:]); err != nil {
		return nil, 0, err
	}
	return ip, int(binary.BigEndian.Uint16(pb[:])), nil
}

func dialSocks5TCP(ctx context.Context, cfg proxyClientConfig, target string) (net.Conn, error) {
	addrBytes, err := encodeSocksTarget(target)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", cfg.Server)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	if err := socks5Handshake(conn, cfg.Username, cfg.Password); err != nil {
		conn.Close()
		return nil, err
	}
	req := append([]byte{0x05, 0x01, 0x00}, addrBytes...)
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	if _, _, err := readSocksReplyAddr(conn); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

type socks5UDPConn struct {
	udp        *net.UDPConn
	ctrl       net.Conn
	remoteAddr *net.UDPAddr
	header     []byte
	closeOnce  sync.Once
}

func dialSocks5UDP(ctx context.Context, cfg proxyClientConfig, remote string) (net.Conn, error) {
	rHost, rPortStr, err := net.SplitHostPort(remote)
	if err != nil {
		return nil, err
	}
	rPort, err := strconv.Atoi(rPortStr)
	if err != nil {
		return nil, err
	}
	rIP := net.ParseIP(rHost).To4()
	if rIP == nil {
		return nil, errors.New("ePDG target must be IPv4")
	}
	targetHdr, err := encodeSocksTarget(remote)
	if err != nil {
		return nil, err
	}
	ctrl, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", cfg.Server)
	if err != nil {
		return nil, err
	}
	_ = ctrl.SetDeadline(time.Now().Add(8 * time.Second))
	if err := socks5Handshake(ctrl, cfg.Username, cfg.Password); err != nil {
		ctrl.Close()
		return nil, err
	}
	// UDP ASSOCIATE with 0.0.0.0:0
	if _, err := ctrl.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		ctrl.Close()
		return nil, err
	}
	bndIP, bndPort, err := readSocksReplyAddr(ctrl)
	if err != nil {
		ctrl.Close()
		return nil, errors.New("SOCKS5 节点不支持 UDP 转发 (UDP ASSOCIATE)")
	}
	_ = ctrl.SetDeadline(time.Time{})
	srvHost, _, _ := net.SplitHostPort(cfg.Server)
	relayIP := net.ParseIP(srvHost).To4()
	if bndIP != nil && !bndIP.IsUnspecified() && !bndIP.IsLoopback() {
		if v4 := bndIP.To4(); v4 != nil {
			relayIP = v4
		}
	} else if bndIP != nil && bndIP.IsLoopback() && net.ParseIP(srvHost).IsLoopback() {
		relayIP = net.IPv4(127, 0, 0, 1)
	}
	if relayIP == nil || bndPort < 1 || bndPort > 65535 {
		ctrl.Close()
		return nil, errors.New("SOCKS5 UDP 中继地址无效")
	}
	udpConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: relayIP, Port: bndPort})
	if err != nil {
		ctrl.Close()
		return nil, err
	}
	c := &socks5UDPConn{
		udp:        udpConn,
		ctrl:       ctrl,
		remoteAddr: &net.UDPAddr{IP: rIP, Port: rPort},
		header:     append([]byte{0x00, 0x00, 0x00}, targetHdr...),
	}
	go func() {
		var sink [16]byte
		for {
			if _, e := ctrl.Read(sink[:]); e != nil {
				_ = udpConn.Close()
				return
			}
		}
	}()
	return c, nil
}

func (c *socks5UDPConn) Read(b []byte) (int, error) {
	buf := make([]byte, max(len(b)+262, 2048))
	for {
		n, err := c.udp.Read(buf)
		if err != nil {
			return 0, err
		}
		if n < 10 || buf[2] != 0x00 {
			continue
		}
		var offset int
		switch buf[3] {
		case 0x01:
			offset = 10
		case 0x03:
			offset = 7 + int(buf[4])
		case 0x04:
			offset = 22
		default:
			continue
		}
		if n < offset {
			continue
		}
		return copy(b, buf[offset:n]), nil
	}
}

func (c *socks5UDPConn) Write(b []byte) (int, error) {
	pkt := make([]byte, len(c.header)+len(b))
	copy(pkt, c.header)
	copy(pkt[len(c.header):], b)
	if _, err := c.udp.Write(pkt); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *socks5UDPConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		_ = c.ctrl.Close()
		err = c.udp.Close()
	})
	return err
}

func (c *socks5UDPConn) LocalAddr() net.Addr                { return c.udp.LocalAddr() }
func (c *socks5UDPConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *socks5UDPConn) SetDeadline(t time.Time) error      { return c.udp.SetDeadline(t) }
func (c *socks5UDPConn) SetReadDeadline(t time.Time) error  { return c.udp.SetReadDeadline(t) }
func (c *socks5UDPConn) SetWriteDeadline(t time.Time) error { return c.udp.SetWriteDeadline(t) }

// --- Shadowsocks AEAD (aes-128-gcm / aes-256-gcm) ---

func ssKeySize(method string) (int, error) {
	switch method {
	case "aes-128-gcm":
		return 16, nil
	case "aes-256-gcm":
		return 32, nil
	default:
		return 0, errors.New("unsupported Shadowsocks cipher")
	}
}

func ssDeriveMasterKey(password string, keyLen int) []byte {
	var out, prev []byte
	pw := []byte(password)
	for len(out) < keyLen {
		h := md5.New()
		h.Write(prev)
		h.Write(pw)
		prev = h.Sum(nil)
		out = append(out, prev...)
	}
	return out[:keyLen]
}

func hkdfSHA1(secret, salt, info []byte, outLen int) []byte {
	prkMac := hmac.New(sha1.New, salt)
	prkMac.Write(secret)
	prk := prkMac.Sum(nil)
	var out, prev []byte
	for counter := byte(1); len(out) < outLen; counter++ {
		m := hmac.New(sha1.New, prk)
		m.Write(prev)
		m.Write(info)
		m.Write([]byte{counter})
		prev = m.Sum(nil)
		out = append(out, prev...)
	}
	clear(prk)
	return out[:outLen]
}

func newSSAEAD(masterKey, salt []byte) (cipher.AEAD, error) {
	subkey := hkdfSHA1(masterKey, salt, []byte("ss-subkey"), len(masterKey))
	defer clear(subkey)
	block, err := aes.NewCipher(subkey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type ssUDPConn struct {
	udp        *net.UDPConn
	masterKey  []byte
	targetAddr []byte
	remoteAddr *net.UDPAddr
}

func dialShadowsocksUDP(ctx context.Context, cfg proxyClientConfig, remote string) (net.Conn, error) {
	if strings.HasPrefix(cfg.Method, "2022-") {
		return dialSS2022UDP(ctx, cfg, remote)
	}
	keyLen, err := ssKeySize(cfg.Method)
	if err != nil {
		return nil, err
	}
	rHost, rPortStr, err := net.SplitHostPort(remote)
	if err != nil {
		return nil, err
	}
	rPort, err := strconv.Atoi(rPortStr)
	if err != nil {
		return nil, err
	}
	rIP := net.ParseIP(rHost).To4()
	if rIP == nil {
		return nil, errors.New("ePDG target must be IPv4")
	}
	targetAddr, err := encodeSocksTarget(remote)
	if err != nil {
		return nil, err
	}
	srvHost, srvPortStr, err := net.SplitHostPort(cfg.Server)
	if err != nil {
		return nil, err
	}
	srvPort, err := strconv.Atoi(srvPortStr)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP(srvHost).To4(), Port: srvPort})
	if err != nil {
		return nil, err
	}
	_ = ctx
	return &ssUDPConn{
		udp:        udpConn,
		masterKey:  ssDeriveMasterKey(cfg.Password, keyLen),
		targetAddr: targetAddr,
		remoteAddr: &net.UDPAddr{IP: rIP, Port: rPort},
	}, nil
}

func (c *ssUDPConn) Write(b []byte) (int, error) {
	saltLen := len(c.masterKey)
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return 0, err
	}
	aead, err := newSSAEAD(c.masterKey, salt)
	if err != nil {
		return 0, err
	}
	plain := make([]byte, len(c.targetAddr)+len(b))
	copy(plain, c.targetAddr)
	copy(plain[len(c.targetAddr):], b)
	var zeroNonce [12]byte
	pkt := aead.Seal(salt, zeroNonce[:], plain, nil)
	if _, err := c.udp.Write(pkt); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *ssUDPConn) Read(b []byte) (int, error) {
	saltLen := len(c.masterKey)
	buf := make([]byte, max(len(b)+saltLen+280, 2048))
	var zeroNonce [12]byte
	for {
		n, err := c.udp.Read(buf)
		if err != nil {
			return 0, err
		}
		if n < saltLen+16+7 {
			continue
		}
		aead, err := newSSAEAD(c.masterKey, buf[:saltLen])
		if err != nil {
			continue
		}
		plain, err := aead.Open(nil, zeroNonce[:], buf[saltLen:n], nil)
		if err != nil {
			continue
		}
		var offset int
		switch plain[0] {
		case 0x01:
			offset = 7
		case 0x03:
			if len(plain) < 4 {
				continue
			}
			offset = 4 + int(plain[1])
		case 0x04:
			offset = 19
		default:
			continue
		}
		if len(plain) < offset {
			continue
		}
		return copy(b, plain[offset:]), nil
	}
}

func (c *ssUDPConn) Close() error {
	clear(c.masterKey)
	return c.udp.Close()
}

func (c *ssUDPConn) LocalAddr() net.Addr                { return c.udp.LocalAddr() }
func (c *ssUDPConn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *ssUDPConn) SetDeadline(t time.Time) error      { return c.udp.SetDeadline(t) }
func (c *ssUDPConn) SetReadDeadline(t time.Time) error  { return c.udp.SetReadDeadline(t) }
func (c *ssUDPConn) SetWriteDeadline(t time.Time) error { return c.udp.SetWriteDeadline(t) }

type ssTCPConn struct {
	net.Conn
	masterKey []byte
	targetHdr []byte
	txAEAD    cipher.AEAD
	txNonce   [12]byte
	rxAEAD    cipher.AEAD
	rxNonce   [12]byte
	rxBuf     bytes.Buffer
}

func incNonceLE(n *[12]byte) {
	for i := range n {
		n[i]++
		if n[i] != 0 {
			break
		}
	}
}

func dialShadowsocksTCP(ctx context.Context, cfg proxyClientConfig, target string) (net.Conn, error) {
	if strings.HasPrefix(cfg.Method, "2022-") {
		return dialSS2022TCP(ctx, cfg, target)
	}
	keyLen, err := ssKeySize(cfg.Method)
	if err != nil {
		return nil, err
	}
	targetHdr, err := encodeSocksTarget(target)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", cfg.Server)
	if err != nil {
		return nil, err
	}
	return &ssTCPConn{
		Conn:      conn,
		masterKey: ssDeriveMasterKey(cfg.Password, keyLen),
		targetHdr: targetHdr,
	}, nil
}

func (c *ssTCPConn) writeChunk(chunk []byte) error {
	var lenPlain [2]byte
	binary.BigEndian.PutUint16(lenPlain[:], uint16(len(chunk)))
	out := make([]byte, 0, 2+16+len(chunk)+16)
	out = c.txAEAD.Seal(out, c.txNonce[:], lenPlain[:], nil)
	incNonceLE(&c.txNonce)
	out = c.txAEAD.Seal(out, c.txNonce[:], chunk, nil)
	incNonceLE(&c.txNonce)
	_, err := c.Conn.Write(out)
	return err
}

func (c *ssTCPConn) Write(b []byte) (int, error) {
	var prefix []byte
	if c.txAEAD == nil {
		salt := make([]byte, len(c.masterKey))
		if _, err := rand.Read(salt); err != nil {
			return 0, err
		}
		aead, err := newSSAEAD(c.masterKey, salt)
		if err != nil {
			return 0, err
		}
		if _, err := c.Conn.Write(salt); err != nil {
			return 0, err
		}
		c.txAEAD = aead
		prefix = c.targetHdr
		c.targetHdr = nil
	}
	written := 0
	rem := b
	for len(rem) > 0 || len(prefix) > 0 {
		avail := 0x3fff - len(prefix)
		take := min(len(rem), avail)
		var chunk []byte
		if len(prefix) > 0 {
			chunk = make([]byte, len(prefix)+take)
			copy(chunk, prefix)
			copy(chunk[len(prefix):], rem[:take])
			prefix = nil
		} else {
			chunk = rem[:take]
		}
		if err := c.writeChunk(chunk); err != nil {
			return written, err
		}
		written += take
		rem = rem[take:]
	}
	return written, nil
}

func (c *ssTCPConn) Read(b []byte) (int, error) {
	if c.rxBuf.Len() > 0 {
		return c.rxBuf.Read(b)
	}
	if c.rxAEAD == nil {
		salt := make([]byte, len(c.masterKey))
		if _, err := io.ReadFull(c.Conn, salt); err != nil {
			return 0, err
		}
		aead, err := newSSAEAD(c.masterKey, salt)
		if err != nil {
			return 0, err
		}
		c.rxAEAD = aead
	}
	var encLen [2 + 16]byte
	if _, err := io.ReadFull(c.Conn, encLen[:]); err != nil {
		return 0, err
	}
	decLen, err := c.rxAEAD.Open(nil, c.rxNonce[:], encLen[:], nil)
	if err != nil {
		return 0, err
	}
	incNonceLE(&c.rxNonce)
	size := int(binary.BigEndian.Uint16(decLen) & 0x3fff)
	encPayload := make([]byte, size+16)
	if _, err := io.ReadFull(c.Conn, encPayload); err != nil {
		return 0, err
	}
	plain, err := c.rxAEAD.Open(nil, c.rxNonce[:], encPayload, nil)
	if err != nil {
		return 0, err
	}
	incNonceLE(&c.rxNonce)
	n := copy(b, plain)
	if n < len(plain) {
		c.rxBuf.Write(plain[n:])
	}
	return n, nil
}

func (c *ssTCPConn) Close() error {
	clear(c.masterKey)
	return c.Conn.Close()
}
