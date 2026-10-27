// SPDX-License-Identifier: AGPL-3.0-or-later
// Temporary SMS-over-IMS host. Subscriber data and SMS spool stay on device.
package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iniwex5/vowifi-go/runtimehost/messaging"
	"github.com/iniwex5/vowifi-go/runtimehost/voiceclient"
)

const imsSMSDir = "/data/adb/sp970-wfc/sms"

type imsSMSHost struct {
	p                       *protectedIMSRegister
	profile                 voiceclient.IMSProfile
	binding                 voiceclient.RegistrationBinding
	service                 *messaging.Service
	forward                 *smsForwarder
	ctx                     context.Context
	mu                      sync.Mutex
	sendMu                  sync.Mutex
	smsc, own, token        string
	sessions                map[string]time.Time
	seen                    map[string]bool
	registered              bool
	received, sent, reports int
	rpMR                    byte
	started                 time.Time
}

func privateJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	defer clear(raw)
	f, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func smscFromSMSP(record []byte) (string, error) {
	if len(record) < 28 {
		return "", errors.New("short EF_SMSP")
	}
	tail := record[len(record)-28:]
	if tail[0]&2 != 0 {
		return "", errors.New("SIM has no SMSC parameter")
	}
	address := tail[13:25]
	n := int(address[0])
	if n < 2 || n > 11 {
		return "", errors.New("invalid SIM SMSC length")
	}
	var digits strings.Builder
	for _, b := range address[2 : n+1] {
		for _, d := range []byte{b & 15, b >> 4} {
			if d < 10 {
				digits.WriteByte('0' + d)
			} else if d != 15 {
				return "", errors.New("invalid SIM SMSC digit")
			}
		}
	}
	if digits.Len() < 5 {
		return "", errors.New("empty SIM SMSC")
	}
	prefix := ""
	if address[1]&0x70 == 0x10 {
		prefix = "+"
	}
	return prefix + digits.String(), nil
}

func carrierOwnNumber(uris []string) (string, error) {
	match := regexp.MustCompile(`(?i)^(?:tel:|sips?:)(\+[0-9]{8,15})(?:[@;>]|$)`)
	numbers := map[string]bool{}
	for _, uri := range uris {
		m := match.FindStringSubmatch(strings.Trim(strings.TrimSpace(uri), "<>"))
		if len(m) > 1 {
			numbers[m[1]] = true
		}
	}
	if len(numbers) != 1 {
		return "", errors.New("carrier did not confirm one unique own telephone number")
	}
	for n := range numbers {
		return n, nil
	}
	return "", errors.New("no own number")
}

func nativeSMSInsert(ctx context.Context, peer, text string, at time.Time, sent bool) error {
	box := "inbox"
	if sent {
		box = "sent"
	}
	if at.IsZero() {
		at = time.Now()
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	where := "address=" + quote(peer) + " AND body=" + quote(text) + " AND date=" + strconv.FormatInt(at.UnixMilli(), 10)
	query := func() ([]byte, error) {
		return exec.CommandContext(ctx, "/system/bin/sh", "/system/bin/content", "query", "--uri", "content://sms/"+box, "--projection", "_id", "--where", where).CombinedOutput()
	}
	if existing, err := query(); err == nil && strings.Contains(string(existing), "_id=") {
		clear(existing)
		return nil
	}
	args := []string{"insert", "--uri", "content://sms/" + box, "--bind", "address:s:" + peer, "--bind", "body:s:" + text, "--bind", "date:l:" + strconv.FormatInt(at.UnixMilli(), 10), "--bind", "read:i:0"}
	if sent {
		args[len(args)-1] = "read:i:1"
	}
	// Stock KitKat's content launcher is a shell file without a shebang.
	out, err := exec.CommandContext(ctx, "/system/bin/sh", append([]string{"/system/bin/content"}, args...)...).CombinedOutput()
	defer clear(out)
	if err != nil || strings.Contains(string(out), "Error") || strings.Contains(string(out), "Exception") {
		return errors.New("Android SMS provider insertion failed")
	}
	check, err := query()
	defer clear(check)
	if err != nil || !strings.Contains(string(check), "_id=") {
		return errors.New("Android SMS provider did not confirm saved message")
	}
	return nil
}

func sipURI(value string) string {
	if left := strings.IndexByte(value, '<'); left >= 0 {
		if right := strings.IndexByte(value[left+1:], '>'); right >= 0 {
			return value[left+1 : left+1+right]
		}
	}
	return strings.TrimSpace(strings.SplitN(value, ";tag=", 2)[0])
}

func (h *imsSMSHost) receive(ctx context.Context, msg voiceclient.SIPIncomingRequest) (int, string) {
	// Persist every original RPDU, including concatenation parts, before ACK.
	h.mu.Lock()
	defer h.mu.Unlock()
	id := sha256.Sum256([]byte(sipKey(msg.Headers)))
	name := hex.EncodeToString(id[:])
	if h.seen[name] {
		return 200, "OK"
	}
	if err := privateJSON(filepath.Join(imsSMSDir, name+".json"), msg); err != nil {
		event("sms_storage_error", nil)
		return 500, "Server Internal Error"
	}
	result, err := h.service.HandleIMSMessage(ctx, messaging.IMSMessageRequest{FromURI: sipURI(sipHeader(msg.Headers, "From")), ToURI: sipURI(sipHeader(msg.Headers, "To")), CallID: sipHeader(msg.Headers, "Call-ID"), CSeq: sipCSeq(msg.Headers), ContentType: sipHeader(msg.Headers, "Content-Type"), Body: msg.Body, Headers: msg.Headers})
	if err != nil {
		event("sms_parse_error", map[string]any{"status": result.StatusCode})
		return result.StatusCode, "SMS Parsing Failed"
	}
	if result.Incoming != nil {
		in := result.Incoming
		storeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = nativeSMSInsert(storeCtx, in.Sender, in.Content, in.Timestamp, false)
		cancel()
		if err != nil {
			event("sms_native_storage_error", nil)
			return 500, "Server Internal Error"
		}
		h.received++
		event("sms_received", map[string]any{"native_sms_saved": true, "count": h.received, "characters": len([]rune(in.Content))})
		if h.forward != nil {
			if err := h.forward.enqueue(in.Sender, in.Content, in.Timestamp); err != nil {
				event("sms_forward_queue_error", nil)
				// The network retry can recover a durable-queue write failure.
				return 500, "Server Internal Error"
			}
		}
	}
	if result.DeliveryReport != nil {
		h.reports++
		event("sms_delivery_report", map[string]any{"state": result.DeliveryReport.State, "rp_cause": result.DeliveryReport.RPCause, "count": h.reports})
	}
	h.seen[name] = true
	if len(result.ReplyBody) > 0 {
		body := append([]byte(nil), result.ReplyBody...)
		remote := sipURI(sipHeader(msg.Headers, "From"))
		go func() {
			defer clear(body)
			_, err := h.sendRPDU(ctx, remote, body, sipHeader(msg.Headers, "Call-ID"))
			if err != nil {
				event("sms_rp_ack_error", map[string]any{"error": err.Error()})
			} else {
				event("sms_rp_ack_sent", nil)
			}
		}()
	}
	return 200, "OK"
}

func (h *imsSMSHost) sendRPDU(ctx context.Context, target string, body []byte, inReplyTo ...string) (int, error) {
	var id [16]byte
	_, err := rand.Read(id[:])
	if err != nil {
		return 0, err
	}
	msg, err := voiceclient.BuildMessageRequest(voiceclient.DialogRequestConfig{Profile: h.profile, Registration: h.binding, LocalURI: h.binding.PublicIdentity, RemoteURI: target, RemoteTargetURI: target, CallID: hex.EncodeToString(id[:]) + "@sp970-wfc", LocalTag: "sms", CSeq: 1, UserAgent: h.profile.UserAgent}, messaging.IMS3GPPSMSContentType, body)
	if err != nil {
		return 0, err
	}
	msg.Headers["P-Access-Network-Info"] = h.p.headers["P-Access-Network-Info"]
	msg.Headers["Accept-Contact"] = "*;+g.3gpp.smsip;require;explicit"
	if len(inReplyTo) == 1 && inReplyTo[0] != "" {
		msg.Headers["In-Reply-To"] = inReplyTo[0]
	}
	resp, err := h.p.flow.RoundTripRequest(ctx, msg)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("IMS SMS rejected: %d %s", resp.StatusCode, resp.Reason)
	}
	return resp.StatusCode, nil
}

func (h *imsSMSHost) send(ctx context.Context, to, text string) (int, error) {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	if !regexp.MustCompile(`^\+?[0-9]{3,15}$`).MatchString(to) {
		return 0, errors.New("invalid SMS recipient")
	}
	parts := messaging.SegmentSMS(text, "")
	if len(parts) < 1 || len(parts) > 6 {
		return 0, errors.New("SMS must contain between one and six parts")
	}
	if h.smsc == "" {
		return 0, errors.New("SIM SMSC is unavailable")
	}
	code := 0
	for _, part := range parts {
		h.rpMR++
		tpdu, err := messaging.BuildSMSSubmitTPDU(to, part, h.rpMR)
		if err != nil {
			return code, err
		}
		rpdu, err := messaging.BuildSMSRPData(h.rpMR, h.smsc, tpdu)
		clear(tpdu)
		if err != nil {
			return code, err
		}
		code, err = h.sendRPDU(ctx, "sip:"+h.smsc+"@"+h.profile.Domain+";user=phone", rpdu)
		clear(rpdu)
		if err != nil {
			return code, err
		}
	}
	h.mu.Lock()
	h.sent++
	count := h.sent
	h.mu.Unlock()
	storeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := nativeSMSInsert(storeCtx, to, text, time.Now(), true); err != nil {
		event("sms_sent_storage_error", nil)
	}
	event("sms_submitted", map[string]any{"sip_status": code, "parts": len(parts), "count": count})
	return code, nil
}

func (h *imsSMSHost) authorized(r *http.Request) bool {
	value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if value != "" && subtle.ConstantTimeCompare([]byte(value), []byte(h.token)) == 1 {
		return true
	}
	return sharedUIAuthorized(value)
}

func nativeUIAuthenticate(ctx context.Context, account, password string) bool {
	if len(account) > 128 || len(password) > 256 {
		return false
	}
	out, err := exec.CommandContext(ctx, "/system/xbin/sqlite3", "/data/data/com.pingbao/databases/ufi_router.db", "SELECT account,password FROM users;").Output()
	defer clear(out)
	if err != nil {
		return false
	}
	digest := md5.Sum([]byte(password))
	hash := hex.EncodeToString(digest[:])
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && subtle.ConstantTimeCompare([]byte(parts[0]), []byte(account)) == 1 && subtle.ConstantTimeCompare([]byte(strings.ToLower(parts[1])), []byte(hash)) == 1 {
			return true
		}
	}
	return false
}

func recoverIMSSMS(ctx context.Context) error {
	files, err := filepath.Glob(filepath.Join(imsSMSDir, "*.json"))
	if err != nil {
		return err
	}
	service := messaging.NewService("sp970b-slot1", "", nil, nil)
	count := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var msg voiceclient.SIPIncomingRequest
		err = json.Unmarshal(raw, &msg)
		clear(raw)
		if err != nil {
			return errors.New("invalid saved IMS SMS")
		}
		result, err := service.HandleIMSMessage(ctx, messaging.IMSMessageRequest{FromURI: sipURI(sipHeader(msg.Headers, "From")), ToURI: sipURI(sipHeader(msg.Headers, "To")), CallID: sipHeader(msg.Headers, "Call-ID"), CSeq: sipCSeq(msg.Headers), ContentType: sipHeader(msg.Headers, "Content-Type"), Body: msg.Body, Headers: msg.Headers})
		clear(msg.Body)
		if err != nil {
			return errors.New("saved IMS SMS could not be parsed")
		}
		if result.Incoming != nil {
			in := result.Incoming
			if err := nativeSMSInsert(ctx, in.Sender, in.Content, in.Timestamp, false); err != nil {
				return err
			}
			count++
		}
	}
	event("sms_recovered", map[string]any{"native_sms_saved": true, "records_processed": count})
	return nil
}

func (h *imsSMSHost) api(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	if r.URL.Path == "/login" && r.Method == "POST" {
		uiLogin(w, r)
		return
	}
	if !h.authorized(r) {
		http.Error(w, `{"error":"login required"}`, 401)
		return
	}
	switch {
	case r.URL.Path == "/forward/preview" && r.Method == "POST":
		if h.forward == nil {
			http.Error(w, `{"error":"转发服务未就绪"}`, 503)
			return
		}
		var config smsForwardConfig
		if json.NewDecoder(r.Body).Decode(&config) != nil {
			http.Error(w, `{"error":"转发参数无效"}`, 400)
			return
		}
		preview, err := h.forward.preview(config)
		if err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(preview)
	case r.URL.Path == "/forward/check" && r.Method == "POST":
		if h.forward == nil {
			http.Error(w, `{"error":"转发服务未就绪"}`, 503)
			return
		}
		if err := h.forward.check(r.Context()); err != nil {
			w.WriteHeader(502)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"connected": true, "sms_sent": false})
	case r.URL.Path == "/forward" && r.Method == "GET":
		if h.forward == nil {
			http.Error(w, `{"error":"转发服务未就绪"}`, 503)
			return
		}
		_ = json.NewEncoder(w).Encode(h.forward.view())
	case r.URL.Path == "/forward" && r.Method == "POST":
		if h.forward == nil {
			http.Error(w, `{"error":"转发服务未就绪"}`, 503)
			return
		}
		var config smsForwardConfig
		if json.NewDecoder(r.Body).Decode(&config) != nil {
			http.Error(w, `{"error":"转发参数无效"}`, 400)
			return
		}
		if err := h.forward.save(config); err != nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(h.forward.view())
	case r.URL.Path == "/status" && r.Method == "GET":
		h.mu.Lock()
		defer h.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"registered": h.registered, "imsi_prefix": "23487", "own_number_confirmed": h.own != "", "received": h.received, "submitted": h.sent, "reports": h.reports, "started": h.started, "expires_seconds": h.binding.Expires})
	case r.URL.Path == "/send" && r.Method == "POST":
		var request struct{ To, Text string }
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, `{"error":"invalid SMS"}`, 400)
			return
		}
		code, err := h.send(r.Context(), strings.TrimSpace(request.To), request.Text)
		if err != nil {
			w.WriteHeader(502)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "sip_status": code})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"submitted": true, "sip_status": code})
	default:
		http.NotFound(w, r)
	}
}

func keepIMSSMS(ctx context.Context, p *protectedIMSRegister, profile voiceclient.IMSProfile, result voiceclient.RegisterResult, duration time.Duration, selfTest bool) error {
	if err := os.MkdirAll(imsSMSDir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(imsSMSDir, 0700); err != nil {
		return err
	}
	h := &imsSMSHost{p: p, profile: profile, binding: result.Binding, ctx: ctx, service: messaging.NewService("sp970b-slot1", p.aka.base.imsi, nil, nil), sessions: map[string]time.Time{}, seen: map[string]bool{}, registered: true, started: time.Now()}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	h.token = hex.EncodeToString(b[:])
	if existing, err := os.ReadFile("/data/adb/sp970-wfc/sms-api-token"); err == nil {
		if decoded, e := hex.DecodeString(string(existing)); e == nil && len(decoded) == 32 {
			h.token = string(existing)
			clear(decoded)
		}
		clear(existing)
	}
	if err := os.WriteFile("/data/adb/sp970-wfc/sms-api-token", []byte(h.token), 0600); err != nil {
		return err
	}
	if raw, err := os.ReadFile("/data/adb/sp970-wfc/ui-sessions.json"); err == nil {
		_ = json.Unmarshal(raw, &h.sessions)
		clear(raw)
		if h.sessions == nil {
			h.sessions = map[string]time.Time{}
		}
		for token, until := range h.sessions {
			if time.Now().After(until) {
				delete(h.sessions, token)
			}
		}
	}
	if fields, err := bridge(ctx, "smsp", ""); err == nil && len(fields) == 1 {
		if raw, err := hex.DecodeString(fields[0]); err == nil {
			h.smsc, _ = smscFromSMSP(raw)
			clear(raw)
		}
	}
	h.own, _ = carrierOwnNumber(result.Binding.AssociatedURIs)
	forward, err := newSMSForwarder("/data/adb/sp970-wfc/forward")
	if err != nil {
		return err
	}
	h.forward = forward
	forwardCtx, stopForward := context.WithCancel(ctx)
	defer stopForward()
	go h.forward.run(forwardCtx)
	p.sms.Store(h)
	defer p.sms.Store(nil)
	listener, err := net.Listen("tcp4", "127.0.0.1:17892")
	if err != nil {
		return err
	}
	server := &http.Server{Handler: http.HandlerFunc(h.api), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	event("wfc_sms_online", map[string]any{"registered": true, "api": "127.0.0.1:17892", "own_number_confirmed": h.own != "", "sim_smsc_available": h.smsc != "", "temporary_session_minutes": duration.Minutes()})
	if selfTest {
		if h.own == "" {
			event("sms_self_test_blocked", map[string]any{"reason": "no carrier-confirmed own number"})
		} else {
			testText := "SP970 WFC test " + time.Now().UTC().Format("20060102T150405Z")
			go func() {
				code, err := h.send(ctx, h.own, testText)
				if err != nil {
					event("sms_self_test_error", map[string]any{"sip_status": code, "error": err.Error()})
				} else {
					event("sms_self_test_submitted", map[string]any{"sip_status": code})
				}
			}()
		}
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.flow.ctx.Done():
			return errors.New("registered IMS flow disconnected")
		case <-deadline.C:
			return nil
		case <-ticker.C:
			if err := p.flow.SendCRLFKeepalive(ctx); err != nil {
				return err
			}
		}
	}
}
