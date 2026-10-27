// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// This is a request importer, never a shell or a call to the curl executable.
func smsCurlWords(source string) ([]string, error) {
	if len(source) > 32768 {
		return nil, errors.New("cURL 命令过长")
	}
	var words []string
	var word strings.Builder
	var quote rune
	started := false
	chars := []rune(source)
	for i := 0; i < len(chars); i++ {
		ch := chars[i]
		if quote == '\'' {
			if ch == '\'' {
				quote = 0
			} else {
				word.WriteRune(ch)
			}
			continue
		}
		if ch == '\\' {
			if i+1 >= len(chars) {
				return nil, errors.New("命令末尾的反斜杠不完整")
			}
			next := chars[i+1]
			if next == '\n' {
				i++
				continue
			}
			if next == '\r' && i+2 < len(chars) && chars[i+2] == '\n' {
				i += 2
				continue
			}
			if quote == '"' && !strings.ContainsRune("\\\"$`", next) {
				word.WriteRune(ch)
				continue
			}
			word.WriteRune(next)
			i++
			started = true
			continue
		}
		if ch == '$' || ch == '`' {
			return nil, errors.New("请使用 {{短信变量}}，不支持 Shell 变量或命令替换")
		}
		if quote == '"' {
			if ch == '"' {
				quote = 0
			} else {
				word.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			started = true
			continue
		}
		if unicode.IsSpace(ch) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		if strings.ContainsRune(";|&<>()", ch) {
			return nil, errors.New("请只粘贴一条 cURL 命令；地址和数据用引号包住")
		}
		word.WriteRune(ch)
		started = true
	}
	if quote != 0 {
		return nil, errors.New("cURL 命令的引号未闭合")
	}
	if started {
		words = append(words, word.String())
	}
	if len(words) < 2 || (words[0] != "curl" && words[0] != "/usr/bin/curl") {
		return nil, errors.New("请粘贴以 curl 开头的命令")
	}
	return words, nil
}

type smsCurlBody struct{ kind, value string }

func parseSMSCurl(source string, old smsForwardConfig) (smsForwardConfig, error) {
	words, err := smsCurlWords(source)
	if err != nil {
		return smsForwardConfig{}, err
	}
	c := smsForwardConfig{Method: "GET", Format: "none", Headers: map[string]string{}, Secrets: old.Secrets, SecretHeaders: old.SecretHeaders,
		Success: old.Success, SuccessPath: old.SuccessPath, SuccessValue: old.SuccessValue, CheckEndpoint: old.CheckEndpoint, Timeout: old.Timeout, ClearAuth: true}
	var bodies []smsCurlBody
	explicitMethod, useQuery := false, false
	for i := 1; i < len(words); i++ {
		arg := words[i]
		if !strings.HasPrefix(arg, "-") {
			if c.Endpoint != "" {
				return c, errors.New("只支持单个请求地址")
			}
			c.Endpoint = arg
			continue
		}
		name, value, hasValue := arg, "", false
		if strings.HasPrefix(arg, "--") {
			name, value, hasValue = strings.Cut(arg, "=")
		} else if len(arg) > 2 && strings.ContainsRune("XHdmuAe", rune(arg[1])) {
			name, value, hasValue = arg[:2], arg[2:], true
		}
		switch name {
		case "-s", "-S", "-sS", "-Ss", "--silent", "--show-error", "--compressed", "-f", "--fail", "--fail-with-body", "--http1.1":
			continue
		case "-G", "--get":
			useQuery = true
			continue
		}
		switch name {
		case "-X", "--request", "--url", "-H", "--header", "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "--json", "-u", "--user", "--oauth2-bearer", "-m", "--max-time", "-A", "--user-agent", "-e", "--referer":
		default:
			return c, errors.New("暂不支持 cURL 参数：" + strings.SplitN(name, "=", 2)[0])
		}
		if !hasValue {
			i++
			if i >= len(words) {
				return c, errors.New("cURL 参数缺少值：" + name)
			}
			value = words[i]
		}
		switch name {
		case "-X", "--request":
			c.Method = strings.ToUpper(value)
			explicitMethod = true
		case "--url":
			if c.Endpoint != "" {
				return c, errors.New("只支持单个请求地址")
			}
			c.Endpoint = value
		case "-H", "--header":
			key, v, ok := strings.Cut(value, ":")
			if !ok {
				return c, errors.New("请求头应为 名称: 值")
			}
			key = http.CanonicalHeaderKey(strings.TrimSpace(key))
			v = strings.TrimSpace(v)
			if key == "Authorization" {
				c.Authorization = v
				c.ClearAuth = v == ""
			} else {
				c.Headers[key] = v
			}
		case "-u", "--user":
			c.Authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(value))
			c.ClearAuth = false
		case "--oauth2-bearer":
			c.Authorization = "Bearer " + value
			c.ClearAuth = false
		case "-m", "--max-time":
			seconds, e := strconv.Atoi(value)
			if e != nil || seconds < 3 || seconds > 60 {
				return c, errors.New("--max-time 应为 3–60 的整数秒")
			}
			c.Timeout = seconds
		case "-A", "--user-agent":
			c.Headers["User-Agent"] = value
		case "-e", "--referer":
			c.Headers["Referer"] = value
		default:
			if strings.HasPrefix(value, "@") && name != "--data-raw" {
				return c, errors.New("请直接粘贴数据，不支持读取本地文件")
			}
			kind := "raw"
			if name == "--data-urlencode" {
				kind = "urlencode"
			}
			if name == "--json" {
				kind = "json"
				c.Headers["Content-Type"] = "application/json"
				c.Headers["Accept"] = "application/json"
			}
			if name == "--data-binary" || name == "--data-raw" {
				kind = "raw"
			}
			bodies = append(bodies, smsCurlBody{kind, value})
		}
	}
	if c.Endpoint == "" {
		return c, errors.New("cURL 命令缺少 HTTPS 接收地址")
	}
	if len(bodies) > 0 {
		if !explicitMethod {
			c.Method = "POST"
		}
		contentType := strings.ToLower(c.Headers["Content-Type"])
		isJSON := strings.Contains(contentType, "json")
		for _, b := range bodies {
			if b.kind == "json" {
				isJSON = true
			}
		}
		if isJSON {
			c.Format = "json"
			c.Body = ""
			for _, b := range bodies {
				if b.kind == "urlencode" {
					return c, errors.New("JSON 请求不能混用 --data-urlencode")
				}
				c.Body += b.value
			}
		} else if contentType == "" || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			c.Format = "form"
			c.FormFields = []smsForwardField{}
			for _, b := range bodies {
				if b.kind == "urlencode" {
					key, v, ok := strings.Cut(b.value, "=")
					if !ok || key == "" {
						return c, errors.New("--data-urlencode 应为 字段=值")
					}
					c.FormFields = append(c.FormFields, smsForwardField{key, v})
					continue
				}
				if !strings.Contains(b.value, "=") {
					return c, errors.New("纯文本数据请用 -H 'Content-Type: text/plain'；表单请用 字段=值")
				}
				for _, pair := range strings.Split(b.value, "&") {
					key, v, _ := strings.Cut(pair, "=")
					key, e1 := url.QueryUnescape(key)
					v, e2 := url.QueryUnescape(v)
					if e1 != nil || e2 != nil || key == "" {
						return c, errors.New("表单编码无效，请使用 --data-urlencode '字段=值'")
					}
					c.FormFields = append(c.FormFields, smsForwardField{key, v})
				}
			}
		} else {
			c.Format = "text"
			for i, b := range bodies {
				if b.kind == "urlencode" {
					return c, errors.New("纯文本请求不能混用 --data-urlencode")
				}
				if i > 0 {
					c.Body += "&"
				}
				c.Body += b.value
			}
		}
	}
	if useQuery {
		if c.Format != "form" {
			return c, errors.New("--get 请与表单数据一起使用")
		}
		u, e := url.Parse(c.Endpoint)
		if e != nil {
			return c, errors.New("请求地址无效")
		}
		for _, field := range c.FormFields {
			if u.RawQuery != "" {
				u.RawQuery += "&"
			}
			u.RawQuery += url.QueryEscape(field.Name) + "=" + smsCurlQueryTemplate(field.Value)
		}
		c.Endpoint = u.String()
		c.Format = "none"
		c.FormFields = nil
		c.Body = ""
		if !explicitMethod {
			c.Method = "GET"
		}
	}
	return c, nil
}

func smsCurlQueryTemplate(value string) string {
	encoded := url.QueryEscape(value)
	for _, token := range smsVariable.FindAllString(value, -1) {
		encoded = strings.ReplaceAll(encoded, url.QueryEscape(token), token)
	}
	return encoded
}

func prepareSMSForwardConfig(c, old smsForwardConfig) (smsForwardConfig, error) {
	if c.Curl == "" {
		return preserveSMSSecrets(c, old), nil
	}
	parsed, err := parseSMSCurl(c.Curl, old)
	if err != nil {
		return parsed, err
	}
	parsed.Enabled = c.Enabled
	if c.Secrets != nil {
		parsed.Secrets = c.Secrets
	}
	if c.SecretHeaders != nil {
		parsed.SecretHeaders = c.SecretHeaders
	}
	previous, _ := url.Parse(old.Endpoint)
	next, _ := url.Parse(parsed.Endpoint)
	if previous == nil || next == nil || previous.Scheme != next.Scheme || !strings.EqualFold(previous.Host, next.Host) {
		if strings.Contains(parsed.Authorization, "********") {
			return parsed, errors.New("更换接收站点后请填写新鉴权，不能沿用星号")
		}
		for _, v := range parsed.Headers {
			if v == smsSecretMask {
				return parsed, errors.New("更换接收站点后请重新填写敏感请求头")
			}
		}
		parsed.CheckEndpoint = ""
		parsed.Success = "http"
		parsed.SuccessPath = ""
		parsed.SuccessValue = ""
	}
	if c.Success != "" {
		parsed.Success, parsed.SuccessPath, parsed.SuccessValue = c.Success, c.SuccessPath, c.SuccessValue
	}
	return preserveSMSSecrets(parsed, old), nil
}

func smsCurlQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func smsForwardCurl(config smsForwardConfig) string {
	c := normalizeSMSRequest(config)
	endpoint := maskedSMSForwardEndpoint(c.Endpoint)
	if endpoint == "" {
		return ""
	}
	parts := []string{"curl -X " + c.Method + " " + smsCurlQuote(endpoint)}
	if c.Authorization != "" {
		parts = append(parts, "-H "+smsCurlQuote("Authorization: "+c.Authorization))
	}
	headers := maskedSMSHeaders(c)
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, "-H "+smsCurlQuote(key+": "+headers[key]))
	}
	switch c.Format {
	case "json":
		if headers["Content-Type"] == "" {
			parts = append(parts, "-H 'Content-Type: application/json'")
		}
		parts = append(parts, "--data-raw "+smsCurlQuote(c.Body))
	case "text":
		if headers["Content-Type"] == "" {
			parts = append(parts, "-H 'Content-Type: text/plain; charset=utf-8'")
		}
		parts = append(parts, "--data-raw "+smsCurlQuote(c.Body))
	case "form":
		fields := c.FormFields
		if fields == nil {
			for _, line := range strings.Split(c.Body, "\n") {
				if key, value, ok := strings.Cut(line, "="); ok {
					fields = append(fields, smsForwardField{strings.TrimSpace(key), value})
				}
			}
		}
		for _, field := range fields {
			parts = append(parts, "--data-urlencode "+smsCurlQuote(field.Name+"="+field.Value))
		}
	}
	if c.Timeout != 15 {
		parts = append(parts, "--max-time "+strconv.Itoa(c.Timeout))
	}
	return strings.Join(parts, " \\\n  ")
}
