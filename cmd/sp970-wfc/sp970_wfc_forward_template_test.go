// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSMSCustomJSONAndResponseCondition(t *testing.T) {
	text := "中文 & + \"quoted\"\n{{literal}} \\tail"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.Header.Get("X-Api-Key") != "private-local-only" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("custom request not applied")
		}
		var body struct {
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Message.Text != text {
			t.Error("JSON template corrupted SMS content")
		}
		_, _ = io.WriteString(w, `{"data":{"code":0}}`)
	}))
	defer server.Close()
	c := smsForwardConfig{Endpoint: server.URL, Method: "PUT", Format: "json", Body: `{"message":{"text":"{{sms-body}}"}}`, Headers: map[string]string{"X-Api-Key": "{{secret:key}}"}, Secrets: map[string]string{"key": "private-local-only"}, Success: "json", SuccessPath: "data.code", SuccessValue: "0"}
	if err := validateSMSForwardConfig(c); err != nil {
		t.Fatal(err)
	}
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.client = server.Client()
	if failure := f.deliver(context.Background(), c, smsForwardJob{ID: "local-id", From: "+441234567890", Text: text, At: time.Now()}); failure != "" {
		t.Fatal(failure)
	}
	if smsForwardAccepted(c, []byte(`{"data":{"code":1}}`)) {
		t.Fatal("JSON mismatch counted as accepted")
	}
	c.SuccessValue = "null"
	if smsForwardAccepted(c, []byte(`{"data":{}}`)) {
		t.Fatal("missing field matched null")
	}
}

func TestSMSURLAndFormTemplateEncoding(t *testing.T) {
	text := "A & + / ? 中文\nsecond"
	c := smsForwardConfig{Endpoint: "https://example.com/{{secret:path}}?from={{sms-from}}&text={{sms-body}}", Method: "GET", Format: "none", Success: "http", Secrets: map[string]string{"path": "a/b"}}
	c = preserveSMSSecrets(c, smsForwardConfig{})
	req, err := smsForwardRequest(context.Background(), c, smsForwardJob{From: "+44123", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Query().Get("text") != text || req.URL.Query().Get("from") != "+44123" || req.URL.EscapedPath() != "/a%2Fb" {
		t.Fatal("URL template was double-encoded or split")
	}
	c.Method = "PATCH"
	c.Format = "form"
	c.Body = "message={{sms-body}}\nname=raw + & percent%"
	c.Endpoint = "https://example.com"
	req, err = smsForwardRequest(context.Background(), c, smsForwardJob{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if err := req.ParseForm(); err != nil || req.PostForm.Get("message") != text || req.PostForm.Get("name") != "raw + & percent%" {
		t.Fatal("form template was double-encoded")
	}
}

func TestSMSCustomMaskedSaveAndPreviewNeverDispatches(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.client = server.Client()
	c := smsForwardConfig{Endpoint: server.URL + "/push?key=query-private", Method: "POST", Format: "json", Body: `{"text":"{{sms-body}}","token":"{{secret:token}}"}`, Headers: map[string]string{"X-Private": "header-private"}, SecretHeaders: []string{"X-Private"}, Secrets: map[string]string{"token": "variable-private"}, Success: "http"}
	if err := f.save(c); err != nil {
		t.Fatal(err)
	}
	view := f.view()
	encoded, _ := json.Marshal(view)
	for _, secret := range []string{"query-private", "header-private", "variable-private"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("settings exposed saved secret")
		}
	}
	var displayed smsForwardConfig
	if err := json.Unmarshal(encoded, &displayed); err != nil {
		t.Fatal(err)
	}
	displayed.Endpoint = server.URL + "/new?key=********"
	if err := f.save(displayed); err != nil {
		t.Fatal(err)
	}
	if f.config.Secrets["token"] != "variable-private" || f.config.Headers["X-Private"] != "header-private" || !strings.Contains(f.config.Endpoint, "query-private") {
		t.Fatal("edited masked settings erased secrets")
	}
	preview, err := f.preview(displayed)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(preview)
	for _, secret := range []string{"query-private", "header-private", "variable-private"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("preview exposed saved secret")
		}
	}
	if calls.Load() != 0 || preview["network_request_sent"] != false {
		t.Fatal("preview submitted a notification")
	}
}

func TestSMSCustomRequestValidationAndExplicitAuthClear(t *testing.T) {
	base := smsForwardConfig{Endpoint: "https://example.com/v1/sms", Method: "POST", Format: "text", Body: "{{sms-body}}", Success: "http"}
	bad := []smsForwardConfig{base, base, base, base, base}
	bad[0].Headers = map[string]string{"X-Test": "line\r\ninjected"}
	bad[1].Body = "{{unknown}}"
	bad[2].Method = "GET"
	bad[3].CheckEndpoint = "https://other.example.com/status"
	bad[4].Timeout = 300
	for i, c := range bad {
		if validateSMSForwardConfig(c) == nil {
			t.Fatalf("invalid request %d accepted", i)
		}
	}
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base.Authorization = "Bearer old-test-only"
	if err := f.save(base); err != nil {
		t.Fatal(err)
	}
	base.Authorization = ""
	base.ClearAuth = true
	if err := f.save(base); err != nil {
		t.Fatal(err)
	}
	if f.config.Authorization != "" || f.config.ClearAuth {
		t.Fatal("explicit auth clear did not take effect")
	}
}
