// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSMSCurlRoundTripShowsAuthorizationAndPreservesOtherSecrets(t *testing.T) {
	if smsForwardCurl(smsForwardConfig{}) != "" {
		t.Fatal("empty settings should not display an example command")
	}
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original := smsForwardConfig{Enabled: true, Endpoint: "https://example.com/v1/sms?token=query-private", Authorization: "Bearer private-existing"}
	if err := f.save(original); err != nil {
		t.Fatal(err)
	}
	since := f.config.Since
	source := f.view()["curl"].(string)
	if strings.Contains(source, "query-private") || !strings.Contains(source, "Authorization: Bearer private-existing") {
		t.Fatal("cURL did not show Authorization or mask the query secret")
	}
	preview, err := f.preview(smsForwardConfig{Enabled: true, Curl: source})
	if err != nil {
		t.Fatal(err)
	}
	if preview["headers"].(map[string]string)["Authorization"] != original.Authorization || preview["network_request_sent"] != false {
		t.Fatal("preview changed Authorization or sent a network request")
	}
	if err := f.save(smsForwardConfig{Enabled: true, Curl: source}); err != nil {
		t.Fatal(err)
	}
	if f.config.Authorization != original.Authorization || f.config.Endpoint != original.Endpoint || f.config.Success != "accepted" || !f.config.Since.Equal(since) {
		t.Fatal("masked cURL changed existing credentials or activation boundary")
	}
	job := smsForwardJob{From: "+441234567890", Text: "中文 + & %\n\"quoted\"", At: time.Now()}
	req, err := smsForwardRequest(context.Background(), f.config, job)
	if err != nil {
		t.Fatal(err)
	}
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	for key, want := range smsForwardForm(job) {
		if req.PostForm.Get(key) != want[0] {
			t.Fatal("legacy form changed through cURL import", key)
		}
	}
}

func TestSMSCurlJSONImportUsesHeadersAndExplicitAuthRemoval(t *testing.T) {
	text := "Bob's \"message\" & 中文\nsecond"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "local-private" {
			t.Error("cURL request options not applied")
		}
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["message"] != text {
			t.Error("cURL JSON encoding changed message")
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	f, err := newSMSForwarder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.client = server.Client()
	if err := f.save(smsForwardConfig{Endpoint: server.URL, Authorization: "Bearer should-be-removed"}); err != nil {
		t.Fatal(err)
	}
	source := "curl --request=PATCH " + smsCurlQuote(server.URL) + " \\\n --header 'X-Api-Key: local-private' \\\n --json '{\"message\":\"{{sms-body}}\"}' --max-time 30"
	if err := f.save(smsForwardConfig{Curl: source, Success: "http"}); err != nil {
		t.Fatal(err)
	}
	if f.config.Authorization != "" || f.config.Timeout != 30 {
		t.Fatal("omitted auth was retained or timeout ignored")
	}
	if failure := f.deliver(context.Background(), f.config, smsForwardJob{Text: text}); failure != "" {
		t.Fatal(failure)
	}
	if strings.Contains(f.view()["curl"].(string), "local-private") {
		t.Fatal("sensitive header exposed in cURL")
	}
}

func TestSMSCurlGetAndMultilineForm(t *testing.T) {
	c, err := prepareSMSForwardConfig(smsForwardConfig{Curl: "curl -G 'https://example.com/push' --data-urlencode 'text={{sms-body}}' --data-urlencode 'from={{sms-from}}'"}, smsForwardConfig{})
	if err != nil {
		t.Fatal(err)
	}
	text := "+ & 中文\n{{literal}}"
	req, err := smsForwardRequest(context.Background(), c, smsForwardJob{From: "+44123", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" || req.URL.Query().Get("text") != text || req.URL.Query().Get("from") != "+44123" {
		t.Fatal("GET fields were not encoded correctly")
	}
	exported := smsForwardCurl(c)
	if !strings.Contains(exported, "{{sms-body}}") {
		t.Fatal("export encoded template braces")
	}
	if _, err := prepareSMSForwardConfig(smsForwardConfig{Curl: exported}, c); err != nil {
		t.Fatal(err)
	}
	c, err = prepareSMSForwardConfig(smsForwardConfig{Curl: "curl 'https://example.com' --data-urlencode 'label=first\nsecond' --data-urlencode 'text={{sms-body}}'"}, smsForwardConfig{})
	if err != nil {
		t.Fatal(err)
	}
	req, err = smsForwardRequest(context.Background(), c, smsForwardJob{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if err := req.ParseForm(); err != nil || req.PostForm.Get("label") != "first\nsecond" || req.PostForm.Get("text") != text {
		t.Fatal("multiline form value changed")
	}
}

func TestSMSCurlQuotedApostropheAndRawJSON(t *testing.T) {
	c := normalizeSMSRequest(smsForwardConfig{Endpoint: "https://example.com", Method: "POST", Format: "json", Body: `{"title":"Bob's notice","text":"{{sms-body}}"}`, Success: "http"})
	source := smsForwardCurl(c)
	parsed, err := prepareSMSForwardConfig(smsForwardConfig{Curl: source}, c)
	if err != nil {
		t.Fatal(err)
	}
	req, err := smsForwardRequest(context.Background(), parsed, smsForwardJob{Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(req.Body)
	var message map[string]string
	if json.Unmarshal(body, &message) != nil || message["title"] != "Bob's notice" {
		t.Fatal("single-quote escaping failed")
	}
}

func TestSMSCurlRejectsShellFilesAndCredentialReuseAcrossOrigins(t *testing.T) {
	for _, source := range []string{
		"curl https://example.com; echo unsafe",
		"curl \"https://example.com/$TOKEN\"",
		"curl https://example.com --data @/etc/passwd",
		"curl https://example.com --config /etc/curlrc",
		"curl -k https://example.com",
		"curl 'https://example.com",
	} {
		if _, err := prepareSMSForwardConfig(smsForwardConfig{Curl: source}, smsForwardConfig{}); err == nil {
			t.Fatal("unsupported command accepted")
		}
	}
	old := smsForwardConfig{Endpoint: "https://example.com", Authorization: "Bearer private-test"}
	if _, err := prepareSMSForwardConfig(smsForwardConfig{Curl: "curl 'https://other.example.com' -H 'Authorization: Bearer ****************'"}, old); err == nil {
		t.Fatal("masked credential reused on another origin")
	}
}
