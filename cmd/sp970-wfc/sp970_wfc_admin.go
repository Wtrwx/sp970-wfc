// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var adminMode = flag.Bool("admin", false, "serve device management on loopback, independently of IMS")

const adminBase = "/data/adb/sp970-wfc"
const sessionFile = adminBase + "/ui-sessions.json"

// A file lock and fresh reads share native-login sessions across IMS restarts
// and the independent manager. No credentials enter argv or the access log.
func readUISessions() map[string]time.Time {
	sessions := map[string]time.Time{}
	raw, err := os.ReadFile(sessionFile)
	if err == nil {
		_ = json.Unmarshal(raw, &sessions)
		clear(raw)
	}
	if sessions == nil {
		sessions = map[string]time.Time{}
	}
	return sessions
}
func sharedUIAuthorized(token string) bool {
	if len(token) != 64 {
		return false
	}
	until, ok := readUISessions()[token]
	return ok && time.Now().Before(until)
}
func saveUISession(token string) error {
	return updateUISession(token, false)
}
func updateUISession(token string, remove bool) error {
	lock, err := os.OpenFile(sessionFile+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	sessions := readUISessions()
	for k, v := range sessions {
		if time.Now().After(v) {
			delete(sessions, k)
		}
	}
	if remove {
		delete(sessions, token)
	} else {
		sessions[token] = time.Now().Add(8 * time.Hour)
	}
	return privateJSON(sessionFile, sessions)
}
func uiLogin(w http.ResponseWriter, r *http.Request) {
	var credentials struct{ Account, Password string }
	if json.NewDecoder(r.Body).Decode(&credentials) != nil || !nativeUIAuthenticate(r.Context(), credentials.Account, credentials.Password) {
		http.Error(w, `{"error":"账号或密码错误"}`, 401)
		return
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		http.Error(w, `{}`, 500)
		return
	}
	token := hex.EncodeToString(b[:])
	if saveUISession(token) != nil {
		http.Error(w, `{"error":"无法保存登录会话"}`, 500)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func androidCall(ctx context.Context, request any) (map[string]any, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/system/bin/app_process", "/system/bin", "Sp970Android")
	cmd.Env = append(os.Environ(), "CLASSPATH="+adminBase+"/bin/sp970_android.dex")
	cmd.Stdin = bytes.NewReader(append(raw, '\n'))
	out, runErr := cmd.Output()
	defer clear(out)
	var response map[string]any
	if json.Unmarshal(bytes.TrimSpace(out), &response) != nil {
		return nil, errors.New("Android 设置服务未就绪")
	}
	if message, ok := response["error"].(string); ok {
		return nil, errors.New(message)
	}
	if runErr != nil {
		return nil, errors.New("Android 设置操作失败")
	}
	return response, nil
}
func requireFlight(ctx context.Context) error {
	// Stock KitKat settings is a shell script without a shebang.
	raw, err := exec.CommandContext(ctx, "/system/bin/sh", "/system/bin/settings", "get", "global", "airplane_mode_on").Output()
	if err != nil || strings.TrimSpace(string(raw)) != "1" {
		return errors.New("请先开启飞行模式；操作已停止")
	}
	return nil
}
func readSetting(name string) string {
	raw, _ := os.ReadFile(filepath.Join(adminBase, name))
	return strings.TrimSpace(string(raw))
}
func wfcEnabled() bool { _, err := os.Stat(adminBase + "/disabled"); return os.IsNotExist(err) }
func serviceControl(ctx context.Context, action string) error {
	// All callers use literal actions. Never execute a config-supplied command.
	_, err := exec.CommandContext(ctx, "/system/bin/sh", adminBase+"/service-control.sh", action).Output()
	if err != nil {
		return errors.New("WFC 服务操作失败")
	}
	return nil
}
func waitWFCStopped(ctx context.Context) error {
	for i := 0; i < 25; i++ {
		active := false
		for _, role := range []string{"supervisor", "wfc", "proxy"} {
			pid, _ := strconv.Atoi(readSetting("run/" + role + ".pid"))
			if pid > 1 && syscall.Kill(pid, 0) == nil {
				active = true
			}
		}
		if !active {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("WFC 尚未停止，请稍后重试")
}
func enableWFC(ctx context.Context) error {
	if err := requireFlight(ctx); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, adminBase+"/bin/sp970-wfc", "--check-sim")
	out, err := cmd.Output()
	defer clear(out)
	if err != nil || !bytes.Contains(out, []byte(`"imsi_prefix":"23487"`)) {
		return errors.New("SIM 策略未通过 23487 检查，WFC 保持关闭")
	}
	return serviceControl(ctx, "enable")
}

type networkSettings struct {
	Mode    string `json:"mode"`
	URI     string `json:"uri"`
	EPDG    string `json:"epdg"`
	WFCOnly bool   `json:"wfc_only"`
}

func parseProxyURI(ctx context.Context, uri string) (map[string]any, string, error) {
	invalid := errors.New("Hysteria2 链接无效")
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil || u.User == nil || u.Hostname() == "" || (u.Scheme != "hysteria2" && u.Scheme != "hy2") {
		return nil, "", invalid
	}
	if _, has := u.User.Password(); has {
		return nil, "", errors.New("密码中的冒号需要 URL 编码")
	}
	auth := u.User.Username()
	if auth == "" || len(auth) > 4096 {
		return nil, "", invalid
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, "", invalid
	}
	for k, v := range q {
		if len(v) != 1 {
			return nil, "", invalid
		}
		switch k {
		case "sni", "insecure", "pinSHA256", "obfs", "obfs-password":
		default:
			return nil, "", fmt.Errorf("不支持的节点参数：%s", k)
		}
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return nil, "", invalid
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		resolver := deviceResolver()
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		addresses, e := resolver.LookupIP(ctx, "ip4", host)
		if e != nil || len(addresses) == 0 {
			return nil, "", errors.New("无法解析节点 IPv4，请使用 IPv4 节点链接")
		}
		ip = addresses[0]
	}
	if ip.To4() == nil {
		return nil, "", errors.New("此设备的节点需提供 IPv4")
	}
	address := ip.To4().String()
	pin := strings.ToLower(strings.ReplaceAll(q.Get("pinSHA256"), ":", ""))
	if pin != "" {
		if raw, e := hex.DecodeString(pin); e != nil || len(raw) != 32 {
			return nil, "", errors.New("证书指纹需为 64 位 SHA256")
		}
	}
	insecure := q.Get("insecure") == "1" || q.Get("insecure") == "true"
	if insecure && pin == "" {
		return nil, "", errors.New("跳过 CA 验证时必须提供 pinSHA256")
	}
	sni := q.Get("sni")
	if sni == "" {
		sni = host
	}
	tls := map[string]any{"sni": sni, "insecure": insecure}
	if pin != "" {
		tls["pinSHA256"] = pin
	}
	config := map[string]any{"server": net.JoinHostPort(address, port), "auth": auth, "tls": tls, "fastOpen": false,
		"socks5": map[string]any{"listen": "127.0.0.1:17890"}, "http": map[string]any{"listen": "127.0.0.1:17891"},
		"tun": map[string]any{"name": "hypsm0", "mtu": 1500, "address": map[string]any{"ipv4": "100.100.100.101/30", "ipv6": "fd44:5350:970::1/126"}}}
	if q.Get("obfs") != "" {
		if q.Get("obfs") != "salamander" || q.Get("obfs-password") == "" {
			return nil, "", errors.New("不支持该混淆参数")
		}
		config["obfs"] = map[string]any{"type": "salamander", "salamander": map[string]any{"password": q.Get("obfs-password")}}
	}
	return config, address, nil
}
func proxySummary() map[string]any {
	rawHy2, _ := os.ReadFile(adminBase + "/psm-direct-tun.json")
	defer clear(rawHy2)
	var hy2Node map[string]any
	_ = json.Unmarshal(rawHy2, &hy2Node)

	rawClient, _ := os.ReadFile(adminBase + "/proxy-client.json")
	defer clear(rawClient)
	var clientCfg proxyClientConfig
	_ = json.Unmarshal(rawClient, &clientCfg)

	rawMode := readSetting("network-mode")
	if rawMode == "" {
		rawMode = "hysteria2"
	}
	epdg := readSetting("epdg-ipv4")
	if epdg == "" {
		epdg = "84.246.115.151"
	}
	uri := readSetting("proxy-uri")
	if uri == "" {
		uri = reconstructHy2URI(hy2Node)
	}
	protocol := ""
	var server any
	switch rawMode {
	case "socks5", "ss":
		protocol = rawMode
		server = clientCfg.Server
	case "hysteria2":
		protocol = "hysteria2"
		server = hy2Node["server"]
	default:
		if scheme, _, ok := strings.Cut(uri, "://"); ok {
			switch strings.ToLower(scheme) {
			case "hy2", "hysteria2":
				protocol = "hysteria2"
				server = hy2Node["server"]
			case "socks5", "socks5h", "socks":
				protocol = "socks5"
				server = clientCfg.Server
			case "ss":
				protocol = "ss"
				server = clientCfg.Server
			}
		}
	}
	uiMode := "proxy"
	if rawMode == "direct" {
		uiMode = "direct"
	}
	return map[string]any{
		"mode":       uiMode,
		"raw_mode":   rawMode,
		"protocol":   protocol,
		"uri":        uri,
		"epdg":       epdg,
		"server":     server,
		"configured": uri != "",
		"wfc_only":   (protocol == "ss" || protocol == "socks5") && clientCfg.WFCOnly,
	}
}
func saveNetwork(ctx context.Context, request networkSettings) error {
	if err := requireFlight(ctx); err != nil {
		return err
	}
	switch request.Mode {
	case "direct", "proxy", "hysteria2", "socks5", "ss":
	default:
		return errors.New("请选择直连或代理")
	}
	epdg := strings.TrimSpace(request.EPDG)
	if epdg == "" {
		epdg = readSetting("epdg-ipv4")
		if epdg == "" {
			epdg = "84.246.115.151"
		}
	}
	if epdg != "84.246.115.151" && epdg != "84.246.113.151" {
		return errors.New("ePDG 地址无效")
	}
	uri := strings.TrimSpace(request.URI)
	values := map[string][]byte{"epdg-ipv4": []byte(epdg + "\n")}
	if request.Mode == "direct" {
		values["network-mode"] = []byte("direct\n")
		if uri != "" {
			parsed, err := parseAnyProxyURI(ctx, uri)
			if err != nil {
				return err
			}
			if request.WFCOnly && parsed.Protocol == "hysteria2" {
				return errors.New("仅代理 WFC 目前支持 SOCKS5 / Shadowsocks")
			}
			parsed.ClientConfig.WFCOnly = request.WFCOnly
			values["proxy-uri"] = []byte(parsed.CanonicalURI + "\n")
			values["proxy-ipv4"] = []byte(parsed.ServerIP + "\n")
			if parsed.Protocol == "hysteria2" {
				raw, _ := json.Marshal(parsed.Hy2Config)
				defer clear(raw)
				values["psm-direct-tun.json"] = raw
			} else {
				raw, _ := json.Marshal(parsed.ClientConfig)
				defer clear(raw)
				values["proxy-client.json"] = raw
			}
		}
	} else {
		if uri == "" {
			uri = readSetting("proxy-uri")
			if uri == "" {
				rawHy2, _ := os.ReadFile(adminBase + "/psm-direct-tun.json")
				var hy2Node map[string]any
				_ = json.Unmarshal(rawHy2, &hy2Node)
				clear(rawHy2)
				uri = reconstructHy2URI(hy2Node)
			}
		}
		if uri == "" {
			return errors.New("请填写代理节点链接（支持 socks5://、hy2://、ss://）")
		}
		parsed, err := parseAnyProxyURI(ctx, uri)
		if err != nil {
			return err
		}
		if request.WFCOnly && parsed.Protocol == "hysteria2" {
			return errors.New("仅代理 WFC 目前支持 SOCKS5 / Shadowsocks")
		}
		parsed.ClientConfig.WFCOnly = request.WFCOnly
		values["network-mode"] = []byte(parsed.Protocol + "\n")
		values["proxy-uri"] = []byte(parsed.CanonicalURI + "\n")
		values["proxy-ipv4"] = []byte(parsed.ServerIP + "\n")
		if parsed.Protocol == "hysteria2" {
			raw, _ := json.Marshal(parsed.Hy2Config)
			defer clear(raw)
			values["psm-direct-tun.json"] = raw
		} else {
			raw, _ := json.Marshal(parsed.ClientConfig)
			defer clear(raw)
			values["proxy-client.json"] = raw
		}
	}
	previous := map[string][]byte{}
	for name := range values {
		raw, _ := os.ReadFile(adminBase + "/" + name)
		previous[name] = raw
		defer clear(raw)
	}
	enabled := wfcEnabled()
	if err := serviceControl(ctx, "disable"); err != nil {
		return err
	}
	if err := waitWFCStopped(ctx); err != nil {
		if enabled {
			_ = serviceControl(ctx, "enable")
		}
		return err
	}
	var saveErr error
	for name, value := range values {
		path := adminBase + "/" + name
		if e := os.WriteFile(path+".next", value, 0600); e != nil {
			saveErr = e
			break
		}
		if e := os.Rename(path+".next", path); e != nil {
			saveErr = e
			break
		}
	}
	if saveErr != nil {
		for name, value := range previous {
			if value == nil {
				_ = os.Remove(adminBase + "/" + name)
			} else {
				_ = os.WriteFile(adminBase+"/"+name, value, 0600)
			}
		}
	}
	if enabled {
		if err := enableWFC(ctx); err != nil {
			return err
		}
	}
	if saveErr != nil {
		return errors.New("代理配置保存失败，已恢复旧配置")
	}
	return nil
}

type adminHost struct{ mutations sync.Mutex }

func (h *adminHost) api(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if r.Method == "POST" {
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				http.Error(w, `{"error":"来源不允许"}`, 403)
				return
			}
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, `{"error":"需要 JSON 请求"}`, 415)
			return
		}
	}
	if r.URL.Path == "/login" && r.Method == "POST" {
		uiLogin(w, r)
		return
	}
	if !sharedUIAuthorized(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
		http.Error(w, `{"error":"请登录后台"}`, 401)
		return
	}
	timeout := 75 * time.Second
	if r.URL.Path == "/euicc/download" {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), timeout)
	defer cancel()
	var response any
	var err error
	if r.Method == "GET" {
		switch r.URL.Path {
		case "/status":
			state, e := androidCall(ctx, map[string]string{"action": "status"})
			if e != nil {
				err = e
				break
			}
			state["wfc_enabled"] = wfcEnabled()
			state["service_state"] = readSetting("run/state")
			state["proxy"] = proxySummary()
			state["tailscale"] = tailscaleSummary(ctx)
			state["ssh"] = sshSummary()
			response = state
		case "/wifi/scan":
			response, err = androidCall(ctx, map[string]string{"action": "scan"})
		case "/euicc":
			h.mutations.Lock()
			response, err = euiccProfiles(ctx)
			h.mutations.Unlock()
		case "/forward":
			var forward *smsForwarder
			forward, err = newSMSForwarder(adminBase + "/forward")
			if err == nil {
				response = forward.view()
				forward.client.CloseIdleConnections()
			}
		case "/sms":
			response, err = listNativeSMS(ctx, r.URL.Query().Get("box"), r.URL.Query().Get("archived") == "1")
		default:
			http.NotFound(w, r)
			return
		}
	} else if r.Method == "POST" {
		h.mutations.Lock()
		defer h.mutations.Unlock()
		switch r.URL.Path {
		case "/forward":
			var config smsForwardConfig
			if json.NewDecoder(r.Body).Decode(&config) != nil {
				err = errors.New("转发参数无效")
				break
			}
			var forward *smsForwarder
			forward, err = newSMSForwarder(adminBase + "/forward")
			if err == nil {
				err = forward.save(config)
				if err == nil {
					response = forward.view()
				}
				forward.client.CloseIdleConnections()
			}
		case "/logout":
			err = updateUISession(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), true)
		case "/sms/read", "/sms/archive", "/sms/delete":
			var v struct {
				ID       int64 `json:"id"`
				Archived bool  `json:"archived"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.ID < 1 {
				err = errors.New("短信标识无效")
				break
			}
			if r.URL.Path == "/sms/read" {
				err = markNativeSMSRead(ctx, v.ID)
			} else if r.URL.Path == "/sms/delete" {
				err = deleteNativeSMS(ctx, v.ID)
			} else {
				err = archiveNativeSMS(v.ID, v.Archived)
			}
		case "/wfc":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			if *v.Enabled {
				err = enableWFC(ctx)
			} else {
				err = serviceControl(ctx, "disable")
				if err == nil {
					err = waitWFCStopped(ctx)
				}
			}
		case "/network":
			var v networkSettings
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("代理参数无效")
				break
			}
			err = saveNetwork(ctx, v)
		case "/flight":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			// Pause the runtime first, retaining the user's WFC autostart flag.
			if !*v.Enabled {
				err = serviceControl(ctx, "pause")
				if err == nil {
					err = waitWFCStopped(ctx)
				}
				if err != nil {
					break
				}
			}
			response, err = androidCall(ctx, map[string]any{"action": "flight", "enabled": *v.Enabled})
			if wfcEnabled() {
				_ = serviceControl(ctx, "start")
			}
		case "/wifi":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			response, err = androidCall(ctx, map[string]any{"action": "wifi", "enabled": *v.Enabled})
		case "/wifi/disconnect":
			response, err = androidCall(ctx, map[string]any{"action": "wifi_disconnect"})
		case "/wifi/forget":
			var v struct {
				ID   *int   `json:"network_id"`
				SSID string `json:"ssid"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || ((v.ID == nil || *v.ID < 0) && strings.TrimSpace(v.SSID) == "") {
				err = errors.New("Wi-Fi 参数无效")
				break
			}
			req := map[string]any{"action": "wifi_forget", "ssid": v.SSID}
			if v.ID != nil {
				req["network_id"] = *v.ID
			}
			response, err = androidCall(ctx, req)
		case "/wifi/connect":
			var v struct {
				SSID     string `json:"ssid"`
				Password string `json:"password"`
				Security string `json:"security"`
				ID       *int   `json:"network_id"`
				Saved    bool   `json:"saved"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("Wi-Fi 参数无效")
				break
			}
			request := map[string]any{"action": "connect", "ssid": v.SSID, "password": v.Password, "security": v.Security, "saved": v.Saved}
			if v.ID != nil {
				request["network_id"] = *v.ID
			}
			response, err = androidCall(ctx, request)
		case "/hotspot":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			response, err = androidCall(ctx, map[string]any{"action": "hotspot", "enabled": *v.Enabled})
		case "/hotspot/config":
			var v struct {
				SSID      *string `json:"ssid"`
				Security  *string `json:"security"`
				Password  string  `json:"password"`
				Broadcast *bool   `json:"broadcast"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || (v.SSID == nil && v.Security == nil && v.Broadcast == nil) {
				err = errors.New("热点参数无效")
				break
			}
			request := map[string]any{"action": "hotspot_config"}
			if v.SSID != nil {
				request["ssid"] = *v.SSID
			}
			if v.Security != nil {
				request["security"] = *v.Security
				request["password"] = v.Password
			}
			if v.Broadcast != nil {
				request["broadcast"] = *v.Broadcast
			}
			response, err = androidCall(ctx, request)
		case "/usb-tether", "/mobile-data", "/data-roaming":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			actionName := "usb_tether"
			if r.URL.Path == "/mobile-data" {
				actionName = "mobile_data"
			} else if r.URL.Path == "/data-roaming" {
				actionName = "data_roaming"
			}
			response, err = androidCall(ctx, map[string]any{"action": actionName, "enabled": *v.Enabled})
		case "/euicc/enable":
			var v struct {
				ID string `json:"id"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("配置参数无效")
				break
			}
			response, err = switchEuicc(ctx, v.ID)
		case "/euicc/disable":
			var v struct {
				ID string `json:"id"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("配置参数无效")
				break
			}
			response, err = disableEuicc(ctx, v.ID)
		case "/euicc/nickname":
			var v struct {
				ID       string `json:"id"`
				ICCID    string `json:"iccid"`
				Nickname string `json:"nickname"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || (strings.TrimSpace(v.ID) == "" && strings.TrimSpace(v.ICCID) == "") {
				err = errors.New("配置参数无效")
				break
			}
			response, err = renameEuicc(ctx, v.ID, v.ICCID, v.Nickname)
		case "/euicc/delete":
			var v struct {
				ID    string `json:"id"`
				ICCID string `json:"iccid"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || (strings.TrimSpace(v.ID) == "" && strings.TrimSpace(v.ICCID) == "") {
				err = errors.New("配置参数无效")
				break
			}
			response, err = deleteEuicc(ctx, v.ID, v.ICCID)
		case "/euicc/download":
			var v euiccDownloadRequest
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("激活码参数无效")
				break
			}
			response, err = downloadEuicc(ctx, v)
		case "/ssh":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			response, err = setSSHEnabled(ctx, *v.Enabled)
		case "/tailscale":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			response, err = setTailscaleEnabled(ctx, *v.Enabled)
		case "/tailscale/ssh":
			var v struct {
				Enabled *bool `json:"enabled"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil || v.Enabled == nil {
				err = errors.New("开关参数无效")
				break
			}
			response, err = setTailscaleSSH(ctx, *v.Enabled)
		case "/tailscale/login":
			var v struct {
				AuthKey string `json:"auth_key"`
			}
			if json.NewDecoder(r.Body).Decode(&v) != nil {
				err = errors.New("登录参数无效")
				break
			}
			response, err = loginTailscale(ctx, v.AuthKey)
		case "/tailscale/logout":
			response, err = logoutTailscale(ctx)
		default:
			http.NotFound(w, r)
			return
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if response == nil {
		response = map[string]bool{"ok": true}
	}
	_ = json.NewEncoder(w).Encode(response)
}
func serveAdmin(ctx context.Context) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:17893")
	if err != nil {
		return err
	}
	if err = os.WriteFile(adminBase+"/run/admin.pid", []byte(strconv.Itoa(os.Getpid())+"\n"), 0600); err != nil {
		listener.Close()
		return err
	}
	defer os.Remove(adminBase + "/run/admin.pid")
	server := &http.Server{Handler: http.HandlerFunc((&adminHost{}).api), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 200 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go func() {
		<-ctx.Done()
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(closeCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
