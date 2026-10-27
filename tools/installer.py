from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import shutil
import subprocess
import sys
import tempfile
import time

from .configure import validate, wizard

ROOT = Path(__file__).resolve().parent.parent
BASE = "/data/adb/sp970-wfc"
BOOT = "/data/adb/service.d/90-sp970-wfc.sh"
NGINX = "/system/etc/nginx/conf/nginx.conf"
WWW = "/system/var/www"
MARKER = "# SP970-WFC managed API"


class Adb:
    def __init__(self, args):
        executable = shutil.which(args.adb)
        if not executable:
            raise ValueError("未找到 adb，请先安装 Android Platform Tools，并把 adb 加入 PATH")
        self.prefix = [executable, "-H", args.adb_host, "-P", str(args.adb_port)]
        self.serial = args.serial
        self.use_su = False
        if not self.serial:
            text = self.run("devices", "-l")
            devices = [row.split()[0] for row in text.splitlines() if re.match(r"^\S+\s+device(?:\s|$)", row)]
            if not devices:
                raise ValueError("没有已授权的 ADB 设备，请连接棒子并允许 USB 调试")
            if len(devices) > 1:
                print("可用设备：")
                for i, serial in enumerate(devices, 1):
                    print(f"  {i}. {serial}")
                selected = int(input("选择目标 SP970-B 的序号: ")) - 1
                if selected < 0 or selected >= len(devices):
                    raise ValueError("设备序号无效")
                self.serial = devices[selected]
            else:
                self.serial = devices[0]
        self.prefix += ["-s", self.serial]
        if "uid=0(" not in self.shell("id"):
            self.use_su = True
            if "uid=0(" not in self.shell("id"):
                raise ValueError("设备没有可用的 root，请先完成 Magisk/root 授权")

    def run(self, *args, timeout=45):
        result = subprocess.run(self.prefix + list(args), capture_output=True, timeout=timeout)
        if result.returncode:
            # Never include command arguments (which may refer to private config).
            raise RuntimeError("ADB 操作失败: " + result.stderr.decode(errors="replace").strip()[:500])
        return result.stdout.decode(errors="replace").replace("\r\n", "\n")

    def shell(self, command, *, timeout=45):
        prefix = "su -c " if self.use_su else "/system/bin/sh -c "
        sentinel = "__SP970_RC_" + secrets.token_hex(6) + "__"
        wrapped = prefix + shlex.quote(command + f"\n__sp970_rc=$?; echo {sentinel}$__sp970_rc")
        output = self.run("shell", wrapped, timeout=timeout)
        before, separator, code = output.rpartition(sentinel)
        if not separator or code.strip() != "0":
            raise RuntimeError("设备操作未成功完成: " + before.strip()[-800:])
        return before.strip()

    def push(self, source, destination):
        self.run("push", str(source), destination, timeout=120)


def patch_nginx(text: str) -> str:
    if MARKER in text:
        return text
    anchor = re.search(r"(?m)^\s*location\s+/\s*\{", text)
    if not anchor:
        raise ValueError("不支持该 Nginx 配置：找不到原生 location /，未改动文件")
    api = """        # SP970-WFC managed API
        location /wfc/ {
            proxy_pass http://127.0.0.1:17892/;
            proxy_set_header Host $host;
            proxy_read_timeout 95s;
            client_max_body_size 64k;
            access_log off;
        }
        location = /wfc-state {
            alias /data/adb/sp970-wfc/run/state;
            default_type text/plain;
            add_header Cache-Control no-store;
            access_log off;
        }
"""
    if re.search(r"location\s+/wfc/\s*\{", text):
        # Upgrade the prototype without duplicating its API location.
        api = api[api.index("        location = /wfc-state"):]
        api = "        " + MARKER + "\n" + api
    return text[:anchor.start()] + "\n" + api + text[anchor.start():]


def patch_native_html(text: str) -> str:
    if 'src="/wfc-native-sms.js"' in text:
        return text
    match = re.search(r'(?i)(<script\b[^>]*\bsrc=["\'][^"\']*/?common\.js(?:\?[^"\']*)?["\'][^>]*>\s*</script>)', text)
    if not match:
        raise ValueError("不支持该原生页面：找不到 common.js，未改动文件")
    return text[:match.end()] + '\n<script src="/wfc-native-sms.js"></script>' + text[match.end():]


def reload_native_nginx() -> str:
    # HUP the verified native master only. Keep the listener available even for
    # --no-start installs and when restoring the original configuration.
    return f'''/system/bin/nginx -t -p /sdcard/nginx -c {NGINX} && (
P=$(cat /sdcard/nginx/logs/nginx.pid 2>/dev/null)
case "$P" in ''|*[!0-9]*|0|1) exit 0;; esac
E=$(/system/xbin/busybox readlink /proc/$P/exe)
if [ "$E" = /system/bin/nginx ]; then kill -HUP "$P"; fi
)'''


def rollback_script(files: dict[str, str], backup: str, *, old_control: bool, old_enabled: bool) -> str:
    lines = ["#!/system/bin/sh", "BB=/system/xbin/busybox",
             f"/system/bin/sh {BASE}/service-control.sh disable 2>/dev/null", "sleep 18"]
    for destination, name in files.items():
        lines += [f'if [ -f {backup}/{name}.absent ]; then rm -f {destination}; '
                  f'elif [ -f {backup}/{name} ]; then "$BB" cp -p {backup}/{name} {destination}; fi']
    lines += [reload_native_nginx()]
    if old_control:
        lines += [f"/system/bin/sh {BASE}/service-control.sh {'enable' if old_enabled else 'disable'}"]
    else:
        lines += [f"rm -f {BOOT}"]
    return "\n".join(lines) + "\n"


def verify_payload(payload: Path) -> None:
    sums = payload / "SHA256SUMS"
    if not sums.is_file():
        raise ValueError("找不到完整安装包。请下载 Releases 的 android-armv7.zip，或先运行 tools/build.py")
    seen = set()
    for line in sums.read_text().splitlines():
        digest, name = line.split("  ", 1)
        relative = Path(name)
        if relative.is_absolute() or ".." in relative.parts or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise ValueError("安装包校验清单格式无效")
        file = payload / relative
        if not file.is_file() or hashlib.sha256(file.read_bytes()).hexdigest() != digest:
            raise ValueError("安装包校验失败: " + name)
        seen.add(name)
    required = {"bin/sp970-wfc", "bin/hysteria", "bin/sp970_uim_bridge", "bin/sp970_ims_xfrm",
                "service/service-control.sh", "service/90-sp970-wfc.sh", "web/wfc-panel.html",
                "web/wfc-panel.js", "web/native-sms.js", "web/forward-editor.js", "data/ca-roots.pem"}
    if not required <= seen:
        raise ValueError("安装包缺少运行文件，请重新下载")


def preflight(adb: Adb) -> dict:
    sdk = adb.shell("getprop ro.build.version.sdk")
    arch = adb.shell("uname -m")
    if sdk != "19" or not arch.startswith("armv7"):
        raise ValueError(f"此版本只支持已验证的 Android 4.4 / ARMv7，当前 SDK={sdk}, arch={arch}")
    adb.shell("test -f /system/vendor/lib/libqmi.so && test -x /system/xbin/busybox && test -x /system/xbin/sqlite3 && test -x /system/bin/nginx && magisk -v >/dev/null")
    if adb.shell("settings get global airplane_mode_on") != "1":
        raise ValueError("请先从原生后台开启飞行模式，保持 Wi-Fi 开启；安装器不会自动打开或关闭蜂窝射频")
    gateway = adb.shell("/system/bin/ip -4 route show default dev wlan0")
    if "default via " not in gateway:
        raise ValueError("棒子尚未通过 wlan0 连上 Wi-Fi，请先在原生后台配置 Wi-Fi")
    return {"sdk": sdk, "arch": arch, "flight_mode": 1}


def install(adb: Adb, config: dict, payload: Path, *, start=True, wait_seconds=120) -> None:
    validate(config)
    verify_payload(payload)
    print("[1/5] 检查设备、飞行模式和 Wi-Fi…")
    preflight(adb)
    original_nginx = adb.shell(f"cat {NGINX}") + "\n"
    original_html = {name: adb.shell(f"cat {WWW}/{name}") + "\n" for name in ("login.html", "sms-device.html")}
    patched = {"nginx.conf": patch_nginx(original_nginx)}
    patched.update({name: patch_native_html(text) for name, text in original_html.items()})
    stage = "/data/local/tmp/sp970-wfc-install-" + secrets.token_hex(6)
    backup = BASE + "/backups/" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    old_control = adb.shell(f"if [ -f {BASE}/service-control.sh ]; then echo yes; fi") == "yes"
    old_enabled = adb.shell(f"if [ -e {BASE}/disabled ]; then echo disabled; fi") != "disabled"
    restore_script = f"{backup}/rollback.sh"
    print("[2/5] 传输已校验安装包（不包含个人短信和历史日志）…")
    adb.shell(f"mkdir -p {stage}; chmod 700 {stage}")
    try:
        with tempfile.TemporaryDirectory(prefix="sp970-wfc-install-") as temporary:
            tmp = Path(temporary)
            for relative in ["bin/sp970-wfc", "bin/hysteria", "bin/sp970_uim_bridge", "bin/sp970_ims_xfrm",
                             "service/service-control.sh", "service/90-sp970-wfc.sh", "web/wfc-panel.html",
                             "web/wfc-panel.js", "web/native-sms.js", "web/forward-editor.js", "data/ca-roots.pem"]:
                adb.push(payload / relative, stage + "/" + Path(relative).name)
            local = {
                **patched, "network-mode": config["mode"] + "\n",
                "proxy-ipv4": config.get("proxy_ipv4", "") + "\n",
                "epdg-ipv4": config.get("epdg_ipv4", "84.246.115.151") + "\n",
                "psm-direct-tun.json": json.dumps(config.get("hysteria", {})),
            }
            for name, value in local.items():
                file = tmp / name; file.write_text(value, encoding="utf-8"); file.chmod(0o600)
                adb.push(file, stage + "/" + name)
        adb.shell(f"chmod 600 {stage}/*")
        adb.shell(f"chmod 700 {stage}/sp970_uim_bridge")
        identity = adb.shell(f"LD_LIBRARY_PATH=/system/vendor/lib:/system/lib {stage}/sp970_uim_bridge identity", timeout=45).split()
        if len(identity) != 5 or identity[0] != "OK" or len(identity[1]) != 15 or not identity[1].startswith("23487") or identity[3] != "2":
            raise ValueError("SIM 策略不是受支持的 23487 / 两位 MNC，未安装任何运行文件")
        del identity
        print("[3/5] 备份原生页面与现有程序，安装运行文件…")
        # A device-local transaction: backups include only the files we replace.
        # SMS, sessions, credentials and forwarding queue remain in place.
        files = {
            f"{BASE}/bin/sp970-wfc": "sp970-wfc", f"{BASE}/bin/hysteria": "hysteria",
            f"{BASE}/bin/sp970_uim_bridge": "sp970_uim_bridge", f"{BASE}/bin/sp970_ims_xfrm": "sp970_ims_xfrm",
            f"{BASE}/service-control.sh": "service-control.sh", BOOT: "90-sp970-wfc.sh",
            f"{BASE}/ca-roots.pem": "ca-roots.pem", NGINX: "nginx.conf",
            f"{WWW}/login.html": "login.html", f"{WWW}/sms-device.html": "sms-device.html",
            f"{WWW}/wfc-panel.html": "wfc-panel.html", f"{WWW}/wfc-panel.js": "wfc-panel.js",
            f"{WWW}/forward-editor.js": "forward-editor.js",
            f"{WWW}/wfc-native-sms.js": "native-sms.js", f"{BASE}/network-mode": "network-mode",
            f"{BASE}/proxy-ipv4": "proxy-ipv4", f"{BASE}/epdg-ipv4": "epdg-ipv4",
            f"{BASE}/psm-direct-tun.json": "psm-direct-tun.json",
        }
        if old_control:
            adb.shell(f"/system/bin/sh {BASE}/service-control.sh disable; sleep 18", timeout=35)
        commands = ["set -e", "BB=/system/xbin/busybox",
                    f"mkdir -p {BASE}/bin {BASE}/run {BASE}/log {backup} /data/adb/service.d",
                    f"chmod 700 {BASE} {BASE}/bin {BASE}/run {BASE}/log {backup}",
                    f"touch {BASE}/disabled",
                    f"test -w {WWW} || /system/bin/mount -o remount,rw /system"]
        for destination, name in files.items():
            commands += [f"if [ -f {destination} ]; then \"$BB\" cp -p {destination} {backup}/{name}; else touch {backup}/{name}.absent; fi"]
        # Save rollback before touching any payload file.
        with tempfile.TemporaryDirectory() as t:
            path = Path(t) / "rollback.sh"
            path.write_text(rollback_script(files, backup, old_control=old_control, old_enabled=old_enabled))
            adb.push(path, stage + "/rollback.sh")
        commands += [f'"$BB" cp {stage}/rollback.sh {restore_script}', f"chmod 700 {restore_script}"]
        # Retain a permanent pre-toolkit snapshot for uninstall, not a later upgrade.
        commands += [f"if [ ! -f {BASE}/uninstall-backup ]; then echo {backup} >{BASE}/uninstall-backup; fi"]
        for destination, name in files.items():
            mode = "700" if "/bin/" in destination or destination.endswith(".sh") else "644" if destination.startswith(WWW) or destination == NGINX else "600"
            commands += [f'"$BB" cp {stage}/{name} {destination}.next', f"chmod {mode} {destination}.next", f'"$BB" mv -f {destination}.next {destination}']
        commands += [f"/system/bin/sh -n {BASE}/service-control.sh", f"/system/bin/nginx -t -p /sdcard/nginx -c {NGINX}"]
        try:
            adb.shell("\n".join(commands), timeout=120)
            print("[4/5] 只读检查 SIM 策略（只显示 23487 前缀）…")
            sim = adb.shell(f"LD_LIBRARY_PATH=/system/vendor/lib:/system/lib {BASE}/bin/sp970-wfc --check-sim", timeout=45)
            if '"imsi_prefix":"23487"' not in sim:
                raise ValueError("SIM 策略不是受支持的 23487，已尝试恢复安装前配置；没有修改 SIM")
            adb.shell(reload_native_nginx())
        except Exception:
            try:
                adb.shell(f"/system/bin/sh {restore_script}", timeout=90)
            except Exception:
                print("自动回滚未完成，设备上的恢复脚本：" + restore_script)
            raise
        print("SIM 23487，MNC 长度 2，飞行模式开启。")
        if not start:
            print("已安装。启动：python3 setup.py enable")
            return
        print("[5/5] 开启自启并等待 WFC 注册；不会发送测试短信…")
        print(adb.shell(f"/system/bin/sh {BASE}/service-control.sh enable", timeout=30))
        until = time.monotonic() + wait_seconds
        while time.monotonic() < until:
            log = adb.shell(f"/system/xbin/busybox tail -n 25 {BASE}/log/wfc.log 2>/dev/null; true")
            if '"wfc_registered":true' in log:
                print("WFC 注册成功。打开 http://<棒子IP>/wfc-panel.html，使用原生后台账号登录。")
                return
            time.sleep(5)
        raise ValueError("安装已完成，但 WFC 尚未注册。运行 python3 setup.py logs 查看阶段；无需重新安装")
    finally:
        try:
            adb.shell(f"/system/xbin/busybox rm -rf {stage}")
        except (RuntimeError, subprocess.SubprocessError):
            print("连接中断，安装暂存文件仍保留在设备的私有目录：" + stage)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description="SP970-B V13 Wi-Fi Calling / SMS toolkit")
    parser.add_argument("command", nargs="?", default="wizard", choices=["wizard", "configure", "install", "status", "logs", "doctor", "enable", "disable", "uninstall"])
    parser.add_argument("--adb", default="adb")
    parser.add_argument("--adb-host", default="127.0.0.1")
    parser.add_argument("--adb-port", type=int, default=5037)
    parser.add_argument("--serial")
    parser.add_argument("--config", type=Path, default=ROOT / ".local/config.json")
    parser.add_argument("--payload", type=Path, default=ROOT)
    parser.add_argument("--no-start", action="store_true")
    parser.add_argument("--wait-seconds", type=int, default=120)
    args = parser.parse_args(argv)
    try:
        if args.command in ("wizard", "configure"):
            config = wizard(args.config)
            if args.command == "configure":
                return 0
        else:
            config = None
        if args.command in ("wizard", "install"):
            if config is None:
                config = validate(json.loads(args.config.read_text()))
            payload = args.payload
            if not (payload / "SHA256SUMS").exists() and (ROOT / "dist/payload/SHA256SUMS").exists():
                payload = ROOT / "dist/payload"
            verify_payload(payload)
            install(Adb(args), config, payload, start=not args.no_start, wait_seconds=args.wait_seconds)
            return 0
        adb = Adb(args)
        if args.command == "doctor":
            print(json.dumps(preflight(adb), ensure_ascii=False))
            print(adb.shell(f"LD_LIBRARY_PATH=/system/vendor/lib:/system/lib {BASE}/bin/sp970-wfc --check-sim", timeout=45))
        elif args.command == "logs":
            print(adb.shell(f"/system/xbin/busybox tail -n 60 {BASE}/log/supervisor.log; /system/xbin/busybox tail -n 40 {BASE}/log/wfc.log"))
        elif args.command in ("status", "enable", "disable"):
            print(adb.shell(f"/system/bin/sh {BASE}/service-control.sh {args.command}", timeout=30))
        elif args.command == "uninstall":
            backup = adb.shell(f"cat {BASE}/uninstall-backup")
            if not re.fullmatch(re.escape(BASE) + r"/backups/\d{8}T\d{6}Z", backup):
                raise ValueError("找不到受信任的卸载备份路径，未删除任何文件")
            print(adb.shell(f"/system/bin/sh {backup}/rollback.sh", timeout=120))
            print("已恢复安装前的程序和页面。短信、转发配置及本机备份保留在 " + BASE)
        return 0
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as exc:
        print("错误：" + str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
