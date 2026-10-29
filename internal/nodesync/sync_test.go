package nodesync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrdinarySubscriptionURLsNeedNoKey(t *testing.T) {
	for _, link := range []string{"https://updates.example.invalid/nodes.json?token=example", "http://192.168.1.1/nodes.yaml"} {
		c, err := ParseDescriptor(link, Config{})
		if err != nil || Descriptor(c) != link {
			t.Fatal("ordinary subscription rejected")
		}
	}
	for _, link := range []string{"file:///etc/passwd", "https://user:secret@updates.example.invalid/nodes", "ftp://updates.example.invalid/nodes"} {
		if _, err := ParseDescriptor(link, Config{}); err == nil {
			t.Fatal("unsupported subscription URL accepted")
		}
	}
	migrated, err := ParseDescriptor("https://updates.example.invalid/nodes#key=obsolete&instance=obsolete", Config{})
	if err != nil || strings.Contains(Descriptor(migrated), "#") {
		t.Fatal("obsolete signature fragment was retained")
	}
}

func TestSubscriptionFormatsAndSSCredentials(t *testing.T) {
	credential := "aes-256-gcm:example/plus+=:value"
	plain := "ss://aes-256-gcm:example%2Fplus+%3D%3Avalue@uk.example.invalid:8443#UK"
	sip002 := "ss://" + base64.RawURLEncoding.EncodeToString([]byte(credential)) + "@uk.example.invalid:8443#UK"
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte(credential+"@uk.example.invalid:8443")) + "#UK"
	fixtures := map[string]string{
		"mihomo":      "proxies:\n  - {name: UK, type: ss, server: uk.example.invalid, port: 8443, cipher: aes-256-gcm, password: 'example/plus+=:value', udp: true}\n  - {name: Other, type: vless, server: other.example.invalid, port: 443}\n",
		"sing-box":    `{"outbounds":[{"type":"direct","tag":"direct"},{"type":"shadowsocks","tag":"UK","server":"uk.example.invalid","server_port":8443,"method":"aes-256-gcm","password":"example/plus+=:value"}]}`,
		"plain":       plain,
		"sip002":      sip002,
		"legacy":      legacy,
		"base64-list": base64.StdEncoding.EncodeToString([]byte(sip002 + "\n")),
	}
	var expectedHash string
	for name, text := range fixtures {
		t.Run(name, func(t *testing.T) {
			nodes, err := ParseSubscription([]byte(text))
			if err != nil || len(nodes) != 1 {
				t.Fatalf("cannot parse %s subscription: %v", name, err)
			}
			n := nodes[0]
			if n.Server != "uk.example.invalid" || n.Port != 8443 || n.Method != "aes-256-gcm" || n.Password != "example/plus+=:value" || n.Name != "UK" {
				t.Fatal("subscription altered node fields")
			}
		})
	}
	for _, link := range []string{plain, sip002, legacy} {
		got, err := CredentialHash(link)
		if err != nil {
			t.Fatal(err)
		}
		if expectedHash == "" {
			expectedHash = got
		}
		if got != expectedHash {
			t.Fatal("node encoding changed the credential identity")
		}
	}
	for _, text := range []string{
		`proxies: [{type: ss, server: host, port: 443, cipher: aes-256-gcm, password: demo, udp: false}]`,
		`{"outbounds":[{"type":"shadowsocks","server":"host","server_port":443,"method":"aes-256-gcm","password":"demo","network":"tcp"}]}`,
		`proxies: [{type: ss, server: host, port: 443, cipher: aes-256-gcm, password: demo, plugin: v2ray-plugin}]`,
		`{"payload":"obsolete-signed-envelope","signature":"obsolete"}`,
		strings.Repeat("x", maxSubscriptionSize+1),
	} {
		if _, err := ParseSubscription([]byte(text)); err == nil {
			t.Fatal("accepted an incompatible subscription")
		}
	}
}

func TestCheckMatchesCurrentNodeAndPreservesManualChoice(t *testing.T) {
	base := t.TempDir()
	original := "ss://aes-256-gcm:demo@1.1.1.1:443#Local-Name"
	os.WriteFile(filepath.Join(base, "network-mode"), []byte("ss\n"), 0600)
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(original), 0600)
	hash, _ := CredentialHash(original)
	body := `{"outbounds":[{"type":"shadowsocks","tag":"UK","server":"8.8.8.8","server_port":8443,"method":"aes-256-gcm","password":"demo"}]}`
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected credentials or mutation")
		}
		w.Write([]byte(body))
	}))
	defer server.Close()
	c := Config{Base: base, FeedURL: server.URL, CredentialSHA256: hash}
	result, err := Check(context.Background(), c, server.Client())
	if err != nil || result.State != "available" || !strings.Contains(result.URI, "@8.8.8.8:8443#Local-Name") {
		t.Fatalf("update failed: %v", err)
	}
	disk, _ := os.ReadFile(filepath.Join(base, "proxy-uri"))
	if string(disk) != original {
		t.Fatal("read-only check changed the local node")
	}
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(result.URI), 0600)
	result, err = Check(context.Background(), c, server.Client())
	if err != nil || result.State != "unchanged" {
		t.Fatal("unchanged node would be rewritten")
	}
	body = `{"proxies":[{"type":"ss","name":"A","server":"8.8.8.8","port":443,"cipher":"aes-256-gcm","password":"demo"},{"type":"ss","name":"B","server":"1.1.1.1","port":443,"cipher":"aes-256-gcm","password":"demo"}]}`
	if _, err = Check(context.Background(), c, server.Client()); err == nil {
		t.Fatal("ambiguous subscription was applied")
	}
	os.WriteFile(filepath.Join(base, "proxy-uri"), []byte(strings.Replace(original, ":demo@", ":manual@", 1)), 0600)
	before := requests
	result, err = Check(context.Background(), c, server.Client())
	if err != nil || result.State != "skipped" || requests != before {
		t.Fatal("manual node choice was not preserved")
	}
}

func TestCheckResultCannotSerializeNodeCredentials(t *testing.T) {
	raw, err := json.Marshal(Result{State: "available", URI: "ss://private-node-secret@8.8.8.8:443", OriginalURI: "ss://private-old-secret@1.1.1.1:443"})
	if err != nil || strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "ss://") {
		t.Fatal("node credentials became serializable")
	}
}
