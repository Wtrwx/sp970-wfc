# 节点订阅与 Spot 自动恢复（可选）

让一台持续在线的 Linux 主机定时恢复**一台已经存在的**英国 GCP Spot 节点；不依赖 Mac、Codex 或 ADB。棒子读取普通代理订阅，自动更新当前 Shadowsocks 节点的地址。订阅链接在面板填写，没有固定的恢复主机地址。

此功能不自动购买备用实例，也不改变 Spot 规格、计费账号或流量限制。Google 没有可用容量时仍会中断；它不能让 Spot 变成保证在线的普通实例。

## 服务器端

- Python 3 + `cryptography`，以独立低权限用户运行。
- Google API 通过 IPv4 连接，保留 HTTPS 主机名和证书验证，避免主机宣告 IPv6 但无法连通时认证超时。
- 仅为指定 VM 授予 `compute.instances.get`、`compute.instances.start`。不要授予项目 Owner / Editor / Compute Admin，不要复制个人 Google ADC。
- 用专用身份获取短期 OAuth token；凭据归 root 所有、0600，通过 systemd `LoadCredential` 注入。支持服务账号 JSON 私钥，也支持下述身份联合；项目禁止服务账号私钥时优先使用身份联合，保留组织策略。
- `gcp-uk-recovery.timer` 每约一分钟检查一次。固定实例 ID 或 Spot 属性变化就拒绝操作。
- 启动请求预先保存 UUID，不确定的响应重用该 UUID；启动未成功时至少间隔两分钟再发新请求。
- 输出标准 sing-box JSON，包含节点地址、端口、加密方式和密码；没有额外加密、签名或公钥参数。订阅文件使用随机路径，链接由用户自行保管。Google 身份凭据和 SIM 数据不进入订阅。

配置示意（按实际环境填写）：

```json
{
  "project": "YOUR_PROJECT",
  "zone": "europe-west2-a",
  "instance": "YOUR_EXISTING_VM",
  "instance_id": "YOUR_PINNED_INSTANCE_ID",
  "state_dir": "/var/lib/gcp-uk-recovery",
  "feed_file": "/var/lib/gcp-uk-recovery/public/wfc-subscription-YOUR_TOKEN.json"
}
```

将 Google 身份配置 `gcp.json` 和节点配置 `proxy.json` 放在 `/etc/gcp-uk-recovery/`，均为 root:root / 0600，通过 systemd `LoadCredential` 读取。节点配置如下，填写自己的真实节点信息：

```json
{"name":"UK-WFC","method":"2022-blake3-aes-256-gcm","password":"YOUR_NODE_KEY","port":443}
```

程序从 Google 查询当前 IP，并生成标准 sing-box `outbounds`。把 `feed_file` 和 Caddy 路由中的 `YOUR_TOKEN` 替换为自己生成的随机字符串，仅开放这个文件。`config.json` 不含 Google 私钥，但订阅路径也应私下保管。

`gcp-uk-recovery.caddy` 作为已有 HTTPS 站点的精确路径路由导入。先验证 Caddy 配置再 reload，不需要重启现有代理或短信网关。

### 无 Google 服务账号私钥的身份联合

可以在恢复主机本地生成 RSA 签名密钥，将其公钥以 JWKS 上传到一个专用 OIDC Workload Identity Federation 提供方。固定发行方、受众和 subject；只给这个单一 subject 在上述专用服务账号上授予 `roles/iam.workloadIdentityUser`，不要信任整个池，也不要给主机更广的 VM 权限。需要启用 STS 和 IAM Service Account Credentials API。

此方式的 `gcp.json` 是恢复主机自己的身份配置，包含 `type: "oidc_federation"`、`private_key_id`、RSA PEM `private_key`、`client_email`、`issuer`、`subject`、`audience` 和 `provider_audience`。最后一项使用 `//iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL/providers/PROVIDER`。私钥只留在恢复主机的 root 文件中；Google 只保存对应公钥。不要把这种配置或普通服务账号 JSON 放入仓库。

程序签发两分钟有效的身份 JWT，由 STS 验证后换取专用服务账号十分钟有效的 Compute token。`cloud-platform` 交换范围不增加 IAM 权限；最终服务账号仍只能查询、启动指定 VM。停用身份提供方或撤销该 subject 的服务账号授权可以阻止后续取令牌。详见 [Google 身份联合与上传 JWKS](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-other-providers)。

## 在棒子面板配置

原生管理服务内置订阅更新，随管理服务开机启动，不需要另外的脚本或后台密码。

1. 打开 **VPN 与代理 → 节点订阅**。
2. 粘贴自己的 HTTP / HTTPS 订阅链接，选择 1 / 5 / 15 分钟或 1 小时的更新间隔。
3. 开启自动更新并保存，或点击“立即更新”。首次安装时链接为空，不内置任何服务器。

可直接使用以下常见格式，不需要转换成专用签名格式：

- Mihomo / Clash YAML 的 `proxies` 列表（完整配置、代理提供者文件均可）。
- sing-box JSON 的 `outbounds` 列表。
- 一行一个 `ss://` 的文本或常见 Base64 编码订阅，支持 SIP002 和旧式 SS 链接。

当前同步范围是已有的 Shadowsocks 节点，支持 AES-GCM 和 SS2022。自动按加密方式与密码匹配，更新地址和端口，保留本机节点名称；不导入路由、DNS、TUN 等订阅配置。仅 TCP、需要外部插件或不支持的协议会被跳过。同一凭据对应多个不同地址时报告歧义，不随意选择。

第一次保存或明确修改订阅链接时绑定当前 SS 节点。之后手动选择其他节点或直连时跳过更新；仅调整间隔不会重新绑定。地址没变时不重启 WFC，只更新检查时间。地址变化时先备份配置，沿用现有 ePDG、分流和 WFC 开关，并检查飞行模式与 `23487` SIM 策略；不会切换飞行模式。

面板显示上次检查和实际结果，未保存输入不会被刷新覆盖。`ip-sync.json` 和状态文件为 root / 0600；管理 API 不返回节点密码或后台账号。保存订阅会清理旧版本中的签名参数及后台登录凭据。订阅内容是普通节点信息，HTTP / HTTPS 是传输协议，不要求额外密钥。

### 可选的旧独立客户端

`ip-sync/` 仍提供 ARMv7 独立客户端，与面板使用相同的订阅解析库。它使用本地管理 API，因此需要单独的 root 私有配置，其中包含 `feed_url`、`credential_sha256`、`base`、`ca_file`，以及本机后台的 `account` / `password`。

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags='-s -w' -o proxy-ip-sync ./extras/spot-recovery/ip-sync
```

此方式可配合 `91-sp970-ip-sync.sh` 使用；新面板不需要安装它。升级到原生面板时先停止旧同步进程、备份并移走该开机脚本，保留已有配置；原生服务会接管更新。二者共用文件锁，避免后台重复执行。

## 验证和停用

```sh
python3 -m unittest discover -s extras/spot-recovery -v
go test ./internal/nodesync ./extras/spot-recovery/ip-sync ./cmd/sp970-wfc
```

`proxy-ip-sync --check` 只读取并解析订阅，不改节点；`--once` 检查一次，只有匹配的节点地址发生变化才更新。不要为测试而停止真实 VM、改飞行模式或伪造生产地址。

恢复分支通过模拟云接口验证实例身份、启动幂等和重试间隔；订阅测试覆盖多种格式、特殊字符、原节点匹配、歧义拒绝及无变化时不登录后台。部署验证保持现有节点，不停止真实 VM 或更改生产 IP 来制造故障。

计划手动关闭 VM 时，先在恢复主机执行 `sudo systemctl disable --now gcp-uk-recovery.timer`，防止自动拉起。也可创建 `/var/lib/gcp-uk-recovery/paused` 暂停恢复。棒子端可以在面板关闭自动更新；该设置跨重启保留，也可以创建 `/data/adb/sp970-wfc/ip-sync-disabled` 暂停后续自动检查。手动“立即更新”仍可执行一次检查，已有 WFC 会话保持运行。

上游说明：[Spot VM 恢复](https://docs.cloud.google.com/compute/docs/instances/create-use-spot)、[资源级 IAM](https://docs.cloud.google.com/compute/docs/access/managing-access-to-resources)、[服务账号凭据](https://docs.cloud.google.com/iam/docs/service-account-creds)。
