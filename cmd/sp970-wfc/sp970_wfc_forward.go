// SPDX-License-Identifier: AGPL-3.0-or-later
// Durable forwarding with configurable HTTP requests and masked credentials.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type smsForwardConfig struct {
	Enabled       bool              `json:"enabled"`
	Endpoint      string            `json:"endpoint"`
	Authorization string            `json:"authorization"`
	Since         time.Time         `json:"since"`
	Method        string            `json:"method,omitempty"`
	Format        string            `json:"format,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	SecretHeaders []string          `json:"secret_headers,omitempty"`
	Secrets       map[string]string `json:"secrets,omitempty"`
	Body          string            `json:"body,omitempty"`
	Success       string            `json:"success,omitempty"`
	SuccessPath   string            `json:"success_path,omitempty"`
	SuccessValue  string            `json:"success_value,omitempty"`
	CheckEndpoint string            `json:"check_endpoint,omitempty"`
	Timeout       int               `json:"timeout_seconds,omitempty"`
	ClearAuth     bool              `json:"clear_authorization,omitempty"`
}

type smsForwardJob struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
	State     string    `json:"state"`
	Attempts  int       `json:"attempts"`
	Next      time.Time `json:"next"`
	LastError string    `json:"last_error,omitempty"`
	Delivered time.Time `json:"delivered,omitempty"`
}

type smsForwarder struct {
	mu     sync.Mutex
	dir    string
	config smsForwardConfig
	client *http.Client
	wake   chan struct{}
}

func validateSMSForwardConfig(c smsForwardConfig) error {
	if !c.Enabled && c.Endpoint == "" && c.Authorization == "" {
		return nil
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("转发地址必须是 HTTPS 地址")
	}
	if len(c.Endpoint) > 2048 || len(c.Authorization) > 2048 || strings.ContainsAny(c.Authorization, "\r\n") {
		return errors.New("转发参数无效")
	}
	return validateSMSRequest(c)
}

func newSMSForwarder(dir string) (*smsForwarder, error) {
	if err := os.MkdirAll(filepath.Join(dir, "jobs"), 0700); err != nil {
		return nil, err
	}
	f := &smsForwarder{dir: dir, wake: make(chan struct{}, 1)}
	if raw, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		err = json.Unmarshal(raw, &f.config)
		clear(raw)
		if err != nil || validateSMSForwardConfig(f.config) != nil {
			return nil, errors.New("invalid saved forwarding settings")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	f.config = normalizeSMSRequest(f.config)
	roots, _ := x509.SystemCertPool()
	if roots == nil {
		roots = x509.NewCertPool()
	}
	if pem, err := os.ReadFile("/data/adb/sp970-wfc/ca-roots.pem"); err == nil {
		roots.AppendCertsFromPEM(pem)
		clear(pem)
	}
	resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, "1.1.1.1:53")
	}}
	f.client = &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 8 * time.Second, Resolver: resolver}).DialContext(ctx, "tcp4", address)
			},
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
			TLSHandshakeTimeout: 5 * time.Second,
			IdleConnTimeout:     30 * time.Second,
		},
		// A redirect must never carry the existing ZTE token elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return f, nil
}

func (f *smsForwarder) check(ctx context.Context) error {
	f.mu.Lock()
	c := f.config
	f.mu.Unlock()
	c = normalizeSMSRequest(c)
	endpoint := c.CheckEndpoint
	if endpoint == "" {
		u, err := url.Parse(c.Endpoint)
		if err != nil || !strings.HasSuffix(u.Path, "/v1/sms") || c.Success != "accepted" {
			return errors.New("请先设置同站点的只读检查地址；不会向通知地址发送测试请求")
		}
		u.Path = strings.TrimSuffix(u.Path, "/sms") + "/status"
		u.RawQuery = ""
		endpoint = u.String()
	}
	endpoint, err := renderSMSTemplate(endpoint, c, smsForwardJob{}, "url")
	if err != nil {
		return errors.New("检查地址模板无效")
	}
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, "GET", endpoint, nil)
	if err != nil {
		return errors.New("转发地址无效")
	}
	if err := applySMSHeaders(req, c, smsForwardJob{}); err != nil {
		return err
	}
	response, err := f.client.Do(req)
	if err != nil {
		return errors.New("无法连接转发服务")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 || (c.CheckEndpoint == "" && response.StatusCode != 200) {
		return fmt.Errorf("转发服务返回 HTTP %d", response.StatusCode)
	}
	if c.CheckEndpoint != "" {
		return nil
	}
	var status struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&status) != nil || status.Status != "ok" {
		return errors.New("转发服务未确认连接")
	}
	return nil
}

func smsForwardID(from, text string, at time.Time) string {
	raw, _ := json.Marshal([]string{from, text, at.UTC().Format(time.RFC3339Nano)})
	id := sha256.Sum256(raw)
	clear(raw)
	return hex.EncodeToString(id[:])
}

func (f *smsForwarder) enqueue(from, text string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Enabling forwarding never exports the existing inbox or old network retries.
	if !f.config.Enabled || at.IsZero() || at.Before(f.config.Since) {
		return nil
	}
	id := smsForwardID(from, text, at)
	path := filepath.Join(f.dir, "jobs", id+".json")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	job := smsForwardJob{ID: id, From: from, Text: text, At: at, State: "pending", Next: time.Now()}
	if err := privateJSON(path, job); err != nil {
		return err
	}
	select {
	case f.wake <- struct{}{}:
	default:
	}
	return nil
}

func smsForwardForm(j smsForwardJob) url.Values {
	// UFI-TOOLS JSON-escapes the SMS body before substituting {{sms-body}}.
	escaped, _ := json.Marshal(j.Text)
	defer clear(escaped)
	return url.Values{
		"from": {j.From}, "time": {j.At.UTC().Format(time.RFC3339Nano)},
		"body": {string(escaped[1 : len(escaped)-1])}, "ufi_escaped": {"1"},
	}
}

func (f *smsForwarder) deliver(ctx context.Context, c smsForwardConfig, j smsForwardJob) string {
	c = normalizeSMSRequest(c)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.Timeout)*time.Second)
	defer cancel()
	req, err := smsForwardRequest(ctx, c, j)
	if err != nil {
		return "转发请求参数无效"
	}
	response, err := f.client.Do(req)
	if err != nil {
		return "无法连接转发服务，将自动重试"
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	defer clear(raw)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Sprintf("转发服务返回 HTTP %d", response.StatusCode)
	}
	if err != nil || len(raw) > 65536 || !smsForwardAccepted(c, raw) {
		return "转发服务未确认接收"
	}
	return ""
}

func (f *smsForwarder) deliverDue(ctx context.Context) {
	f.mu.Lock()
	c := f.config
	f.mu.Unlock()
	if !c.Enabled {
		return
	}
	files, err := filepath.Glob(filepath.Join(f.dir, "jobs", "*.json"))
	if err != nil {
		return
	}
	for _, path := range files {
		if ctx.Err() != nil {
			return
		}
		f.mu.Lock()
		var j smsForwardJob
		raw, err := os.ReadFile(path)
		if err == nil {
			err = json.Unmarshal(raw, &j)
		}
		clear(raw)
		f.mu.Unlock()
		if err != nil || j.State != "pending" || time.Now().Before(j.Next) {
			continue
		}
		f.mu.Lock()
		c = f.config
		f.mu.Unlock()
		if !c.Enabled {
			return
		}
		failure := f.deliver(ctx, c, j)
		if ctx.Err() != nil {
			return
		}
		j.Attempts++
		j.LastError = failure
		if failure == "" {
			j.State, j.Delivered = "delivered", time.Now()
		} else {
			backoff := 30 * time.Second * time.Duration(1<<min(j.Attempts-1, 7))
			j.Next = time.Now().Add(backoff)
		}
		f.mu.Lock()
		err = privateJSON(path, j)
		f.mu.Unlock()
		if err != nil {
			event("sms_forward_storage_error", nil)
			continue
		}
		event("sms_forward_attempt", map[string]any{"accepted": failure == "", "attempts": j.Attempts})
	}
}

func (f *smsForwarder) run(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	defer f.client.CloseIdleConnections()
	for {
		f.deliverDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-f.wake:
		}
	}
}

func maskedSMSAuthorization(value string) string {
	if value == "" {
		return ""
	}
	if scheme, _, ok := strings.Cut(value, " "); ok && (strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "Basic") || strings.EqualFold(scheme, "Token")) {
		return scheme + " ****************"
	}
	return "****************"
}

func maskedSMSForwardEndpoint(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.RawQuery == "" {
		return value
	}
	q := u.Query()
	for key, values := range q {
		for i := range values {
			if match := smsVariable.FindString(values[i]); match == values[i] && match != "" {
				continue
			}
			values[i] = "********"
		}
		q[key] = values
	}
	u.RawQuery = strings.ReplaceAll(q.Encode(), "%2A", "*")
	return u.String()
}

func (f *smsForwarder) view() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := normalizeSMSRequest(f.config)
	view := map[string]any{
		"enabled": c.Enabled, "endpoint": maskedSMSForwardEndpoint(c.Endpoint),
		"authorization_masked":     maskedSMSAuthorization(c.Authorization),
		"authorization_configured": c.Authorization != "", "method": c.Method,
		"format": c.Format, "body": c.Body, "headers": maskedSMSHeaders(c),
		"secret_headers": c.SecretHeaders, "secrets": maskedSMSSecrets(c.Secrets),
		"success": c.Success, "success_path": c.SuccessPath, "success_value": c.SuccessValue,
		"check_endpoint": maskedSMSForwardEndpoint(c.CheckEndpoint), "timeout_seconds": c.Timeout,
		"since": f.config.Since, "pending": 0, "delivered": 0, "last_error": "",
	}
	files, _ := filepath.Glob(filepath.Join(f.dir, "jobs", "*.json"))
	var latest time.Time
	for _, path := range files {
		raw, err := os.ReadFile(path)
		var j smsForwardJob
		if err == nil {
			err = json.Unmarshal(raw, &j)
		}
		clear(raw)
		if err != nil {
			continue
		}
		if j.State == "pending" {
			view["pending"] = view["pending"].(int) + 1
		}
		if j.State == "delivered" {
			view["delivered"] = view["delivered"].(int) + 1
		}
		if j.At.After(latest) {
			latest = j.At
			view["last_error"] = j.LastError
		}
	}
	return view
}

func (f *smsForwarder) save(c smsForwardConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c = preserveSMSSecrets(c, f.config)
	if err := validateSMSForwardConfig(c); err != nil {
		return err
	}
	c.Since = f.config.Since
	if c.Enabled && (!f.config.Enabled || c.Since.IsZero()) {
		c.Since = time.Now()
	}
	if err := privateJSON(filepath.Join(f.dir, "config.json"), c); err != nil {
		return errors.New("保存转发设置失败")
	}
	f.config = c
	select {
	case f.wake <- struct{}{}:
	default:
	}
	return nil
}
