// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	M "github.com/sagernet/sing/common/metadata"
)

// SS2022 uses a base64 PSK, never the legacy Shadowsocks password KDF.
// Keep the protocol, replay protection and session handling in upstream's implementation.
func newSS2022Method(cfg proxyClientConfig) (shadowsocks.Method, error) {
	keySize := 32
	switch cfg.Method {
	case "2022-blake3-aes-128-gcm":
		keySize = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
	default:
		return nil, errors.New("不支持的 SS2022 加密方式")
	}
	keys := strings.Split(cfg.Password, ":")
	if cfg.Method == "2022-blake3-chacha20-poly1305" && len(keys) != 1 {
		return nil, errors.New("SS2022 ChaCha20 不支持多用户密钥链")
	}
	for _, encoded := range keys {
		key, err := base64.StdEncoding.Strict().DecodeString(encoded)
		valid := err == nil && len(key) == keySize
		clear(key)
		if !valid {
			return nil, errors.New("SS2022 密钥必须是匹配加密方式的 Base64 密钥（AES-128 为 16 字节，其余为 32 字节）")
		}
	}
	method, err := shadowaead_2022.NewWithPassword(cfg.Method, cfg.Password, nil)
	if err != nil {
		return nil, errors.New("SS2022 密钥无效")
	}
	return method, nil
}

func dialSS2022TCP(ctx context.Context, cfg proxyClientConfig, target string) (net.Conn, error) {
	if _, err := encodeSocksTarget(target); err != nil {
		return nil, err
	}
	method, err := newSS2022Method(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "tcp4", cfg.Server)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	wrapped, err := method.DialConn(conn, M.ParseSocksaddr(target))
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return wrapped, nil
}

// The IKE engine requires a connected net.Conn with a *net.UDPAddr LocalAddr.
// Preserve packet boundaries and authenticate/filter the expected ePDG source.
type ss2022UDPConn struct {
	net.PacketConn
	remote  *net.UDPAddr
	readMu  sync.Mutex
	writeMu sync.Mutex
	readBuf [65535]byte
}

func dialSS2022UDP(ctx context.Context, cfg proxyClientConfig, remote string) (net.Conn, error) {
	method, err := newSS2022Method(cfg)
	if err != nil {
		return nil, err
	}
	destination := M.ParseSocksaddr(remote)
	if !destination.Addr.Is4() || destination.Port == 0 {
		return nil, errors.New("ePDG target must be IPv4")
	}
	conn, err := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, "udp4", cfg.Server)
	if err != nil {
		return nil, err
	}
	return &ss2022UDPConn{PacketConn: method.DialPacketConn(conn), remote: destination.UDPAddr()}, nil
}

func (c *ss2022UDPConn) RemoteAddr() net.Addr { return c.remote }

func (c *ss2022UDPConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		n, source, err := c.PacketConn.ReadFrom(c.readBuf[:])
		if err != nil {
			var networkErr net.Error
			if errors.As(err, &networkErr) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return 0, err
			}
			// Discard malformed, unauthenticated or replayed UDP packets.
			continue
		}
		if source == nil || source.String() != c.remote.String() {
			continue
		}
		return copy(p, c.readBuf[:n]), nil
	}
}

func (c *ss2022UDPConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.PacketConn.WriteTo(p, c.remote)
}
