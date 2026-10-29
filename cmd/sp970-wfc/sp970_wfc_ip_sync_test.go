// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wtrwx/sp970-wfc/internal/nodesync"
)

func TestIPSubscriptionSettingsPreserveBindingAndHideLocalSecrets(t *testing.T) {
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "run"), 0700)
	node := "ss://2022-blake3-aes-256-gcm:local-secret@8.8.8.8:443#Example"
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(node), 0600)
	hash, _ := nodesync.CredentialHash(node)
	config := nodesync.Config{FeedURL: "https://updates.example.invalid/node.json", CredentialSHA256: hash, Account: "legacy-account", Password: "legacy-private-password", IntervalSeconds: 60}
	privateJSON(filepath.Join(base, "ip-sync.json"), config)
	enabled := true
	request := ipSyncSettings{Enabled: &enabled, SubscriptionURL: nodesync.Descriptor(config), IntervalSeconds: 300}
	if err := saveIPSyncSettings(base, request); err != nil {
		t.Fatal(err)
	}
	saved, err := loadIPSyncConfig(base)
	if err != nil || saved.Account != "" || saved.Password != "" || saved.CredentialSHA256 != hash || saved.IntervalSeconds != 300 {
		t.Fatal("saved subscription changed binding or retained login credentials")
	}
	// Editing an interval after a manual node change must not silently rebind.
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(strings.Replace(node, "local-secret", "another-secret", 1)), 0600)
	if err = saveIPSyncSettings(base, request); err != nil {
		t.Fatal(err)
	}
	saved, _ = loadIPSyncConfig(base)
	if saved.CredentialSHA256 != hash {
		t.Fatal("manual node choice was rebound")
	}
	raw, _ := json.Marshal(ipSyncSummaryAt(base, time.Now()))
	for _, forbidden := range []string{"local-secret", "legacy-private-password", "legacy-account", "credential_sha256", "password", "account"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("private configuration entered the public API")
		}
	}
	enabled = false
	if err = saveIPSyncSettings(base, request); err != nil || ipSyncSummaryAt(base, time.Now())["state"] != "paused" {
		t.Fatal("pause preference did not persist")
	}
	if info, _ := os.Stat(filepath.Join(base, "ip-sync.json")); info.Mode().Perm() != 0600 {
		t.Fatal("subscription configuration is not private")
	}
}

func TestIPSubscriptionManagementRequiresLogin(t *testing.T) {
	for _, route := range []struct{ method, path string }{{"GET", "/ip-sync"}, {"POST", "/ip-sync"}, {"POST", "/ip-sync/check"}} {
		request := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"enabled":true}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		(&adminHost{}).api(response, request)
		if response.Code != 401 {
			t.Fatalf("unauthenticated route %s returned %d", route.path, response.Code)
		}
	}
}
