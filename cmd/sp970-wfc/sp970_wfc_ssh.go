// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const sshBin = adminBase + "/bin/dropbearmulti"
const sshEnabledFile = adminBase + "/ssh-enabled"

func sshInstalled() bool {
	info, err := os.Stat(sshBin)
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}

func sshDaemonPID() int {
	raw, err := os.ReadFile(adminBase + "/run/ssh.pid")
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 1 || syscall.Kill(pid, 0) != nil {
		return 0
	}
	proc := "/proc/" + strconv.Itoa(pid)
	exe, err := os.Readlink(proc + "/exe")
	if err != nil || exe != sshBin {
		return 0
	}
	cmd, err := os.ReadFile(proc + "/cmdline")
	if err != nil || !bytes.Contains(cmd, []byte("\x00dropbear\x00")) {
		return 0
	}
	return pid
}

func sshSummary() map[string]any {
	_, err := os.Stat(sshEnabledFile)
	return map[string]any{
		"installed": sshInstalled(), "enabled": err == nil,
		"running": sshDaemonPID() > 0, "port": 22, "auth": "publickey",
	}
}

func setSSHEnabled(ctx context.Context, enabled bool) (map[string]any, error) {
	if enabled && !sshInstalled() {
		return nil, errors.New("设备未安装 SSH 组件")
	}
	if enabled {
		if err := os.WriteFile(sshEnabledFile, []byte("1\n"), 0600); err != nil {
			return nil, errors.New("无法保存 SSH 开关状态")
		}
	} else if err := os.Remove(sshEnabledFile); err != nil && !os.IsNotExist(err) {
		return nil, errors.New("无法保存 SSH 开关状态")
	}
	sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(sctx, "/system/bin/sh", adminBase+"/service-control.sh", "ssh")
	if err := cmd.Run(); err != nil {
		return nil, errors.New("设置 SSH 失败，请检查设备日志")
	}
	st := sshSummary()
	if st["running"] != enabled {
		return nil, errors.New("SSH 状态未更新，请重试")
	}
	return st, nil
}
