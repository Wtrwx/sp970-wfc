// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/damonto/euicc-go/http/rootci"
	"github.com/damonto/euicc-go/lpa"
	sgp22 "github.com/damonto/euicc-go/v2"
)

const fallbackEuiccIMEI = "356938035643809"

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

type smdpRoundTripper struct {
	base http.RoundTripper
}

func (s *smdpRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = strings.ReplaceAll(req.URL.Host, " ", "")
	return s.base.RoundTrip(req)
}

func euiccHTTPClient() *http.Client {
	roots := rootci.TrustedRootCAs()
	if roots == nil {
		roots = x509.NewCertPool()
	}
	if pem, err := os.ReadFile(adminBase + "/ca-roots.pem"); err == nil {
		roots.AppendCertsFromPEM(pem)
		clear(pem)
	}
	resolver := deviceResolver()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
			return dialForwardTCP(ctx, resolver, address)
		},
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     30 * time.Second,
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: &smdpRoundTripper{base: tr},
	}
}

func openEuicc(ctx context.Context) (*lpa.Client, error) {
	channel := &stockEuiccChannel{ctx: ctx}
	client, err := lpa.New(&lpa.Options{
		Channel: channel,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Timeout: 60 * time.Second,
	})
	if err != nil {
		_ = channel.Disconnect()
		return nil, err
	}
	if client.HTTP != nil {
		client.HTTP.Client = euiccHTTPClient()
	}
	return client, nil
}

type euiccProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Nickname    string `json:"nickname"`
	ProfileName string `json:"profile_name"`
	Provider    string `json:"provider"`
	ICCID       string `json:"iccid"`
	Enabled     bool   `json:"enabled"`
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
	return euiccProfile{
		ID:          hex.EncodeToString(profile.ISDPAID),
		Name:        name,
		Nickname:    profile.ProfileNickname,
		ProfileName: profile.ProfileName,
		Provider:    profile.ServiceProviderName,
		ICCID:       profile.ICCID.String(),
		Enabled:     profile.ProfileState == sgp22.ProfileEnabled,
	}
}

func readEuiccState(client *lpa.Client) (map[string]any, error) {
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, errors.New("无法读取 eSIM 配置列表：" + err.Error())
	}
	result := make([]euiccProfile, 0, len(profiles))
	for _, p := range profiles {
		result = append(result, profileView(p))
	}
	eidHex := ""
	if eid, e := client.EID(); e == nil && len(eid) > 0 {
		eidHex = strings.ToUpper(hex.EncodeToString(eid))
	}
	return map[string]any{"eid": eidHex, "profiles": result}, nil
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
	return readEuiccState(client)
}

func findTargetProfile(profiles []*sgp22.ProfileInfo, idHex, iccidStr string) *sgp22.ProfileInfo {
	idHex = strings.ToLower(strings.TrimSpace(idHex))
	iccidStr = strings.TrimSpace(iccidStr)
	for _, p := range profiles {
		if idHex != "" && hex.EncodeToString(p.ISDPAID) == idHex {
			return p
		}
		if iccidStr != "" && p.ICCID.String() == iccidStr {
			return p
		}
	}
	return nil
}

func flushEuiccNotifications(client *lpa.Client, events ...sgp22.NotificationEvent) {
	for _, ev := range events {
		pending, err := client.RetrieveNotificationList(ev)
		if err != nil {
			continue
		}
		for _, item := range pending {
			if item == nil || item.Notification == nil {
				continue
			}
			if client.HandleNotification(item) == nil {
				_ = client.RemoveNotificationFromList(item.Notification.SequenceNumber)
			}
		}
	}
}

func switchEuicc(ctx context.Context, id string) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	aid, err := hex.DecodeString(strings.TrimSpace(id))
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
	flushEuiccNotifications(client, sgp22.NotificationEventEnable, sgp22.NotificationEventDisable)
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
	message := "eSIM 已切换，Wi-Fi 通话已关闭"
	if enabled {
		if e := enableWFC(ctx); e != nil {
			message = "eSIM 已切换；当前卡非 23487，Wi-Fi 通话保持关闭"
		} else {
			message = "eSIM 已切换，Wi-Fi 通话正在重新注册"
		}
	}
	return map[string]any{"ok": true, "changed": true, "message": message}, nil
}

func disableEuicc(ctx context.Context, id string) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	aid, err := hex.DecodeString(strings.TrimSpace(id))
	if err != nil || len(aid) < 5 || len(aid) > 16 {
		return nil, errors.New("eSIM 配置标识无效")
	}
	client, err := openEuicc(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, errors.New("无法读取当前 eSIM 配置")
	}
	target := findTargetProfile(profiles, id, "")
	if target == nil {
		return nil, errors.New("这张卡上没有该配置")
	}
	if target.ProfileState != sgp22.ProfileEnabled {
		st, _ := readEuiccState(client)
		return st, nil
	}
	if err = serviceControl(ctx, "disable"); err != nil {
		return nil, err
	}
	if err = waitWFCStopped(ctx); err != nil {
		return nil, err
	}
	if err = requireFlight(ctx); err != nil {
		return nil, err
	}
	if err = client.DisableProfile(sgp22.ISDPAID(aid), true); err != nil {
		return nil, errors.New("停用 eSIM 配置失败：" + err.Error())
	}
	flushEuiccNotifications(client, sgp22.NotificationEventDisable)
	return readEuiccState(client)
}

func renameEuicc(ctx context.Context, id, iccid, nickname string) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	nickname = strings.TrimSpace(nickname)
	if len(nickname) > 64 || strings.ContainsAny(nickname, "\r\n\x00") {
		return nil, errors.New("配置名称不能超过 64 字节")
	}
	client, err := openEuicc(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	profiles, err := client.ListProfile(nil, nil)
	if err != nil {
		return nil, errors.New("无法读取当前 eSIM 配置")
	}
	target := findTargetProfile(profiles, id, iccid)
	if target == nil {
		return nil, errors.New("未找到指定的 eSIM 配置")
	}
	if err := client.SetNickname(target.ICCID, nickname); err != nil {
		return nil, errors.New("修改配置名称失败：" + err.Error())
	}
	return readEuiccState(client)
}

func deleteEuicc(ctx context.Context, id, iccid string) (any, error) {
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
		return nil, errors.New("无法读取当前 eSIM 配置")
	}
	target := findTargetProfile(profiles, id, iccid)
	if target == nil {
		return nil, errors.New("未找到指定的 eSIM 配置")
	}
	if target.ProfileState == sgp22.ProfileEnabled {
		return nil, errors.New("请先停用该 eSIM 配置后再删除")
	}
	if err := client.DeleteProfile(sgp22.ISDPAID(target.ISDPAID)); err != nil {
		return nil, errors.New("删除 eSIM 配置失败：" + err.Error())
	}
	flushEuiccNotifications(client, sgp22.NotificationEventDelete)
	return readEuiccState(client)
}

type euiccDownloadRequest struct {
	Code             string `json:"code"`
	SMDP             string `json:"smdp"`
	MatchingID       string `json:"matching_id"`
	ConfirmationCode string `json:"confirmation_code"`
	IMEI             string `json:"imei"`
	Nickname         string `json:"nickname"`
}

func parseEuiccActivation(req euiccDownloadRequest, deviceIMEI string) (*lpa.ActivationCode, error) {
	code := strings.TrimSpace(req.Code)
	smdp := strings.TrimSpace(req.SMDP)
	matching := strings.TrimSpace(req.MatchingID)
	confirm := strings.TrimSpace(req.ConfirmationCode)
	if len(code) > 1024 || len(smdp) > 255 || len(matching) > 255 || len(confirm) > 128 {
		return nil, errors.New("eSIM 激活码参数过长")
	}
	ac := &lpa.ActivationCode{}
	if code != "" {
		if strings.HasPrefix(code, "1$") {
			code = "LPA:" + code
		} else if !strings.HasPrefix(strings.ToUpper(code), "LPA:") && strings.Contains(code, "$") {
			code = "LPA:1$" + code
		} else if strings.HasPrefix(strings.ToUpper(code), "LPA:") {
			code = "LPA:" + code[4:]
		}
		if err := ac.UnmarshalText([]byte(code)); err != nil || ac.SMDP == nil || ac.SMDP.Hostname() == "" {
			return nil, errors.New("激活码格式无效（示例：LPA:1$smdp.example.com$MATCHING-ID）")
		}
	} else if smdp != "" {
		smdp = strings.TrimPrefix(strings.TrimPrefix(smdp, "https://"), "http://")
		smdp = strings.TrimSuffix(smdp, "/")
		u, err := url.Parse("https://" + smdp)
		if err != nil || u.Hostname() == "" || u.Path != "" || u.User != nil {
			return nil, errors.New("SM-DP+ 服务器地址无效")
		}
		ac.SMDP = u
		ac.MatchingID = matching
	} else {
		return nil, errors.New("请填写 LPA 激活码或 SM-DP+ 服务器地址")
	}
	ac.ConfirmationCode = confirm

	chosenIMEI := strings.TrimSpace(req.IMEI)
	if chosenIMEI == "" {
		chosenIMEI = strings.TrimSpace(deviceIMEI)
	}
	if chosenIMEI == "" {
		chosenIMEI = fallbackEuiccIMEI
	}
	if len(chosenIMEI) < 14 || len(chosenIMEI) > 16 {
		if strings.TrimSpace(req.IMEI) != "" {
			return nil, errors.New("IMEI 格式无效（需为 14–15 位有效数字）")
		}
		chosenIMEI = fallbackEuiccIMEI
	}
	if _, err := sgp22.NewIMEI(chosenIMEI); err != nil {
		if strings.TrimSpace(req.IMEI) != "" {
			return nil, errors.New("IMEI 格式无效（需为 14–15 位有效数字）")
		}
		chosenIMEI = fallbackEuiccIMEI
	}
	ac.IMEI = chosenIMEI
	return ac, nil
}

func downloadEuicc(ctx context.Context, req euiccDownloadRequest) (any, error) {
	if err := requireFlight(ctx); err != nil {
		return nil, err
	}
	nickname := strings.TrimSpace(req.Nickname)
	if len(nickname) > 64 || strings.ContainsAny(nickname, "\r\n\x00") {
		return nil, errors.New("配置名称不能超过 64 字节")
	}
	deviceIMEI := ""
	if st, err := androidCall(ctx, map[string]string{"action": "status"}); err == nil {
		deviceIMEI, _ = st["imei"].(string)
	}
	ac, err := parseEuiccActivation(req, deviceIMEI)
	if err != nil {
		return nil, err
	}
	client, err := openEuicc(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	beforeProfiles, _ := client.ListProfile(nil, nil)
	existingICCIDs := map[string]bool{}
	for _, p := range beforeProfiles {
		existingICCIDs[p.ICCID.String()] = true
	}

	_, err = client.DownloadProfile(ctx, ac, &lpa.DownloadOptions{
		OnConfirm: func(*sgp22.ProfileInfo) bool { return true },
		OnEnterConfirmationCode: func() string {
			return ac.ConfirmationCode
		},
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "confirmation code is required") {
			return nil, errors.New("此 eSIM 需要填写确认码（Confirmation Code）")
		}
		return nil, errors.New("下载安装 eSIM 失败：" + msg)
	}
	flushEuiccNotifications(client, sgp22.NotificationEventInstall)

	if nickname != "" {
		if afterProfiles, e := client.ListProfile(nil, nil); e == nil {
			for _, p := range afterProfiles {
				if !existingICCIDs[p.ICCID.String()] {
					_ = client.SetNickname(p.ICCID, nickname)
					break
				}
			}
		}
	}
	return readEuiccState(client)
}
