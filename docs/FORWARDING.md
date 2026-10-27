# 自定义短信转发

请求设置只在点击“保存设置”后生效。现有转发配置自动按兼容表单载入，不会更改接收地址、密钥或启用时间。

## 直接粘贴 cURL

页面只有转发开关、cURL 命令框和保存按钮，没有示例或高级设置。已有配置自动转换成 cURL，Authorization 显示完整值，方便编辑和复制。

JSON 请求示例：

```sh
curl -X POST 'https://notify.example.com/webhook' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  --json '{"from":"{{sms-from}}","text":"{{sms-body}}"}'
```

表单示例：

```sh
curl 'https://notify.example.com/webhook' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  --data-urlencode 'sender={{sms-from}}' \
  --data-urlencode 'message={{sms-body}}'
```

支持单行、多行反斜杠续行、单/双引号、`-X` / `--request`、`-H` / `--header`、`--url`、`-d` / `--data` / `--data-raw` / `--data-binary`、`--data-urlencode`、`--json`、`-u` / `--user`、`--oauth2-bearer`、`-G` / `--get`、`-A` / `--user-agent`、`-e` / `--referer`、`--max-time`。请求超时为 3–60 整数秒，默认 15 秒。

`-d` 的表单数据按 URL 编码解析，`--data-urlencode '字段=值'` 的值无需手动编码。纯文本请求请声明 `Content-Type: text/plain`。敏感值推荐放在鉴权请求头，或使用下文的密钥变量。

本功能解析一条 cURL 请求，不启动 Shell，也不依赖设备安装 curl。不支持命令拼接、Shell 变量、命令替换、本地文件读取、`--insecure` 或重定向；不支持的参数会明确报错。`-s` / `-S` / `--compressed` 等输出/压缩选项可正常粘贴。

新接收站点默认以 HTTP 2xx 判定成功；原有接收站点保留之前的成功判定。修改接收站点时需要填入新的鉴权，不能把星号当成新站点的密钥。

## 模板变量

| 变量 | 含义 |
| --- | --- |
| `{{sms-from}}` | 发件号码 |
| `{{sms-time}}` | UTC / RFC3339 时间 |
| `{{sms-body}}` | 原始短信正文 |
| `{{sms-id}}` | 稳定消息 ID；重试保持不变 |
| `{{sms-body-escaped}}` | JSON 转义后的正文，用于兼容原有表单接收器 |
| `{{secret:token}}` | 名为 token 的私有密钥变量 |

以下表单值可分别放入 `--data-urlencode`，无需手动 URL 编码：

```text
sender={{sms-from}}
message={{sms-body}}
message_id={{sms-id}}
```

JSON 模板中变量放在字符串内。程序先解析 JSON，再替换字符串并重新编码，中文、引号与换行都会正确转义：

```json
{
  "title": "短信 · {{sms-from}}",
  "message": "{{sms-body}}",
  "timestamp": "{{sms-time}}"
}
```

URL 查询变量也会自动编码，例如 `https://notify.example.com/push?text={{sms-body}}`。正文中的 `&` 或 `+` 不会变成额外查询参数。

## 私有参数与星号

已有密钥变量继续保留，可在地址、请求头或请求体引用 `{{secret:token}}`，例如 `Bearer {{secret:token}}`。页面不再单独编辑这些变量；新配置直接在 cURL 中填写参数即可。

Authorization 在命令和预览中显示完整值。Cookie、Token、API-Key 等其他请求头自动视为敏感；其他自定义敏感请求头列表继续保留。直接保存星号会保留原值。删除 cURL 中的请求头会移除该项，包括 Authorization；旧页面留下的星号仍可兼容保存。

## 兼容已有配置

所有模式都要求 HTTP 2xx；不会跟随重定向。已有成功判定可以是：

1. 响应 JSON 的 `accepted` 为 `true`。
2. 仅判断 HTTP 2xx，适用于返回空响应的服务。
3. JSON 字段路径匹配，例如路径 `data.code`，期望值 `0`。期望值应是有效 JSON，字符串要填 `"ok"`。

页面不提供示例预览。开发接口 `/forward/preview` 仍兼容，只渲染示例、不访问接收服务；Authorization 显示原值，其他密钥会遮盖。

开发接口 `/forward/check` 只访问已有配置的同站点只读 GET 地址。已有自定义健康检查地址会继续使用。兼容 `/v1/sms` 接收器可不填，自动检查 `/v1/status`；不会发送测试短信。

关闭转发会暂停待重试任务。历史收件箱不会补发，已成功任务不会再次转发。
