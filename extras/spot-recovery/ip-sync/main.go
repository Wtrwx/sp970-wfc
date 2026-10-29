// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Wtrwx/sp970-wfc/internal/nodesync"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type config = nodesync.Config

var feedClient = nodesync.NewClient

type manager struct {
	client *http.Client
	token  string
}

func (m *manager) call(path string, body any, result any) error {
	method := "GET"
	var data io.Reader
	if body != nil {
		method = "POST"
		raw, e := json.Marshal(body)
		if e != nil {
			return e
		}
		data = bytes.NewReader(raw)
	}
	req, e := http.NewRequest(method, "http://127.0.0.1:17893"+path, data)
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	resp, e := m.client.Do(req)
	if e != nil {
		return errors.New("device manager unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("device manager HTTP %d", resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(result)
	}
	return nil
}

func backup(base string) error {
	dir := filepath.Join(base, "backups", "ip-sync-"+time.Now().UTC().Format("20060102T150405Z"))
	if e := os.MkdirAll(dir, 0700); e != nil {
		return errors.New("node backup directory unavailable")
	}
	for _, name := range []string{"proxy-uri", "proxy-ipv4", "proxy-client.json", "network-mode", "epdg-ipv4"} {
		b, e := os.ReadFile(filepath.Join(base, name))
		if e != nil {
			return errors.New("node backup read failed")
		}
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			return errors.New("node backup write failed")
		}
	}
	return nil
}

func syncOnce(c config, client *http.Client, check bool) (string, error) {
	result, e := nodesync.Check(context.Background(), c, client)
	if e != nil {
		return "", e
	}
	if result.State != "available" || check {
		return result.Message, nil
	}
	rawURI, uri := result.OriginalURI, result.URI
	m := manager{client: &http.Client{Timeout: 100 * time.Second, Transport: &http.Transport{Proxy: nil}}}
	var login struct {
		Token string `json:"token"`
	}
	if e = m.call("/login", map[string]string{"account": c.Account, "password": c.Password}, &login); e != nil {
		return "", e
	}
	if len(login.Token) != 64 {
		return "", errors.New("manager login failed")
	}
	m.token = login.Token
	defer m.call("/logout", struct{}{}, nil)
	var status struct {
		FlightMode bool   `json:"flight_mode"`
		SIMPrefix  string `json:"sim_prefix"`
		Proxy      struct {
			URI     string `json:"uri"`
			RawMode string `json:"raw_mode"`
			EPDG    string `json:"epdg"`
			WFCOnly bool   `json:"wfc_only"`
		} `json:"proxy"`
	}
	if e = m.call("/status", nil, &status); e != nil {
		return "", e
	}
	if !status.FlightMode || status.SIMPrefix != "23487" {
		return "", errors.New("flight mode or SIM policy guard rejected update")
	}
	if status.Proxy.RawMode != "ss" || strings.TrimSpace(status.Proxy.URI) != strings.TrimSpace(rawURI) {
		return "", errors.New("node changed during update; preserving user choice")
	}
	if e = backup(c.Base); e != nil {
		return "", e
	}
	if e = m.call("/network", map[string]any{"mode": "proxy", "uri": uri, "epdg": status.Proxy.EPDG, "wfc_only": status.Proxy.WFCOnly}, nil); e != nil {
		return "", e
	}
	return "updated managed node IP", nil
}

func main() {
	path := flag.String("config", "/data/adb/sp970-wfc/ip-sync.json", "private configuration")
	once := flag.Bool("once", false, "check once; apply only a matching node address change")
	check := flag.Bool("check", false, "read subscription without changing configuration")
	flag.Parse()
	raw, e := os.ReadFile(*path)
	var c config
	if e != nil || json.Unmarshal(raw, &c) != nil || c.Base != "/data/adb/sp970-wfc" {
		fmt.Fprintln(os.Stderr, "invalid sync configuration")
		os.Exit(1)
	}
	client, e := feedClient(c)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	if *once || *check {
		state, e := syncOnce(c, client, *check)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		fmt.Println(state)
		return
	}
	lock, e := os.OpenFile(filepath.Join(c.Base, "run", "ip-sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		os.Exit(1)
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return
	}
	last := ""
	for {
		state, e := syncOnce(c, client, false)
		if e != nil {
			state = "error: " + e.Error()
		}
		if state != last {
			fmt.Println(time.Now().UTC().Format(time.RFC3339), state)
			last = state
		}
		time.Sleep(time.Minute)
	}
}
