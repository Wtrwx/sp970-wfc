# 自定义短信转发

请求设置只在点击“保存设置”后生效。现有转发配置自动按兼容表单载入，不会更改接收地址、密钥或启用时间。

## 请求编辑

- 方式：GET、POST、PUT、PATCH、DELETE。
- 地址：HTTPS，可在路径和查询参数中使用变量；主机名固定。
- 格式：表单、JSON、纯文本或无请求体。GET 使用“无请求体”。
- 请求头：JSON 字符串对象，可覆盖 Content-Type；Host 和 Content-Length 等由客户端管理。
- 超时：3–60 秒。失败进入持久队列，最多每小时左右重试一次，直到成功或关闭转发。

预设只填写请求结构，不替换地址和密钥。通用 JSON / 纯文本 / URL 预设按任意 HTTP 2xx 判定成功；兼容表单预设还要求响应 `accepted=true`。应根据自己的接收服务调整成功条件。

## 模板变量

| 变量 | 含义 |
| --- | --- |
| `{{sms-from}}` | 发件号码 |
| `{{sms-time}}` | UTC / RFC3339 时间 |
| `{{sms-body}}` | 原始短信正文 |
| `{{sms-id}}` | 稳定消息 ID；重试保持不变 |
| `{{sms-body-escaped}}` | JSON 转义后的正文，用于兼容原有表单接收器 |
| `{{secret:token}}` | 名为 token 的私有密钥变量 |

表单模板每行一个 `字段=值`，无需手动 URL 编码：

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

在“密钥变量”输入 JSON：

```json
{"token":"替换成自己的密钥"}
```

随后在地址、请求头或请求体引用 `{{secret:token}}`，例如 `Bearer {{secret:token}}`。保存后只显示星号，真实值留在棒子私有配置中。不要把密钥直接写进公开模板截图。

Authorization、Cookie、Token、API-Key 等请求头自动视为敏感。其他名称可以加入“其他敏感请求头”，用逗号分隔。直接保存星号会保留原值；删除 JSON 对象中的键会移除该项。Authorization 留空表示保留，勾选“清除已有 Authorization”才能删除。

## 成功判定和预览

所有模式都要求 HTTP 2xx；不会跟随重定向。可选择：

1. 响应 JSON 的 `accepted` 为 `true`。
2. 仅判断 HTTP 2xx，适用于返回空响应的服务。
3. JSON 字段路径匹配，例如路径 `data.code`，期望值 `0`。期望值应是有效 JSON，字符串要填 `"ok"`。

“预览请求”仅使用固定示例号码和示例正文渲染地址、请求头与请求体，不保存配置，不访问接收服务，密钥会遮盖。

“检查已保存配置”只访问你配置的同站点只读 GET 地址。请填写健康检查接口，避免填写会触发通知的地址。兼容 `/v1/sms` 接收器可不填，自动检查 `/v1/status`；不会发送测试短信。

关闭转发会暂停待重试任务。历史收件箱不会补发，已成功任务不会再次转发。
