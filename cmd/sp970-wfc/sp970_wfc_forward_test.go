// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSMSForwardEncodingDurabilityAndRetry(t *testing.T) {
	var calls atomic.Int32
	text := "验证码 & + % 中文\n\"quoted\" \\tail"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/v1/status" {
			if r.Header.Get("Authorization") != "Bearer test-only" {
				t.Error("status check did not verify configured credentials")
			}
			_, _ = io.WriteString(w, `{"status":"ok"}`)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-only" || r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing authorization or stable idempotency key")
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("from") != "+12345678901" || r.Form.Get("ufi_escaped") != "1" {
			t.Error("ZTE fields changed")
		}
		var decoded string
		if err := json.Unmarshal([]byte(`"`+r.Form.Get("body")+`"`), &decoded); err != nil || decoded != text {
			t.Error("SMS text was corrupted by escaping", err)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_, _ = io.WriteString(w, `{"accepted":true,"duplicate":false}`)
	}))
	defer server.Close()
	dir := t.TempDir()
	f, err := newSMSForwarder(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.client = server.Client()
	if err := f.save(smsForwardConfig{Enabled: true, Endpoint: server.URL + "/v1/sms", Authorization: "Bearer test-only"}); err != nil {
		t.Fatal(err)
	}
	if err := f.check(context.Background()); err != nil || calls.Load() != 0 {
		t.Fatal("connection check submitted a notification", err)
	}
	now := time.Now()
	if err := f.enqueue("+12345678901", "old inbox message", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if f.view()["pending"] != 0 {
		t.Fatal("enabling forwarded historical inbox")
	}
	for i := 0; i < 2; i++ {
		if err := f.enqueue("+12345678901", text, now); err != nil {
			t.Fatal(err)
		}
	}
	if f.view()["pending"] != 1 {
		t.Fatal("network duplicate created another forwarding job")
	}
	f.deliverDue(context.Background())
	if f.view()["pending"] != 1 || calls.Load() != 1 {
		t.Fatal("temporary failure did not retain queue")
	}
	// Restart the runtime and let the durable pending job become due.
	f, err = newSMSForwarder(dir)
	if err != nil {
		t.Fatal(err)
	}
	f.client = server.Client()
	path := filepath.Join(dir, "jobs", smsForwardID("+12345678901", text, now)+".json")
	raw, _ := os.ReadFile(path)
	var job smsForwardJob
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	job.Next = time.Now().Add(-time.Second)
	if err := privateJSON(path, job); err != nil {
		t.Fatal(err)
	}
	f.deliverDue(context.Background())
	if f.view()["delivered"] != 1 || f.view()["pending"] != 0 || calls.Load() != 2 {
		t.Fatal("durable retry did not complete")
	}
	if err := f.enqueue("+12345678901", text, now); err != nil {
		t.Fatal(err)
	}
	f.deliverDue(context.Background())
	if calls.Load() != 2 {
		t.Fatal("completed message was pushed again")
	}
	view, _ := json.Marshal(f.view())
	if strings.Contains(string(view), text) {
		t.Fatal("settings exposed SMS content")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("SMS queue is not private")
	}
	if err := f.save(smsForwardConfig{Enabled: false, Endpoint: server.URL}); err != nil {
		t.Fatal(err)
	}
	if f.config.Authorization != "Bearer test-only" {
		t.Fatal("blank UI credential erased existing token")
	}
}

func TestSMSForwardRejectsRedirectsAndFalseSuccess(t *testing.T) {
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var stolen atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { stolen.Add(1) }))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL, 307)
			return
		}
		_, _ = io.WriteString(w, "<html>login required</html>")
	}))
	defer source.Close()
	// Reuse the forwarding redirect policy while trusting only the test CA.
	f.client.Transport = source.Client().Transport
	j := smsForwardJob{ID: "test", Text: "local test", At: time.Now()}
	for _, path := range []string{"/redirect", "/html"} {
		if f.deliver(context.Background(), smsForwardConfig{Endpoint: source.URL + path, Authorization: "Bearer local-test"}, j) == "" {
			t.Fatal("unconfirmed destination counted as success")
		}
	}
	if stolen.Load() != 0 {
		t.Fatal("forwarding leaked credentials to a redirected destination")
	}
	for _, endpoint := range []string{"http://example.com", "https://user:password@example.com", "file:///etc/passwd"} {
		if validateSMSForwardConfig(smsForwardConfig{Endpoint: endpoint}) == nil {
			t.Fatal("accepted unsafe endpoint", endpoint)
		}
	}
}

func TestSMSForwardMaskedSettingsPreserveRealCredentials(t *testing.T) {
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := smsForwardConfig{Enabled: true, Endpoint: "https://example.com/v1/sms?access_token=private-example&route=private-route", Authorization: "Bearer private-example"}
	if err := f.save(original); err != nil {
		t.Fatal(err)
	}
	view := f.view()
	maskedAuth := view["authorization_masked"].(string)
	maskedEndpoint := view["endpoint"].(string)
	if maskedAuth != "Bearer ****************" || strings.Contains(maskedEndpoint, "private-") || !strings.Contains(maskedEndpoint, "********") {
		t.Fatal("settings did not mask stored secrets")
	}
	if err := f.save(smsForwardConfig{Enabled: true, Endpoint: maskedEndpoint, Authorization: maskedAuth}); err != nil {
		t.Fatal(err)
	}
	if f.config.Authorization != original.Authorization || f.config.Endpoint != original.Endpoint {
		t.Fatal("saving displayed stars overwrote real credentials")
	}
}
