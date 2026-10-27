# 可选短信推送网关

使用标准库 Python 服务，将棒子的转发表单保存到 SQLite 队列，再推送到自己的 ntfy topic。无需使用项目作者的服务器或凭据。

```sh
cp .env.example .env
python3 -c "import secrets; print(secrets.token_hex(32))"
# 将随机字符串填入 .env 的 INGRESS_TOKEN，另生成一个不同的随机 NTFY_TOPIC。
docker compose up -d --build
```

服务只映射本机 `127.0.0.1:2587`，请用自己的域名和 HTTPS 反向代理发布。例如 Caddy：

```caddy
sms.example.com {
    reverse_proxy 127.0.0.1:2587
}
```

棒子面板填写 `https://sms.example.com/v1/sms` 和 `Bearer <INGRESS_TOKEN>`。连接检查使用鉴权后的 `GET /v1/status`，不会发布通知。接收成功返回 `{"accepted":true}`；重复请求持久去重，ntfy 失败自动重试。

手机上的 ntfy 客户端订阅自己设置的 topic。通知到达速度和后台推送能力取决于客户端、所用 ntfy 服务及系统的后台限制，相关设置参考 [ntfy 官方文档](https://docs.ntfy.sh/subscribe/phone/)。

`.env` 含真实令牌，不能提交。服务端数据库包含短信正文，应保存在自己的受保护存储中。
