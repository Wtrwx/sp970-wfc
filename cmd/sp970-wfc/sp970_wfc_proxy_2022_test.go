// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-shadowsocks/shadowaead_2022"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func ss2022TestKey(method string) string {
	length := 32
	if method == "2022-blake3-aes-128-gcm" {
		length = 16
	}
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, length))
}

func TestSS2022URIImport(t *testing.T) {
	for _, method := range shadowaead_2022.List {
		key := ss2022TestKey(method)
		for _, credential := range []string{
			method + ":" + url.PathEscape(key),
			base64.RawURLEncoding.EncodeToString([]byte(method + ":" + key)),
		} {
			node, err := parseAnyProxyURI(context.Background(), "ss://"+credential+"@192.0.2.8:443#UK")
			if err != nil {
				t.Fatal(err)
			}
			if node.Protocol != "ss" || node.ClientConfig.Method != method || node.ClientConfig.Password != key || node.ClientConfig.Server != "192.0.2.8:443" {
				t.Fatal("SS2022 import lost protocol, PSK or endpoint")
			}
		}
	}
	for _, credential := range []string{
		"2022-blake3-aes-256-gcm:not-a-key",
		"2022-blake3-aes-256-gcm:" + ss2022TestKey("2022-blake3-aes-128-gcm"),
		"2022-blake3-aes-128-gcm:" + ss2022TestKey("2022-blake3-aes-256-gcm"),
		"2022-blake3-aes-256-gcm:%GG",
		"2022-unknown:" + ss2022TestKey("2022-blake3-aes-256-gcm"),
	} {
		_, err := parseAnyProxyURI(context.Background(), "ss://"+credential+"@192.0.2.8:443")
		if err == nil {
			t.Fatal("invalid SS2022 credential accepted")
		}
		if strings.Contains(err.Error(), "not-a-key") || strings.Contains(err.Error(), "%GG") {
			t.Fatal("parse error exposed credential")
		}
	}
}

type ss2022EchoHandler struct{ errors chan error }

func ss2022ReplyPacket(payload []byte) *buf.Buffer {
	packet := buf.NewSize(2048 + len(payload))
	packet.Resize(1024, 0)
	_, _ = packet.Write(payload)
	return packet
}

func (h *ss2022EchoHandler) NewError(_ context.Context, err error) {
	h.errors <- err
}

func (h *ss2022EchoHandler) NewConnection(_ context.Context, conn net.Conn, metadata M.Metadata) error {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if metadata.Destination.String() != "unit.example:443" {
		return errors.New("TCP destination changed")
	}
	data := make([]byte, 5)
	if _, err := io.ReadFull(conn, data); err != nil {
		return err
	}
	_, err := conn.Write(append([]byte("tcp:"), data...))
	return err
}

func (h *ss2022EchoHandler) NewPacketConnection(_ context.Context, conn N.PacketConn, _ M.Metadata) error {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 2; i++ {
		packet := buf.NewPacket()
		destination, err := conn.ReadPacket(packet)
		if err != nil {
			packet.Release()
			return err
		}
		if destination.String() != "192.0.2.20:4500" {
			packet.Release()
			return errors.New("UDP ePDG destination changed")
		}
		response := append([]byte("udp:"), packet.Bytes()...)
		packet.Release()
		// An authenticated packet for another destination must also be dropped.
		if err := conn.WritePacket(ss2022ReplyPacket([]byte("wrong source")), M.ParseSocksaddr("192.0.2.21:4500")); err != nil {
			return err
		}
		if err := conn.WritePacket(ss2022ReplyPacket(response), destination); err != nil {
			return err
		}
	}
	return nil
}

type ss2022ReplayWriter struct{ net.PacketConn }

func (c *ss2022ReplayWriter) WriteTo(p []byte, address net.Addr) (int, error) {
	// Exercise corrupt-packet and ciphertext replay rejection on the client.
	_, _ = c.PacketConn.WriteTo([]byte{1, 2, 3}, address)
	n, err := c.PacketConn.WriteTo(p, address)
	if err == nil {
		_, err = c.PacketConn.WriteTo(p, address)
	}
	return n, err
}

func TestSS2022ReferenceServerInteroperability(t *testing.T) {
	for _, method := range shadowaead_2022.List {
		t.Run(method, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			handler := &ss2022EchoHandler{errors: make(chan error, 8)}
			server, err := shadowaead_2022.NewServiceWithPassword(method, ss2022TestKey(method), 10, handler, nil)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				if err := server.NewConnection(ctx, conn, M.Metadata{}); err != nil {
					conn.Close()
					handler.errors <- err
				}
			}()
			cfg := proxyClientConfig{Protocol: "ss", Server: listener.Addr().String(), Method: method, Password: ss2022TestKey(method)}
			conn, err := dialShadowsocksTCP(ctx, cfg, "unit.example:443")
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			_, err = conn.Write([]byte("hello"))
			if err != nil {
				t.Fatal(err)
			}
			reply := make([]byte, 9)
			_, err = io.ReadFull(conn, reply)
			conn.Close()
			if err != nil || string(reply) != "tcp:hello" {
				t.Fatalf("TCP interoperability failed: %v", err)
			}

			udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer udp.Close()
			writer := bufio.NewPacketConn(&ss2022ReplayWriter{PacketConn: udp})
			go func() {
				for {
					packet := buf.NewPacket()
					n, source, err := udp.ReadFrom(packet.FreeBytes())
					if err != nil {
						packet.Release()
						return
					}
					packet.Truncate(n)
					if err := server.NewPacket(ctx, writer, packet, M.Metadata{Source: M.SocksaddrFromNet(source)}); err != nil {
						packet.Release()
						handler.errors <- err
					}
				}
			}()
			cfg.Server = udp.LocalAddr().String()
			cfg.WFCOnly = true
			packetConn, err := dialRuntimeUDP(ctx, &cfg, "ss", "192.0.2.20:4500")
			if err != nil {
				t.Fatal(err)
			}
			defer packetConn.Close()
			if _, ok := packetConn.LocalAddr().(*net.UDPAddr); !ok {
				t.Fatal("IKE requires *net.UDPAddr LocalAddr")
			}
			_ = packetConn.SetDeadline(time.Now().Add(3 * time.Second))
			for i := 0; i < 2; i++ {
				payload := fmt.Sprintf("packet-%d", i)
				if _, err := packetConn.Write([]byte(payload)); err != nil {
					t.Fatal(err)
				}
				var reply [128]byte
				n, err := packetConn.Read(reply[:])
				if err != nil || string(reply[:n]) != "udp:"+payload {
					t.Fatalf("UDP interoperability/replay filtering failed: %v", err)
				}
			}
			_ = packetConn.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
			if _, err := packetConn.Read(make([]byte, 64)); err == nil {
				t.Fatal("replayed UDP response was accepted")
			}
			select {
			case err := <-handler.errors:
				t.Fatal(err)
			default:
			}
		})
	}
}

func TestWFCOnlyLeavesApplicationTrafficDirect(t *testing.T) {
	for _, mode := range []string{"ss", "socks5"} {
		t.Run(mode, func(t *testing.T) {
			// This proxy cannot carry traffic: both application connections must
			// reach the local destination without contacting it.
			cfg := &proxyClientConfig{Protocol: mode, Server: "127.0.0.1:1", WFCOnly: true}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			tcp, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer tcp.Close()
			go func() {
				conn, err := tcp.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_, _ = io.WriteString(conn, "direct TCP")
			}()
			conn, err := dialRuntimeTCP(ctx, cfg, mode, nil, tcp.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			reply, err := io.ReadAll(conn)
			conn.Close()
			if err != nil || string(reply) != "direct TCP" {
				t.Fatalf("application TCP did not use the local network: %v", err)
			}

			udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer udp.Close()
			go func() {
				var packet [64]byte
				n, from, err := udp.ReadFrom(packet[:])
				if err == nil {
					_, _ = udp.WriteTo(packet[:n], from)
				}
			}()
			conn, err = dialRuntimeUDP(ctx, cfg, mode, udp.LocalAddr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			if _, err = conn.Write([]byte("direct UDP")); err != nil {
				t.Fatal(err)
			}
			var packet [64]byte
			n, err := conn.Read(packet[:])
			if err != nil || string(packet[:n]) != "direct UDP" {
				t.Fatalf("non-IPsec UDP did not use the local network: %v", err)
			}
		})
	}
}
