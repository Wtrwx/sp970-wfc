// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wtrwx/sp970-wfc/internal/nodesync"
)

type ipSyncSettings struct {
	Enabled         *bool  `json:"enabled"`
	SubscriptionURL string `json:"subscription_url"`
	IntervalSeconds int    `json:"interval_seconds"`
}

type ipSyncState struct {
	CheckedAt time.Time `json:"checked_at"`
	State     string    `json:"state"`
	Error     string    `json:"error,omitempty"`
}

func loadIPSyncConfig(base string) (nodesync.Config, error) {
	var c nodesync.Config
	raw, err := os.ReadFile(filepath.Join(base, "ip-sync.json"))
	if err != nil {
		return c, err
	}
	defer clear(raw)
	if json.Unmarshal(raw, &c) != nil {
		return c, errors.New("节点订阅配置无效")
	}
	if c.IntervalSeconds == 0 {
		c.IntervalSeconds = 60
	}
	return c, nil
}

func ipSyncSummaryAt(base string, now time.Time) map[string]any {
	c, err := loadIPSyncConfig(base)
	configured := err == nil && c.CredentialSHA256 != "" && nodesync.Descriptor(c) != ""
	_, paused := os.Stat(filepath.Join(base, "ip-sync-disabled"))
	enabled := configured && os.IsNotExist(paused)
	interval := c.IntervalSeconds
	if interval < 60 {
		interval = 60
	}
	state := ipSyncState{}
	if raw, e := os.ReadFile(filepath.Join(base, "run/ip-sync-state.json")); e == nil {
		_ = json.Unmarshal(raw, &state)
	}
	code := state.State
	if !configured {
		code = "unconfigured"
	} else if !enabled {
		code = "paused"
	} else if state.CheckedAt.IsZero() {
		code = "waiting"
	} else if now.Sub(state.CheckedAt) > time.Duration(interval+90)*time.Second {
		code = "stale"
	}
	last := ""
	if !state.CheckedAt.IsZero() {
		last = state.CheckedAt.UTC().Format(time.RFC3339)
	}
	// The full private configuration may contain legacy UI credentials. Only
	// this explicit public view is returned; never serialize the configuration.
	return map[string]any{"configured": configured, "enabled": enabled,
		"subscription_url": nodesync.Descriptor(c), "interval_seconds": interval,
		"state": code, "last_checked_at": last, "error": state.Error}
}

func saveIPSyncSettings(base string, request ipSyncSettings) error {
	if request.Enabled == nil || request.IntervalSeconds < 60 || request.IntervalSeconds > 86400 {
		return errors.New("请设置自动更新开关和 1 分钟至 24 小时的更新间隔")
	}
	previous, _ := loadIPSyncConfig(base)
	link := strings.TrimSpace(request.SubscriptionURL)
	if link == "" {
		if *request.Enabled {
			return errors.New("请先填写节点订阅链接")
		}
		return os.WriteFile(filepath.Join(base, "ip-sync-disabled"), []byte("paused\n"), 0600)
	}
	c, err := nodesync.ParseDescriptor(link, previous)
	if err != nil {
		return err
	}
	c.Base, c.CAFile, c.IntervalSeconds = base, filepath.Join(base, "ca-roots.pem"), request.IntervalSeconds
	if c.CredentialSHA256 == "" || c.FeedURL != previous.FeedURL {
		raw, e := os.ReadFile(filepath.Join(base, "proxy-uri"))
		if e != nil {
			return errors.New("请先保存要绑定的 Shadowsocks 节点")
		}
		c.CredentialSHA256, err = nodesync.CredentialHash(string(raw))
		clear(raw)
		if err != nil {
			return err
		}
	}
	// The native manager applies subscription changes itself and needs no saved
	// account, password or persistent browser session.
	c.Account, c.Password = "", ""
	if err = privateJSON(filepath.Join(base, "ip-sync.json"), c); err != nil {
		return errors.New("无法保存节点订阅")
	}
	if *request.Enabled {
		err = os.Remove(filepath.Join(base, "ip-sync-disabled"))
		if os.IsNotExist(err) {
			err = nil
		}
		return err
	}
	return os.WriteFile(filepath.Join(base, "ip-sync-disabled"), []byte("paused\n"), 0600)
}

func backupIPSyncNode(base string) error {
	dir := filepath.Join(base, "backups", "ip-sync-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("无法建立节点备份")
	}
	for _, name := range []string{"proxy-uri", "proxy-ipv4", "proxy-client.json", "network-mode", "epdg-ipv4"} {
		raw, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			return errors.New("无法读取原节点备份")
		}
		err = os.WriteFile(filepath.Join(dir, name), raw, 0600)
		clear(raw)
		if err != nil {
			return errors.New("无法保存原节点备份")
		}
	}
	return nil
}

// Caller holds h.mutations; configuration, SIM changes and manual network
// changes therefore cannot race a subscription update.
func (h *adminHost) checkIPSyncLocked(ctx context.Context) error {
	c, err := loadIPSyncConfig(adminBase)
	if err == nil {
		var result nodesync.Result
		client, e := nodesync.NewClient(c)
		err = e
		if err == nil {
			defer client.CloseIdleConnections()
			result, err = nodesync.Check(ctx, c, client)
		}
		if err == nil && result.State == "available" {
			var status map[string]any
			status, err = androidCall(ctx, map[string]string{"action": "status"})
			if err == nil && (status["flight_mode"] != true || status["sim_prefix"] != "23487") {
				err = errors.New("飞行模式或 SIM 策略检查未通过")
			}
			if err == nil && readSetting("proxy-uri") != result.OriginalURI {
				err = errors.New("节点已被手动更换，已保留当前设置")
			}
			if err == nil {
				err = backupIPSyncNode(adminBase)
			}
			if err == nil {
				p := proxySummary()
				err = saveNetwork(ctx, networkSettings{Mode: "proxy", URI: result.URI,
					EPDG: p["epdg"].(string), WFCOnly: p["wfc_only"].(bool)})
				if err == nil {
					result.State = "updated"
				}
			}
		}
		state := ipSyncState{CheckedAt: time.Now().UTC(), State: result.State}
		if err != nil {
			state.State, state.Error = "error", err.Error()
		}
		if e := privateJSON(adminBase+"/run/ip-sync-state.json", state); e != nil && err == nil {
			err = errors.New("无法保存订阅更新状态")
		}
	}
	if err != nil && os.IsNotExist(err) {
		return errors.New("请先配置 节点订阅")
	}
	return err
}

func (h *adminHost) wakeIPSync() {
	select {
	case h.ipSyncWake <- struct{}{}:
	default:
	}
}

func (h *adminHost) ipSyncLoop(ctx context.Context) {
	lock, err := os.OpenFile(adminBase+"/run/ip-sync.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return
	}
	defer lock.Close()
	// Share the lock with the optional legacy standalone client during upgrades.
	for syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	next := time.Time{}
	for {
		view := ipSyncSummaryAt(adminBase, time.Now())
		if view["enabled"] == true && !time.Now().Before(next) {
			h.mutations.Lock()
			// Recheck the saved toggle after acquiring the mutation lock.
			if ipSyncSummaryAt(adminBase, time.Now())["enabled"] == true {
				_ = h.checkIPSyncLocked(ctx)
			}
			h.mutations.Unlock()
			next = time.Now().Add(time.Duration(view["interval_seconds"].(int)) * time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-h.ipSyncWake:
			next = time.Time{}
		case <-tick.C:
		}
	}
}
