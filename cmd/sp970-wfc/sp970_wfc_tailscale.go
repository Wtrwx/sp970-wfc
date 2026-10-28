// SPDX-License-Identifier: AGPL-3.0-or-later
// Tailscale daemon and login management for SP970-B.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	tailscaledBin        = adminBase + "/bin/tailscaled"
	tailscaleBin         = adminBase + "/bin/tailscale"
	tailscaleStateDir    = adminBase + "/tailscale"
	tailscaleSock        = adminBase + "/run/tailscaled.sock"
	tailscalePidFile     = adminBase + "/run/tailscaled.pid"
	tailscaleEnabledFile = adminBase + "/tailscale-enabled"
	tailscaleLogFile     = adminBase + "/log/tailscaled.log"
	tailscaleSSHBinDir   = adminBase + "/ssh-bin"
)

var (
	tailscaleLoginMu  sync.Mutex
	tailscaleLoginCmd *exec.Cmd
)

func tailscaleInstalled() bool {
	s1, err1 := os.Stat(tailscaledBin)
	s2, err2 := os.Stat(tailscaleBin)
	return err1 == nil && err2 == nil && !s1.IsDir() && !s2.IsDir()
}

func tailscaleDaemonPID() int {
	raw, err := os.ReadFile(tailscalePidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 1 {
		return 0
	}
	if syscall.Kill(pid, 0) != nil {
		return 0
	}
	cmdline, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || !bytes.Contains(cmdline, []byte("tailscaled")) {
		return 0
	}
	return pid
}

func tailscaleEnv() []string {
	return append(os.Environ(),
		"SSL_CERT_FILE="+adminBase+"/ca-roots.pem",
		"HOME="+tailscaleStateDir,
		"SHELL=/system/bin/sh",
		"PATH="+tailscaleSSHBinDir+":"+os.Getenv("PATH"),
		"TAILSCALE_SSH_DEFAULT_PATH="+tailscaleSSHBinDir+":/system/bin:/system/xbin:/sbin:"+adminBase+"/bin",
	)
}

func prepareTailscaleSSH() error {
	const busybox = "/system/xbin/busybox"
	if _, err := os.Stat(busybox); os.IsNotExist(err) {
		return nil
	}
	// Android toolbox id ignores -G. Tailscale's Linux SSH helper needs
	// numeric group IDs, so use BusyBox only in Tailscale's private PATH.
	if err := os.MkdirAll(tailscaleSSHBinDir, 0700); err != nil {
		return errors.New("无法准备 Tailscale SSH 兼容命令")
	}
	id := tailscaleSSHBinDir + "/id"
	if target, err := os.Readlink(id); err == nil && target == busybox {
		return nil
	}
	if err := os.Symlink(busybox, id); err != nil {
		return errors.New("无法准备 Tailscale SSH 兼容命令")
	}
	return nil
}

func cancelTailscaleLoginHelper() {
	tailscaleLoginMu.Lock()
	defer tailscaleLoginMu.Unlock()
	if tailscaleLoginCmd != nil && tailscaleLoginCmd.Process != nil {
		_ = tailscaleLoginCmd.Process.Kill()
		tailscaleLoginCmd = nil
	}
}

type tailscaleCommand func(context.Context, ...string) ([]byte, error)

func runTailscale(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, tailscaleBin, append([]string{"--socket=" + tailscaleSock}, args...)...)
	cmd.Env = tailscaleEnv()
	return cmd.Output()
}

func queryTailscaleSSH(ctx context.Context, run tailscaleCommand) (bool, error) {
	qctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	out, err := run(qctx, "debug", "prefs")
	defer clear(out)
	var prefs struct {
		RunSSH *bool
	}
	if err != nil || json.Unmarshal(out, &prefs) != nil || prefs.RunSSH == nil {
		return false, errors.New("无法读取 Tailscale SSH 状态")
	}
	return *prefs.RunSSH, nil
}

func changeTailscaleSSH(ctx context.Context, enabled bool, run tailscaleCommand) error {
	if _, err := queryTailscaleSSH(ctx, run); err != nil {
		return err
	}
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	out, err := run(sctx, "set", "--ssh="+strconv.FormatBool(enabled))
	clear(out)
	cancel()
	if err != nil {
		return errors.New("设置 Tailscale SSH 失败")
	}
	actual, err := queryTailscaleSSH(ctx, run)
	if err != nil {
		return err
	}
	if actual != enabled {
		return errors.New("Tailscale SSH 状态未更新，请重试")
	}
	return nil
}

func setTailscaleSSH(ctx context.Context, enabled bool) (map[string]any, error) {
	if !tailscaleInstalled() {
		return nil, errors.New("设备未安装 Tailscale 组件")
	}
	if tailscaleDaemonPID() == 0 {
		return nil, errors.New("请先开启 Tailscale")
	}
	if err := changeTailscaleSSH(ctx, enabled, runTailscale); err != nil {
		return nil, err
	}
	return tailscaleSummary(ctx), nil
}

func queryTailscaleStatusJSON(ctx context.Context) (map[string]any, error) {
	qctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(qctx, tailscaleBin, "--socket="+tailscaleSock, "status", "--json")
	cmd.Env = tailscaleEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func tailscaleSummary(ctx context.Context) map[string]any {
	installed := tailscaleInstalled()
	_, prefErr := os.Stat(tailscaleEnabledFile)
	prefEnabled := prefErr == nil
	pid := tailscaleDaemonPID()
	summary := map[string]any{
		"installed":     installed,
		"enabled":       prefEnabled && pid > 0,
		"running":       pid > 0,
		"backend_state": "Stopped",
		"ip":            "",
		"hostname":      "",
		"dns_name":      "",
		"tailnet":       "",
		"account":       "",
		"auth_url":      "",
		"ssh_enabled":   false,
		"ssh_available": false,
	}
	if !installed || pid == 0 {
		return summary
	}
	if enabled, err := queryTailscaleSSH(ctx, runTailscale); err == nil {
		summary["ssh_enabled"] = enabled
		summary["ssh_available"] = true
	}
	raw, err := queryTailscaleStatusJSON(ctx)
	if err != nil {
		summary["backend_state"] = "Starting"
		return summary
	}
	backendState, _ := raw["BackendState"].(string)
	if backendState == "" {
		backendState = "Stopped"
	}
	summary["backend_state"] = backendState
	summary["enabled"] = backendState != "Stopped" || prefEnabled
	if authURL, _ := raw["AuthURL"].(string); authURL != "" {
		summary["auth_url"] = authURL
	}
	if ips, ok := raw["TailscaleIPs"].([]any); ok {
		for _, item := range ips {
			if s, ok := item.(string); ok && s != "" {
				if strings.Contains(s, ".") {
					summary["ip"] = s
					break
				}
				if summary["ip"] == "" {
					summary["ip"] = s
				}
			}
		}
	}
	var selfUserID float64
	if selfMap, ok := raw["Self"].(map[string]any); ok {
		if hn, _ := selfMap["HostName"].(string); hn != "" {
			summary["hostname"] = hn
		}
		if dn, _ := selfMap["DNSName"].(string); dn != "" {
			summary["dns_name"] = strings.TrimSuffix(dn, ".")
		}
		selfUserID, _ = selfMap["UserID"].(float64)
	}
	if tn, ok := raw["CurrentTailnet"].(map[string]any); ok {
		if name, _ := tn["Name"].(string); name != "" {
			summary["tailnet"] = name
		}
	}
	if users, ok := raw["User"].(map[string]any); ok && selfUserID > 0 {
		uidKey := strconv.FormatInt(int64(selfUserID), 10)
		if uMap, ok := users[uidKey].(map[string]any); ok {
			if login, _ := uMap["LoginName"].(string); login != "" {
				summary["account"] = login
			} else if disp, _ := uMap["DisplayName"].(string); disp != "" {
				summary["account"] = disp
			}
		}
	}
	return summary
}

func ensureTailscaled(ctx context.Context) error {
	if !tailscaleInstalled() {
		return errors.New("设备未安装 Tailscale 组件")
	}
	if tailscaleDaemonPID() > 0 {
		if _, err := queryTailscaleStatusJSON(ctx); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(tailscaleStateDir, 0700); err != nil {
		return errors.New("无法创建 Tailscale 状态目录")
	}
	if err := prepareTailscaleSSH(); err != nil {
		return err
	}
	_ = os.MkdirAll(adminBase+"/run", 0700)
	_ = os.MkdirAll(adminBase+"/log", 0700)
	_ = os.Remove(tailscaleSock)

	logFile, err := os.OpenFile(tailscaleLogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return errors.New("无法打开 Tailscale 日志文件")
	}
	cmd := exec.Command(tailscaledBin,
		"--statedir="+tailscaleStateDir,
		"--socket="+tailscaleSock,
		"--tun=tailscale0",
		"--socks5-server=127.0.0.1:1055",
	)
	cmd.Env = tailscaleEnv()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return errors.New("无法启动 tailscaled 守护进程")
	}
	pid := cmd.Process.Pid
	_ = os.WriteFile(tailscalePidFile, []byte(strconv.Itoa(pid)+"\n"), 0600)
	go func() {
		_ = cmd.Wait()
		_ = logFile.Close()
	}()

	for i := 0; i < 20; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		if _, err := queryTailscaleStatusJSON(ctx); err == nil {
			return nil
		}
	}
	return errors.New("Tailscale 服务启动超时")
}

func stopTailscaled(ctx context.Context) {
	cancelTailscaleLoginHelper()
	pid := tailscaleDaemonPID()
	if pid > 0 {
		dctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		down := exec.CommandContext(dctx, tailscaleBin, "--socket="+tailscaleSock, "down")
		down.Env = tailscaleEnv()
		_ = down.Run()
		cancel()

		_ = syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 12; i++ {
			if syscall.Kill(pid, 0) != nil {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	_ = os.Remove(tailscalePidFile)
	_ = os.Remove(tailscaleSock)
}

func setTailscaleEnabled(ctx context.Context, enabled bool) (map[string]any, error) {
	if !enabled {
		_ = os.Remove(tailscaleEnabledFile)
		stopTailscaled(ctx)
		return tailscaleSummary(ctx), nil
	}
	if err := os.WriteFile(tailscaleEnabledFile, []byte("1\n"), 0600); err != nil {
		return nil, errors.New("无法保存 Tailscale 开关状态")
	}
	if err := ensureTailscaled(ctx); err != nil {
		return nil, err
	}
	st := tailscaleSummary(ctx)
	if st["backend_state"] == "Stopped" {
		uctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		up := exec.CommandContext(uctx, tailscaleBin, "--socket="+tailscaleSock,
			"up")
		up.Env = tailscaleEnv()
		_ = up.Run()
		cancel()
		st = tailscaleSummary(ctx)
	}
	return st, nil
}

func loginTailscale(ctx context.Context, authKey string) (map[string]any, error) {
	authKey = strings.TrimSpace(authKey)
	if len(authKey) > 512 || strings.ContainsAny(authKey, "\r\n\x00 ") {
		return nil, errors.New("Auth Key 格式无效")
	}
	if err := os.WriteFile(tailscaleEnabledFile, []byte("1\n"), 0600); err != nil {
		return nil, errors.New("无法保存 Tailscale 状态")
	}
	if err := ensureTailscaled(ctx); err != nil {
		return nil, err
	}
	cancelTailscaleLoginHelper()

	if authKey != "" {
		uctx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		cmd := exec.CommandContext(uctx, tailscaleBin, "--socket="+tailscaleSock,
			"up", "--authkey="+authKey, "--accept-dns=false", "--netfilter-mode=off", "--hostname=sp970-b", "--reset", "--timeout=22s")
		cmd.Env = tailscaleEnv()
		out, err := cmd.CombinedOutput()
		clear(out)
		if err != nil {
			return nil, errors.New("Tailscale Auth Key 登录失败，请检查密钥与网络连接")
		}
		return tailscaleSummary(ctx), nil
	}

	// Interactive browser login: keep `tailscale up` running in background for up to 10 minutes
	// so `tailscaled` keeps the interactive AuthURL session active until the user approves it.
	cmd := exec.Command(tailscaleBin, "--socket="+tailscaleSock,
		"up", "--accept-dns=false", "--netfilter-mode=off", "--hostname=sp970-b", "--timeout=10m")
	cmd.Env = tailscaleEnv()
	if err := cmd.Start(); err != nil {
		return nil, errors.New("无法发起 Tailscale 登录")
	}
	tailscaleLoginMu.Lock()
	tailscaleLoginCmd = cmd
	tailscaleLoginMu.Unlock()
	go func(c *exec.Cmd) {
		_ = c.Wait()
		tailscaleLoginMu.Lock()
		if tailscaleLoginCmd == c {
			tailscaleLoginCmd = nil
		}
		tailscaleLoginMu.Unlock()
	}(cmd)

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
		st := tailscaleSummary(ctx)
		if urlStr, _ := st["auth_url"].(string); urlStr != "" {
			return st, nil
		}
		if st["backend_state"] == "Running" {
			return st, nil
		}
	}
	return nil, errors.New("无法获取 Tailscale 登录链接，请确认设备已连接互联网")
}

func logoutTailscale(ctx context.Context) (map[string]any, error) {
	cancelTailscaleLoginHelper()
	if tailscaleDaemonPID() == 0 {
		return tailscaleSummary(ctx), nil
	}
	lctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(lctx, tailscaleBin, "--socket="+tailscaleSock, "logout")
	cmd.Env = tailscaleEnv()
	if err := cmd.Run(); err != nil {
		return nil, errors.New("退出 Tailscale 登录失败")
	}
	return tailscaleSummary(ctx), nil
}
