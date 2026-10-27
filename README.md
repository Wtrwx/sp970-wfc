# SP970 WFC Toolkit

在 **SP970-B V13 原生 Android** 上，通过内置 SIM 卡槽完成 Wi-Fi Calling 注册，在棒子后台收发短信，并把新短信转发到自己的 HTTPS 服务。无需先刷 Linux。

提供 ARMv7 预编译安装包、交互安装向导、开机自启、Web 控制面板和完整源码。**你需要自己的 SIM 和可用网络/代理，项目不提供公共节点。**

[下载最新安装包](https://github.com/Wtrwx/sp970-wfc/releases/latest) · [安装与卸载](docs/INSTALL.md) · [故障排除](docs/TROUBLESHOOTING.md) · [开发与构建](docs/DEVELOPMENT.md)

## 支持范围

| 项目 | 当前状态 |
| --- | --- |
| 设备 | SP970-B V13，MSM8916 / ARMv7，Android 4.4.4 / SDK 19 |
| 权限 | 已有 root / Magisk、BusyBox、原生 Nginx 和管理后台 |
| SIM | 已验证 Lebara UK，当前 IMSI 前缀 **23487**，两位 MNC |
| 网络 | 已连上 Wi-Fi；可选棒子上的 Hysteria2 代理或已提供可用出口的路由器 |
| 功能 | 真实 ePDG / EAP-AKA / IMS 注册、短信接收、显式发送、HTTPS 转发 |
| 自启 | Magisk `service.d`；等待 Wi-Fi，避免重复启动，退出后恢复 |
| 其他卡、Android 版本和硬件 | 未验证；安装器会拒绝不符合已验证范围的设备 |

SIM 必须先在原生后台保持飞行模式、Wi-Fi 开启。工具只读取身份与进行 SIM AKA 鉴权，**不会改写 IMSI、SIM 策略或 eSIM 配置**。若已经变成 `20404`，本工具不能把它改回 `23487`。

部分原厂管理 APK 会在开机时强制开启热点，覆盖已保存的 Wi-Fi 状态。本工具不重签或替换厂商 APK；请先确认原生 Wi-Fi 能自动重连，相关修复参考 [原生 Wi-Fi 补丁](patches/native-wifi-state.patch)。已测试环境包含这项修复。USB 共享设置由原生固件管理。

## 快速开始

电脑需要 Python 3.10+ 和 [Android Platform Tools / adb](https://developer.android.com/tools/releases/platform-tools)。Windows 可用 `py -3` 替代 `python3`。

1. 下载 Releases 中的 `sp970-wfc-v0.2.0-android-armv7.zip`，解压。
2. 棒子插好 SIM，开启飞行模式，连接已保存的 Wi-Fi，通过 USB 允许 ADB/root。
3. 在解压目录运行：

```sh
python3 setup.py
```

向导会选择 ADB 设备，要求填写**自己的 Hysteria2 链接**，或选择“路由器已提供可用网络”。链接输入不回显，配置只保存到本机 `.local/config.json`，不会上传。

安装器校验文件 SHA256、只读确认 `23487` 策略、备份原生页面、安装程序并开启自启。安装过程**不会发送测试短信，也不会重启整机**。

注册成功后，浏览器打开：

```text
http://<棒子的Wi-Fi地址>/wfc-panel.html
```

使用原生管理后台的账号登录。接收的短信写入原生收件箱；发送必须由你点击按钮。首次使用请设置自己的后台密码。

## 短信转发

页面只需要开启转发、粘贴一段 **cURL 命令**、保存。现有配置会自动显示成 cURL，支持多行、请求头、JSON、表单与短信变量，无需逐项填写参数。

```sh
curl -X POST 'https://sms.example.com/v1/sms' \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  --data-urlencode 'from={{sms-from}}' \
  --data-urlencode 'body={{sms-body}}'
```

Authorization 在命令和预览中显示完整值，方便编辑和复制。其他敏感请求头、密钥变量和 URL 查询参数用星号显示，直接保存会保留真实参数。删除命令中的 Authorization 请求头会移除该鉴权。页面仅保留开关、命令框和保存按钮，保存时沿用已有成功判定和密钥变量。详见 [cURL 转发说明](docs/FORWARDING.md)。

工具只转发启用后收到的新短信，先保存到本机，再写入持久队列，失败自动重试，重复消息按稳定 ID 去重。**不会导出历史收件箱。**

原有配置仍保持原来的 HTTPS POST 表单格式：

| 字段 | 内容 |
| --- | --- |
| `from` | 发件号码 |
| `time` | UTC / RFC3339 时间 |
| `body` | JSON 转义后的短信正文，再进行表单编码 |
| `ufi_escaped` | 固定 `1` |

原接收器成功时返回 2xx 和 `{"accepted":true}`。`/v1/sms` 配套的只读连接检查访问 `/v1/status`，不会推送测试短信。可直接部署仓库内的 [可选 ntfy 网关](extras/sms-gateway/README.md)。

## 常用命令

```sh
python3 setup.py status
python3 setup.py doctor
python3 setup.py logs
python3 setup.py disable
python3 setup.py enable
python3 setup.py uninstall
```

多个设备或使用远程 ADB server 时：

```sh
python3 setup.py status --serial YOUR_SERIAL --adb-host 127.0.0.1 --adb-port 15037
```

## 注册与代理

当前采用每轮最多 25 分钟的独立进程，结束后由服务重新注册；运营商返回更短有效期时会提前结束。每次都重新检查飞行模式和 SIM 策略。该周期是本工具的实现选择，不是运营商统一规定。

使用棒子上的代理时，**代理需要持续可用**，收发短信也通过隧道。更换节点后重新运行配置和安装向导；不需要维持同一个出口 IP。原生拨号、语音通话、iPhone 系统 WFC 转接均不在当前功能范围。

## 验证边界

原型已在真实 SP970-B 上验证注册、短信收发、原生收件箱、去重、重连和代理进程恢复。公开安装器另有配置、补丁、回滚和打包测试；新的安装包尚无跨设备、长时间运行或断电启动验证，不承诺所有 V13 固件都可直接兼容。问题反馈请说明固件、SDK、失败阶段，勿贴完整 IMSI、ICCID、验证码或节点密码。

## 开源与致谢

项目代码采用 **AGPL-3.0-or-later**。WFC 库固定为公开的 `vowifi-go.v112` AGPL 历史快照；Hysteria 固定为 app/v2.6.5，并附 ARM32 TUN 补丁。第三方保留各自许可证，详见 [NOTICE](NOTICE)。仓库和安装包不包含厂商 APK、平台签名私钥、个人节点账号或短信数据。
