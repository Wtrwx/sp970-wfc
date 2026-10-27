"""Local-only configuration. Credentials never enter command-line arguments."""
from __future__ import annotations

import getpass
import ipaddress
import json
import os
from pathlib import Path
import re
import socket
from urllib.parse import parse_qs, unquote, urlsplit

EPDGS = {"84.246.115.151", "84.246.113.151"}


def parse_node(uri: str, *, resolve=True) -> tuple[dict, str]:
    u = urlsplit(uri.strip())
    if u.scheme not in ("hysteria2", "hy2") or not u.hostname or not u.username:
        raise ValueError("请输入有效的 hysteria2:// 或 hy2:// 节点链接")
    if u.password is not None:
        raise ValueError("请将完整 Hysteria2 密码放在 @ 前；密码中的冒号需要 URL 编码")
    q = parse_qs(u.query)
    unsupported = set(q) - {"insecure", "sni", "pinSHA256", "obfs", "obfs-password", "alpn"}
    if unsupported:
        raise ValueError("暂不支持的节点参数: " + ", ".join(sorted(unsupported)))
    if "alpn" in q:
        raise ValueError("该版本使用标准 Hysteria2 ALPN，请移除自定义 alpn 参数")
    hostname = u.hostname
    try:
        address = str(ipaddress.IPv4Address(hostname))
    except ipaddress.AddressValueError:
        if not resolve:
            address = "192.0.2.1"
        else:
            choices = socket.getaddrinfo(hostname, u.port or 443, socket.AF_INET, socket.SOCK_DGRAM)
            if not choices:
                raise ValueError("节点没有可用的 IPv4 地址")
            address = choices[0][4][0]
    pin = q.get("pinSHA256", [""])[0].replace(":", "").lower()
    if pin and not re.fullmatch(r"[0-9a-f]{64}", pin):
        raise ValueError("pinSHA256 必须为 64 位十六进制证书 SHA256")
    insecure = q.get("insecure", ["0"])[0] in ("1", "true")
    if insecure and not pin:
        raise ValueError("insecure=1 时必须同时提供 pinSHA256，防止连接到冒充的节点")
    tls = {"sni": q.get("sni", [hostname])[0], "insecure": insecure}
    if pin:
        tls["pinSHA256"] = pin
    config = {
        "server": f"{address}:{u.port or 443}", "auth": unquote(u.username),
        "tls": tls, "fastOpen": False,
        "socks5": {"listen": "127.0.0.1:17890"},
        "http": {"listen": "127.0.0.1:17891"},
        "tun": {"name": "hypsm0", "mtu": 1500, "address": {
            "ipv4": "100.100.100.101/30", "ipv6": "fd44:5350:970::1/126"}},
    }
    if q.get("obfs"):
        if q["obfs"][0] != "salamander" or not q.get("obfs-password"):
            raise ValueError("仅支持带 obfs-password 的 salamander 混淆")
        config["obfs"] = {"type": "salamander", "salamander": {"password": q["obfs-password"][0]}}
    return config, address


def validate(config: dict) -> dict:
    if config.get("mode") not in ("hysteria2", "direct"):
        raise ValueError("mode 只能是 hysteria2 或 direct")
    if config.get("epdg_ipv4", "84.246.115.151") not in EPDGS:
        raise ValueError("当前仅支持已验证的 Lebara UK ePDG 地址")
    if config["mode"] == "hysteria2":
        node = config.get("hysteria", {})
        address = str(ipaddress.IPv4Address(config["proxy_ipv4"]))
        if not node.get("auth") or node.get("auth") == "CHANGE_ME":
            raise ValueError("请填写自己的节点认证信息")
        server = node.get("server", "")
        host, _, port = server.rpartition(":")
        if host != address or not port.isdigit() or not 1 <= int(port) <= 65535:
            raise ValueError("节点 server 的 IPv4 必须与 proxy_ipv4 一致，端口必须有效")
        tls = node.get("tls", {})
        pin = tls.get("pinSHA256", "").replace(":", "")
        if tls.get("insecure") and not re.fullmatch(r"[0-9a-fA-F]{64}", pin):
            raise ValueError("跳过 CA 验证时必须提供有效 pinSHA256")
        expected = {"name": "hypsm0", "mtu": 1500, "address": {
            "ipv4": "100.100.100.101/30", "ipv6": "fd44:5350:970::1/126"}}
        if node.get("tun") != expected:
            raise ValueError("请使用本项目生成的 TUN 配置（hypsm0 / MTU 1500）")
        # DNS resolution is completed locally; the device connects to this IPv4.
        # No config-controlled command or external process is accepted.
        permitted = {"server", "auth", "tls", "fastOpen", "socks5", "http", "tun", "obfs"}
        if set(node) - permitted:
            raise ValueError("节点配置含不支持的字段，请通过向导重新生成")
        for key, expected_port in (("socks5", 17890), ("http", 17891)):
            if node.get(key, {}).get("listen") != f"127.0.0.1:{expected_port}":
                raise ValueError("本地代理监听仅允许本机回环地址")
    return config


def save_private(path: Path, value: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.write("\n")
    path.chmod(0o600)


def wizard(path: Path) -> dict:
    print("网络方式：1. 在棒子上运行 Hysteria2 英国代理（推荐）  2. 路由器已提供可用网络")
    mode = input("选择 [1]: ").strip() or "1"
    if mode == "1":
        node, address = parse_node(getpass.getpass("粘贴自己的 Hysteria2 链接（不回显）: "))
        result = {"mode": "hysteria2", "proxy_ipv4": address, "hysteria": node}
    elif mode == "2":
        result = {"mode": "direct"}
    else:
        raise ValueError("请选择 1 或 2")
    result["epdg_ipv4"] = "84.246.115.151"
    validate(result)
    save_private(path, result)
    print(f"配置已保存至 {path}（权限 0600），不会上传。")
    return result
