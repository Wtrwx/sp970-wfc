// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/damonto/euicc-go/lpa"
	sgp22 "github.com/damonto/euicc-go/v2"
)

// Adapt only the transport: the pinned MIT LPA implements ES10 encoding,
// segmented APDUs and GET RESPONSE. Raw commands are never accepted by HTTP.
type stockEuiccChannel struct {
	ctx     context.Context
	cmd     *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Scanner
	channel byte
}

func (c *stockEuiccChannel) Connect() error {
	if err := requireFlight(c.ctx); err != nil {
		return err
	}
	c.cmd = exec.CommandContext(c.ctx, adminBase+"/bin/sp970_euicc_bridge")
	c.cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH=/system/vendor/lib:/system/lib")
	c.cmd.Stderr = io.Discard
	var err error
	c.input, err = c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := c.cmd.StdoutPipe()
	if err != nil {
		c.input.Close()
		return err
	}
	c.output = bufio.NewScanner(out)
	c.output.Buffer(make([]byte, 1024), 65536)
	if err = c.cmd.Start(); err != nil {
		c.input.Close()
		return err
	}
	if !c.output.Scan() {
		_ = c.Disconnect()
		return errors.New("无法打开 eUICC 管理通道")
	}
	fields := strings.Fields(c.output.Text())
	if len(fields) != 2 || fields[0] != "READY" {
		_ = c.Disconnect()
		return errors.New("小白卡未提供可用的 ISD-R 管理通道")
	}
	value, err := strconv.Atoi(fields[1])
	if err != nil || value < 1 || value > 19 {
		_ = c.Disconnect()
		return errors.New("逻辑通道响应无效")
	}
	c.channel = byte(value)
	return nil
}
func (c *stockEuiccChannel) Disconnect() error {
	if c.input != nil {
		_, _ = io.WriteString(c.input, "CLOSE\n")
		_ = c.input.Close()
		c.input = nil
	}
	if c.cmd != nil {
		err := c.cmd.Wait()
		c.cmd = nil
		return err
	}
	return nil
}
func (c *stockEuiccChannel) OpenLogicalChannel(aid []byte) (byte, error) {
	if !bytes.Equal(aid, lpa.GSMAISDRApplicationAID) {
		return 0, errors.New("只支持标准 ISD-R")
	}
	return c.channel, nil
}
func (c *stockEuiccChannel) CloseLogicalChannel(channel byte) error {
	if channel != c.channel {
		return errors.New("逻辑通道不匹配")
	}
	return nil
}
func (c *stockEuiccChannel) Transmit(command []byte) ([]byte, error) {
	if _, err := fmt.Fprintf(c.input, "%x\n", command); err != nil {
		return nil, errors.New("eUICC 通道已断开")
	}
	if !c.output.Scan() {
		return nil, errors.New("eUICC 没有响应")
	}
	parts := strings.Fields(c.output.Text())
	if len(parts) != 2 || parts[0] != "OK" {
		return nil, errors.New("eUICC 指令失败")
	}
	response, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("eUICC 响应无效")
	}
	return response, nil
}
func openEuicc(ctx context.Context) (*lpa.Client, error) {
	channel := &stockEuiccChannel{ctx: ctx}
	client, err := lpa.New(&lpa.Options{Channel: channel, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		_ = channel.Disconnect()
	}
	return client, err
}

type euiccProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	ICCID    string `json:"iccid"`
	Enabled  bool   `json:"enabled"`
}

func profileView(profile *sgp22.ProfileInfo) euiccProfile {
	name := profile.ProfileNickname
	if name == "" {
		name = profile.ProfileName
	}
	if name == "" {
		name = profile.ServiceProviderName
	}
	if name == "" {
		name = "eSIM 配置"
	}
	iccid := profile.ICCID.String()
	if len(iccid) > 4 {
		iccid = strings.Repeat("*", len(iccid)-4) + iccid[len(iccid)-4:]
	}
	return euiccProfile{ID: hex.EncodeToString(profile.ISDPAID), Name: name, Provider: profile.ServiceProviderName, ICCID: iccid, Enabled: profile.ProfileState == sgp22.ProfileEnabled}
}
func euiccProfiles(ctx context.Context) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	client, err := openEuicc(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, errors.New("无法读取 eSIM 配置列表：" + err.Error())
	}
	result := make([]euiccProfile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, profileView(p))
	}
	return map[string]any{"profiles": result}, nil
}
func switchEuicc(ctx context.Context, id string) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	aid, err := hex.DecodeString(id)
	if err != nil || len(aid) < 5 || len(aid) > 16 {
		return nil, errors.New("eSIM 配置标识无效")
	}
	// Look up the target on this card, instead of accepting arbitrary identifiers.
	client, err := openEuicc(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, errors.New("无法读取当前 eSIM 配置")
	}
	var target *sgp22.ProfileInfo
	for _, p := range profiles {
		if bytes.Equal(p.ISDPAID, aid) {
			target = p
			break
		}
	}
	if target == nil {
		return nil, errors.New("这张卡上没有该配置")
	}
	if target.ProfileState == sgp22.ProfileEnabled {
		return map[string]any{"ok": true, "changed": false}, nil
	}
	enabled := wfcEnabled()
	if err = serviceControl(ctx, "disable"); err != nil {
		return nil, err
	}
	if err = waitWFCStopped(ctx); err != nil {
		return nil, err
	}
	if err = requireFlight(ctx); err != nil {
		return nil, err
	}
	if err = client.EnableProfile(sgp22.ISDPAID(aid), true); err != nil {
		if enabled {
			_ = enableWFC(ctx)
		}
		return nil, errors.New("eSIM 切换失败：" + err.Error())
	}
	_ = client.Close()
	// The modem refresh can change the SIM identity. Never restart Lebara IMS
	// until the read-only policy guard confirms 23487 again.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(4 * time.Second):
	}
	if err = requireFlight(ctx); err != nil {
		return nil, err
	}
	message := "eSIM 已切换，WFC 已关闭"
	if enabled {
		if e := enableWFC(ctx); e != nil {
			message = "eSIM 已切换；SIM 策略未通过 23487 检查，WFC 保持关闭"
		} else {
			message = "eSIM 已切换，WFC 正在重新注册"
		}
	}
	return map[string]any{"ok": true, "changed": true, "message": message}, nil
}
