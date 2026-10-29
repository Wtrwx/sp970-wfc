// SPDX-License-Identifier: AGPL-3.0-or-later
package nodesync

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxSubscriptionSize = 2 << 20

type Node struct {
	Name, Server, Method, Password string
	Port                           int
}

func supported(node Node) bool {
	if node.Server == "" || strings.ContainsAny(node.Server, " /?#@\r\n\t") || node.Password == "" || node.Port < 1 || node.Port > 65535 {
		return false
	}
	switch node.Method {
	case "aes-128-gcm", "aes-256-gcm", "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return true
	}
	return false
}

func decodeBase64(raw string) ([]byte, error) {
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if out, err := encoding.DecodeString(raw); err == nil {
			return out, nil
		}
	}
	return nil, errors.New("invalid base64 subscription")
}

func parseURI(raw string) (Node, error) {
	invalid := errors.New("invalid SS subscription node")
	var node Node
	if !strings.HasPrefix(raw, "ss://") {
		return node, invalid
	}
	body := strings.TrimPrefix(raw, "ss://")
	if head, name, ok := strings.Cut(body, "#"); ok {
		body = head
		node.Name, _ = url.PathUnescape(name)
	}
	if head, query, ok := strings.Cut(body, "?"); ok {
		if query != "" {
			return node, invalid
		}
		body = head
	}
	body = strings.TrimSuffix(body, "/")
	if !strings.Contains(body, "@") {
		decoded, err := decodeBase64(body)
		if err != nil {
			return node, invalid
		}
		body = string(decoded)
		clear(decoded)
	}
	at := strings.LastIndexByte(body, '@')
	if at < 1 {
		return node, invalid
	}
	credentials, address := body[:at], body[at+1:]
	if !strings.Contains(credentials, ":") {
		decoded, err := decodeBase64(credentials)
		if err != nil {
			return node, invalid
		}
		credentials = string(decoded)
		clear(decoded)
	} else {
		var err error
		credentials, err = url.PathUnescape(credentials)
		if err != nil {
			return node, invalid
		}
	}
	var ok bool
	node.Method, node.Password, ok = strings.Cut(credentials, ":")
	if !ok {
		return node, invalid
	}
	node.Method = strings.ToLower(node.Method)
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return node, invalid
	}
	node.Server = host
	node.Port, err = strconv.Atoi(port)
	if err != nil || !supported(node) {
		return node, invalid
	}
	return node, nil
}

func ParseSubscription(raw []byte) ([]Node, error) {
	invalid := errors.New("订阅格式无效，支持 Mihomo YAML、sing-box JSON 和 SS 链接订阅")
	if len(raw) == 0 || len(raw) > maxSubscriptionSize {
		return nil, invalid
	}
	text := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	var document struct {
		Proxies []struct {
			Type     string `json:"type" yaml:"type"`
			Name     string `json:"name" yaml:"name"`
			Server   string `json:"server" yaml:"server"`
			Port     int    `json:"port" yaml:"port"`
			Cipher   string `json:"cipher" yaml:"cipher"`
			Password string `json:"password" yaml:"password"`
			Plugin   string `json:"plugin" yaml:"plugin"`
			UDP      *bool  `json:"udp" yaml:"udp"`
		} `json:"proxies" yaml:"proxies"`
		Outbounds []struct {
			Type       string `json:"type" yaml:"type"`
			Tag        string `json:"tag" yaml:"tag"`
			Server     string `json:"server" yaml:"server"`
			ServerPort int    `json:"server_port" yaml:"server_port"`
			Method     string `json:"method" yaml:"method"`
			Password   string `json:"password" yaml:"password"`
			Plugin     string `json:"plugin" yaml:"plugin"`
			Network    string `json:"network" yaml:"network"`
		} `json:"outbounds" yaml:"outbounds"`
	}
	var nodes []Node
	if strings.HasPrefix(text, "{") {
		if json.Unmarshal([]byte(text), &document) != nil {
			return nil, invalid
		}
	} else if strings.HasPrefix(text, "ss://") {
		// Plain SS URI list.
	} else if decoded, err := decodeBase64(strings.Join(strings.Fields(text), "")); err == nil && strings.HasPrefix(strings.TrimSpace(string(decoded)), "ss://") {
		text = strings.TrimSpace(string(decoded))
		clear(decoded)
	} else if yaml.Unmarshal([]byte(text), &document) != nil {
		return nil, invalid
	}
	if strings.HasPrefix(text, "ss://") {
		for _, line := range strings.Split(text, "\n") {
			if node, err := parseURI(strings.TrimSpace(line)); err == nil {
				nodes = append(nodes, node)
			}
		}
	}
	for _, entry := range document.Proxies {
		node := Node{Name: entry.Name, Server: entry.Server, Port: entry.Port, Method: entry.Cipher, Password: entry.Password}
		if entry.Type == "ss" && entry.Plugin == "" && (entry.UDP == nil || *entry.UDP) && supported(node) {
			nodes = append(nodes, node)
		}
	}
	for _, entry := range document.Outbounds {
		node := Node{Name: entry.Tag, Server: entry.Server, Port: entry.ServerPort, Method: entry.Method, Password: entry.Password}
		if entry.Type == "shadowsocks" && entry.Plugin == "" && entry.Network != "tcp" && supported(node) {
			nodes = append(nodes, node)
		}
	}
	if len(nodes) == 0 {
		return nil, errors.New("订阅中没有支持 UDP 的 Shadowsocks 节点")
	}
	return nodes, nil
}
