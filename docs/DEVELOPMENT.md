# 开发与构建

运行代码与当前设备配置分离。仓库不追踪 `.local/`、`.env`、日志、APK、IPCC 或签名私钥。

## 本机测试

需要 Go 1.26.4+，推荐 **1.26.7**；Python 3.10+。

```sh
go mod download
go test -race -count=1 ./cmd/sp970-wfc
go vet ./cmd/sp970-wfc
python3 -m unittest discover -s tests -v
python3 -m unittest discover -s extras/sms-gateway -v
sh -n service/service-control.sh service/90-sp970-wfc.sh
```

协议测试使用本机 socket / TLS 服务，不连接运营商，不发真实短信。包含 IKE 提议校验、EAP-AKA、COOKIE 重试、NAT-T、IMS 并发消息、Unicode / 表单编码、持久队列、重试去重、掩码参数保留等边界。

## 构建安装包

- Android NDK r27c，版本 `27.2.12479018`。
- WFC：Go 1.26.7。
- 固定 Hysteria 代码：Go **1.25.6**，避免不同 Go 版本对 QUIC 实现的影响。

```sh
python3 tools/build.py --go /path/to/go1.26.7 \
  --hysteria-go /path/to/go1.25.6 \
  --ndk /path/to/android-ndk-r27c --version v0.2.0 --test
```

产物在 `dist/`，包含 SHA256SUMS、独立二进制、安装工具、WebUI、许可证和对应源代码。NDK 以 API 21 头文件编译，使用 KitKat 已验证的 ARM32 vendor ABI、SysV ELF hash 和无分离 rosegment 链接参数；不能由 API 号推断已支持所有 Android 版本。

## 结构

| 路径 | 职责 |
| --- | --- |
| `cmd/sp970-wfc/` | 一轮 WFC / IMS 短信会话、私有 API 与转发队列 |
| `native/` | 内置卡槽的只读 QMI UIM / AKA 适配、精确 XFRM 操作 |
| `service/` | 单实例、开机等待、进程恢复、受限路由清理 |
| `tools/` | 本机配置、安装事务、备份回滚与构建 |
| `web/` | 自有面板及原生后台的小型注入脚本 |
| `third_party/` | 固定的公开 AGPL WFC 快照与 Hysteria 对应源码 |
| `extras/sms-gateway/` | 可选的 HTTPS → 持久队列 → ntfy 接收服务 |

运行器一轮结束后退出，由 supervisor 启动下一轮。不要把原型的同进程无限重连重新加回来；旧内核的阻塞 TUN 读可能保留接口引用。

改 SIM 边界时保留 flight-mode、23487 / 两位 MNC 和身份一致性保护。敏感 AKA 材料只能在内存与 stdin 中传递，不能放到 argv、日志或文件中。新增运营商支持应建立明确配置和测试，不能简单移除前缀检查后宣称兼容。
