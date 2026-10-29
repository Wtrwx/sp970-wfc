// SPDX-License-Identifier: AGPL-3.0-or-later
// Package nodesync reads ordinary proxy subscriptions and updates a bound SS node.
package nodesync

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	IntervalSeconds  int    `json:"interval_seconds,omitempty"`
	FeedURL          string `json:"feed_url"`
	CredentialSHA256 string `json:"credential_sha256"`
	Base             string `json:"base"`
	CAFile           string `json:"ca_file"`
	Account          string `json:"account,omitempty"`
	Password         string `json:"password,omitempty"`
}

func credentialHash(method, password string) string {
	h := sha256.Sum256([]byte(method + "\x00" + password))
	return hex.EncodeToString(h[:])
}

func CredentialHash(raw string) (string, error) {
	node, err := parseURI(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("请先保存要更新的 Shadowsocks 节点")
	}
	return credentialHash(node.Method, node.Password), nil
}

func replaceAddress(raw string, c Config, node Node) (string, bool, error) {
	current, err := parseURI(strings.TrimSpace(raw))
	if err != nil || credentialHash(current.Method, current.Password) != c.CredentialSHA256 {
		return "", false, errors.New("current node changed; preserving user choice")
	}
	if current.Server == node.Server && current.Port == node.Port {
		return strings.TrimSpace(raw), false, nil
	}
	// Also normalize SIP002 and legacy base64 links when the address changes.
	u := &url.URL{Scheme: "ss", User: url.UserPassword(current.Method, current.Password),
		Host: net.JoinHostPort(node.Server, strconv.Itoa(node.Port)), Fragment: current.Name}
	return u.String(), true, nil
}

func resolver() *net.Resolver {
	return &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		server := ""
		for _, name := range []string{"net.dns1", "net.dns2", "dhcp.wlan0.dns1", "dhcp.wlan0.dns2"} {
			out, e := exec.CommandContext(ctx, "/system/bin/getprop", name).Output()
			if e == nil {
				if ip := net.ParseIP(strings.TrimSpace(string(out))); ip != nil && !ip.IsUnspecified() {
					server = ip.String()
					break
				}
			}
		}
		if server == "" {
			return nil, errors.New("Wi-Fi DNS unavailable")
		}
		return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, net.JoinHostPort(server, "53"))
	}}
}

func validFeedURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || len(raw) > 4096 {
		return nil, errors.New("请填写 HTTP 或 HTTPS 订阅链接")
	}
	u.Fragment = "" // Remove obsolete signature parameters during migration.
	return u, nil
}

func NewClient(c Config) (*http.Client, error) {
	u, err := validFeedURL(c.FeedURL)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: nil,
		DialContext:     (&net.Dialer{Timeout: 8 * time.Second, Resolver: resolver()}).DialContext,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	if u.Scheme == "https" && c.CAFile != "" {
		cert, e := os.ReadFile(c.CAFile)
		if e != nil {
			return nil, errors.New("CA bundle unavailable")
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(cert) {
			return nil, errors.New("invalid CA bundle")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	return &http.Client{Timeout: 18 * time.Second, Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.User != nil {
				return errors.New("invalid subscription redirect")
			}
			if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("HTTPS subscription redirected to HTTP")
			}
			return nil
		}}, nil
}

// Result never serializes node credentials.
type Result struct {
	State       string `json:"state"`
	Message     string `json:"message"`
	IPv4        string `json:"ipv4,omitempty"`
	OriginalURI string `json:"-"`
	URI         string `json:"-"`
}

func Descriptor(c Config) string {
	u, err := validFeedURL(c.FeedURL)
	if err != nil {
		return ""
	}
	return u.String()
}

func ParseDescriptor(raw string, previous Config) (Config, error) {
	u, err := validFeedURL(raw)
	if err != nil {
		return Config{}, err
	}
	previous.FeedURL = u.String()
	return previous, nil
}

func Check(ctx context.Context, c Config, client *http.Client) (Result, error) {
	mode, err := os.ReadFile(filepath.Join(c.Base, "network-mode"))
	if err != nil {
		return Result{}, errors.New("network mode unavailable")
	}
	if strings.TrimSpace(string(mode)) != "ss" {
		return Result{State: "skipped", Message: "skipped: user selected another network mode"}, nil
	}
	raw, err := os.ReadFile(filepath.Join(c.Base, "proxy-uri"))
	if err != nil {
		return Result{}, errors.New("node configuration unavailable")
	}
	defer clear(raw)
	bound, err := CredentialHash(string(raw))
	if err != nil || bound != c.CredentialSHA256 {
		return Result{State: "skipped", Message: "skipped: user selected another node"}, nil
	}
	request, err := http.NewRequestWithContext(ctx, "GET", c.FeedURL, nil)
	if err != nil {
		return Result{}, errors.New("invalid subscription URL")
	}
	request.Header.Set("Accept", "application/json, application/yaml, text/yaml, text/plain")
	response, err := client.Do(request)
	if err != nil {
		return Result{}, errors.New("subscription unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return Result{}, fmt.Errorf("subscription HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxSubscriptionSize+1))
	if err != nil {
		return Result{}, errors.New("subscription read failed")
	}
	defer clear(data)
	nodes, err := ParseSubscription(data)
	if err != nil {
		return Result{}, err
	}
	var match *Node
	for _, node := range nodes {
		if credentialHash(node.Method, node.Password) != c.CredentialSHA256 {
			continue
		}
		if match != nil && (match.Server != node.Server || match.Port != node.Port) {
			return Result{}, errors.New("订阅包含多个凭据相同的节点，无法确定要更新的地址")
		}
		copy := node
		match = &copy
	}
	if match == nil {
		return Result{State: "skipped", Message: "skipped: subscription has no matching node"}, nil
	}
	uri, changed, err := replaceAddress(string(raw), c, *match)
	if err != nil {
		return Result{}, err
	}
	result := Result{State: "unchanged", Message: "unchanged", OriginalURI: strings.TrimSpace(string(raw)), URI: uri}
	if ip := net.ParseIP(match.Server); ip != nil && ip.To4() != nil {
		result.IPv4 = ip.String()
	}
	if changed {
		result.State, result.Message = "available", "update available"
	}
	return result, nil
}
