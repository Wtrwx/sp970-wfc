// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const defaultSMSForm = "from={{sms-from}}\ntime={{sms-time}}\nbody={{sms-body-escaped}}\nufi_escaped=1"
const smsSecretMask = "****************"

var smsVariable = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var smsSecretName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var smsHeaderName = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

func normalizeSMSRequest(c smsForwardConfig) smsForwardConfig {
	if c.Method == "" {
		c.Method = "POST"
	}
	c.Method = strings.ToUpper(c.Method)
	if c.Format == "" {
		c.Format = "form"
	}
	if c.Body == "" && c.Format == "form" {
		c.Body = defaultSMSForm
	}
	if c.Success == "" {
		c.Success = "accepted"
	}
	if c.Timeout == 0 {
		c.Timeout = 15
	}
	return c
}

func sensitiveSMSHeader(c smsForwardConfig, name string) bool {
	lower := strings.ToLower(name)
	for _, part := range []string{"authorization", "cookie", "token", "secret", "api-key", "apikey"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	for _, secret := range c.SecretHeaders {
		if strings.EqualFold(name, secret) {
			return true
		}
	}
	return false
}

func maskedSMSHeaders(c smsForwardConfig) map[string]string {
	values := make(map[string]string, len(c.Headers))
	for k, v := range c.Headers {
		if sensitiveSMSHeader(c, k) {
			v = smsSecretMask
		}
		values[k] = v
	}
	return values
}

func maskedSMSSecrets(secrets map[string]string) map[string]string {
	values := make(map[string]string, len(secrets))
	for k := range secrets {
		values[k] = smsSecretMask
	}
	return values
}

func restoreSMSURL(value, old string) string {
	if value == maskedSMSForwardEndpoint(old) {
		return old
	}
	u, err := url.Parse(value)
	previous, oldErr := url.Parse(old)
	if err != nil || oldErr != nil || !strings.EqualFold(u.Host, previous.Host) || u.Scheme != previous.Scheme {
		return value
	}
	pairs, oldQ, indices := strings.Split(u.RawQuery, "&"), previous.Query(), map[string]int{}
	for i, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		name, _ := url.QueryUnescape(key)
		decoded, _ := url.QueryUnescape(value)
		index := indices[name]
		indices[name]++
		if decoded == "********" && index < len(oldQ[name]) {
			pairs[i] = key + "=" + url.QueryEscape(oldQ[name][index])
		}
	}
	u.RawQuery = strings.Join(pairs, "&")
	return u.String()
}

func preserveSMSSecrets(c, old smsForwardConfig) smsForwardConfig {
	// Older clients omit request-editor fields; their saves must preserve them.
	if c.Method == "" {
		c.Method, c.Format, c.Body = old.Method, old.Format, old.Body
		c.Headers, c.SecretHeaders, c.Secrets = old.Headers, old.SecretHeaders, old.Secrets
		c.Success, c.SuccessPath, c.SuccessValue = old.Success, old.SuccessPath, old.SuccessValue
		c.CheckEndpoint, c.Timeout = old.CheckEndpoint, old.Timeout
	}
	if c.ClearAuth {
		c.Authorization = ""
	} else if c.Authorization == "" || c.Authorization == maskedSMSAuthorization(old.Authorization) {
		c.Authorization = old.Authorization
	}
	c.ClearAuth = false
	c.Endpoint = restoreSMSURL(c.Endpoint, old.Endpoint)
	c.CheckEndpoint = restoreSMSURL(c.CheckEndpoint, old.CheckEndpoint)
	if c.Headers != nil {
		headers := make(map[string]string, len(c.Headers))
		for k, v := range c.Headers {
			if v == smsSecretMask {
				for oldK, oldV := range old.Headers {
					if strings.EqualFold(k, oldK) {
						v = oldV
						break
					}
				}
			}
			headers[k] = v
		}
		c.Headers = headers
	}
	if c.Secrets != nil {
		secrets := make(map[string]string, len(c.Secrets))
		for k, v := range c.Secrets {
			if v == smsSecretMask {
				v = old.Secrets[k]
			}
			secrets[k] = v
		}
		c.Secrets = secrets
	}
	return normalizeSMSRequest(c)
}

func smsTemplateValue(name string, c smsForwardConfig, j smsForwardJob) (string, bool) {
	switch name {
	case "sms-from":
		return j.From, true
	case "sms-body":
		return j.Text, true
	case "sms-time":
		return j.At.UTC().Format(time.RFC3339Nano), true
	case "sms-id":
		return j.ID, true
	case "sms-body-escaped":
		b, _ := json.Marshal(j.Text)
		return string(b[1 : len(b)-1]), true
	}
	if strings.HasPrefix(name, "secret:") {
		value, ok := c.Secrets[strings.TrimPrefix(name, "secret:")]
		return value, ok && value != ""
	}
	return "", false
}

func renderSMSTemplate(template string, c smsForwardConfig, j smsForwardJob, encoding string) (string, error) {
	remaining := smsVariable.ReplaceAllString(template, "")
	if strings.Contains(remaining, "{{") || strings.Contains(remaining, "}}") {
		return "", errors.New("模板括号不完整")
	}
	var output strings.Builder
	position, query := 0, strings.Index(template, "?")
	for _, match := range smsVariable.FindAllStringSubmatchIndex(template, -1) {
		name := strings.TrimSpace(template[match[2]:match[3]])
		value, ok := smsTemplateValue(name, c, j)
		if !ok {
			return "", errors.New("模板含未知变量或未配置的密钥变量")
		}
		if encoding == "url" {
			if query < 0 || match[0] < query {
				value = url.PathEscape(value)
			} else {
				value = url.QueryEscape(value)
			}
		}
		output.WriteString(template[position:match[0]])
		output.WriteString(value)
		position = match[1]
	}
	output.WriteString(template[position:])
	return output.String(), nil
}

func renderSMSJSON(value any, c smsForwardConfig, j smsForwardJob) (any, error) {
	switch v := value.(type) {
	case string:
		return renderSMSTemplate(v, c, j, "")
	case []any:
		for i := range v {
			next, err := renderSMSJSON(v[i], c, j)
			if err != nil {
				return nil, err
			}
			v[i] = next
		}
	case map[string]any:
		for key := range v {
			next, err := renderSMSJSON(v[key], c, j)
			if err != nil {
				return nil, err
			}
			v[key] = next
		}
	}
	return value, nil
}

func applySMSHeaders(req *http.Request, c smsForwardConfig, j smsForwardJob) error {
	if c.Authorization != "" {
		value, err := renderSMSTemplate(c.Authorization, c, j, "")
		if err != nil || strings.ContainsAny(value, "\r\n") {
			return errors.New("鉴权模板无效")
		}
		req.Header.Set("Authorization", value)
	}
	for name, template := range c.Headers {
		value, err := renderSMSTemplate(template, c, j, "")
		if err != nil || strings.ContainsAny(value, "\r\n") {
			return errors.New("请求头模板无效")
		}
		req.Header.Set(name, value)
	}
	return nil
}

func smsForwardRequest(ctx context.Context, config smsForwardConfig, j smsForwardJob) (*http.Request, error) {
	c := normalizeSMSRequest(config)
	endpoint, err := renderSMSTemplate(c.Endpoint, c, j, "url")
	if err != nil {
		return nil, err
	}
	var body, contentType string
	switch c.Format {
	case "form":
		values := url.Values{}
		for _, line := range strings.Split(c.Body, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			key, template, ok := strings.Cut(line, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return nil, errors.New("表单模板应每行填写 字段=值")
			}
			value, err := renderSMSTemplate(template, c, j, "")
			if err != nil {
				return nil, err
			}
			values.Add(strings.TrimSpace(key), value)
		}
		body, contentType = values.Encode(), "application/x-www-form-urlencoded"
	case "json":
		var value any
		if json.Unmarshal([]byte(c.Body), &value) != nil {
			return nil, errors.New("JSON 模板格式无效；变量放在 JSON 字符串中")
		}
		value, err = renderSMSJSON(value, c, j)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		body, contentType = string(raw), "application/json"
	case "text":
		body, err = renderSMSTemplate(c.Body, c, j, "")
		if err != nil {
			return nil, err
		}
		contentType = "text/plain; charset=utf-8"
	case "none":
	default:
		return nil, errors.New("不支持的请求体格式")
	}
	req, err := http.NewRequestWithContext(ctx, c.Method, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, errors.New("请求地址无效")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Idempotency-Key", j.ID)
	if err := applySMSHeaders(req, c, j); err != nil {
		return nil, err
	}
	return req, nil
}

func validateSMSRequest(config smsForwardConfig) error {
	c := normalizeSMSRequest(config)
	switch c.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return errors.New("不支持的 HTTP 请求方式")
	}
	if c.Method == "GET" && c.Format != "none" {
		return errors.New("GET 请将请求体格式设为无")
	}
	if c.Timeout < 3 || c.Timeout > 60 {
		return errors.New("超时应为 3–60 秒")
	}
	if len(c.Body) > 16384 || len(c.Headers) > 32 || len(c.Secrets) > 32 {
		return errors.New("请求模板或参数过长")
	}
	for key, value := range c.Secrets {
		if !smsSecretName.MatchString(key) || value == "" || len(value) > 4096 {
			return errors.New("密钥变量名称或值无效")
		}
	}
	for name, value := range c.Headers {
		lower := strings.ToLower(name)
		if !smsHeaderName.MatchString(name) || len(value) > 4096 || strings.ContainsAny(value, "\r\n") {
			return errors.New("请求头名称或值无效")
		}
		switch lower {
		case "host", "content-length", "transfer-encoding", "connection", "upgrade", "proxy-authorization":
			return errors.New("该请求头由 HTTP 客户端管理")
		}
	}
	u, _ := url.Parse(c.Endpoint)
	if u == nil || strings.Contains(u.Host, "{{") {
		return errors.New("接收地址的主机名不能使用模板")
	}
	if c.CheckEndpoint != "" {
		check, err := url.Parse(c.CheckEndpoint)
		if err != nil || check.Scheme != u.Scheme || !strings.EqualFold(check.Host, u.Host) || check.User != nil || check.Fragment != "" || strings.Contains(c.CheckEndpoint, "{{sms-") {
			return errors.New("只读检查地址必须同站点，且不能使用短信变量")
		}
		if _, err := renderSMSTemplate(c.CheckEndpoint, c, smsForwardJob{}, "url"); err != nil {
			return err
		}
		for _, variable := range smsVariable.FindAllStringSubmatch(c.CheckEndpoint, -1) {
			if !strings.HasPrefix(strings.TrimSpace(variable[1]), "secret:") {
				return errors.New("只读检查地址仅支持密钥变量")
			}
		}
	}
	switch c.Success {
	case "http", "accepted":
	case "json":
		var expected any
		if c.SuccessPath == "" || len(c.SuccessPath) > 128 || len(c.SuccessValue) > 1024 || json.Unmarshal([]byte(c.SuccessValue), &expected) != nil {
			return errors.New("请填写 JSON 成功字段和有效 JSON 期望值，例如 code / 0")
		}
	default:
		return errors.New("不支持的成功判定")
	}
	_, err := smsForwardRequest(context.Background(), c, smsForwardJob{ID: "preview", From: "+441234567890", Text: "模板检查", At: time.Now()})
	return err
}

func smsForwardAccepted(c smsForwardConfig, raw []byte) bool {
	c = normalizeSMSRequest(c)
	if c.Success == "http" {
		return true
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	path, expected := c.SuccessPath, c.SuccessValue
	if c.Success == "accepted" {
		path, expected = "accepted", "true"
	}
	for _, key := range strings.Split(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return false
		}
		var found bool
		value, found = object[key]
		if !found {
			return false
		}
	}
	var want any
	if json.Unmarshal([]byte(expected), &want) != nil {
		return false
	}
	a, _ := json.Marshal(value)
	b, _ := json.Marshal(want)
	return string(a) == string(b)
}

func (f *smsForwarder) preview(c smsForwardConfig) (map[string]any, error) {
	f.mu.Lock()
	c = preserveSMSSecrets(c, f.config)
	f.mu.Unlock()
	if err := validateSMSForwardConfig(c); err != nil {
		return nil, err
	}
	j := smsForwardJob{ID: "example-message-id", From: "+441234567890", Text: "示例短信：hello & 中文\n第二行", At: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	req, err := smsForwardRequest(context.Background(), c, j)
	if err != nil {
		return nil, err
	}
	redact := func(value string) string {
		secrets := []string{c.Authorization}
		for k, v := range c.Headers {
			if sensitiveSMSHeader(c, k) {
				secrets = append(secrets, v)
			}
		}
		for _, v := range c.Secrets {
			secrets = append(secrets, v)
		}
		for _, secret := range secrets {
			if secret == "" {
				continue
			}
			encoded, _ := json.Marshal(secret)
			for _, s := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret), string(encoded[1 : len(encoded)-1])} {
				value = strings.ReplaceAll(value, s, smsSecretMask)
			}
		}
		return value
	}
	headers := map[string]string{}
	for key := range req.Header {
		value := req.Header.Get(key)
		if sensitiveSMSHeader(c, key) {
			value = smsSecretMask
		} else {
			value = redact(value)
		}
		headers[key] = value
	}
	body, _ := io.ReadAll(req.Body)
	defer clear(body)
	return map[string]any{"method": req.Method, "endpoint": redact(maskedSMSForwardEndpoint(req.URL.String())), "headers": headers, "body": redact(string(body)), "network_request_sent": false}, nil
}
