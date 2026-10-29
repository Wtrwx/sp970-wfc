package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wtrwx/sp970-wfc/internal/nodesync"
)

func TestSteadyStateDoesNotLoginOrChangeDevice(t *testing.T) {
	base := t.TempDir()
	uri := "ss://aes-256-gcm:demo@8.8.8.8:443#demo"
	hash, _ := nodesync.CredentialHash(uri)
	c := config{Base: base, CredentialSHA256: hash}
	os.WriteFile(filepath.Join(base, "network-mode"), []byte("ss\n"), 0600)
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(uri), 0600)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" {
			t.Error("unexpected mutation")
		}
		w.Write([]byte(uri))
	}))
	defer srv.Close()
	c.FeedURL = srv.URL
	state, err := syncOnce(c, srv.Client(), false)
	if err != nil || state != "unchanged" || requests != 1 {
		t.Fatalf("steady state: %q %v requests=%d", state, err, requests)
	}
	os.WriteFile(filepath.Join(base, "network-mode"), []byte("direct\n"), 0600)
	state, err = syncOnce(c, srv.Client(), false)
	if err != nil || !strings.HasPrefix(state, "skipped:") || requests != 1 {
		t.Fatal("manual network choice was not respected")
	}
}
