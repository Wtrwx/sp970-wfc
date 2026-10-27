// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAdminProxyImportPreservesPinAndEncodedCredential(t *testing.T) {
	pin := strings.Repeat("ab", 32)
	node, ip, err := parseProxyURI(context.Background(), "hy2://user%3Akey%40word@192.0.2.5:8443?insecure=1&sni=uk.example.com&pinSHA256="+pin)
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
}
func TestAdminProxyRejectsUnsafeOrAmbiguousImports(t *testing.T) {
	for _, uri := range []string{
		"https://192.0.2.5", "hy2://@192.0.2.5", "hy2://user:pass@192.0.2.5",
		"hy2://x@192.0.2.5?insecure=1", "hy2://x@192.0.2.5?pinSHA256=bad",
		"hy2://x@192.0.2.5?insecure=0&insecure=1", "hy2://x@192.0.2.5?command=sh",
		"hy2://x@[2001:db8::1]", "hy2://x@192.0.2.5:0", "hy2://x@192.0.2.5?obfs=other",
	} {
		if _, _, err := parseProxyURI(context.Background(), uri); err == nil {
			t.Errorf("accepted %q", uri)
		}
	}
}
func TestAdminRejectsUntrustedMutationBeforeDeviceCalls(t *testing.T) {
	host := &adminHost{}
	for _, path := range []string{"/flight", "/wifi/connect", "/wfc", "/network", "/euicc/enable"} {
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
