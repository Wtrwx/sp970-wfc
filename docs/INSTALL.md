# 安装、升级和卸载

## 安装前

- 确认型号和系统为支持表中的 SP970-B V13 / SDK 19。
- 保持 SIM 在卡槽内，飞行模式开启，Wi-Fi 已连接。不要为了安装而关闭飞行模式。
- 电脑 `adb devices -l` 能看到已授权设备；普通 ADB shell 非 root 时，允许 Magisk 的 `su` 请求。
- 解压 Release 安装包到单独目录，避免把个人配置放进 Git 仓库。

运行 `python3 setup.py`。默认 ADB server 为 `127.0.0.1:5037`；`--serial` 可指定设备。

## 分步配置

```sh
python3 setup.py configure
python3 setup.py install --config .local/config.json
```

也可复制 `examples/config.direct.json` 或 `examples/config.hysteria2.json` 到 `.local/config.json`。示例 IP `192.0.2.10` 和域名 `uk.example.com` 是保留示例，必须替换。

Hysteria2 导入器支持密码、SNI、证书 SHA256 pin 和 salamander 混淆。`insecure=1` 必须同时提供证书 pin；不要把证书哈希填成 Chrome 指纹。TUN MTU 固定为已验证的 **1500**，内部 IMS 接口 MTU 为 **1280**。

安装但暂不开启运行：

```sh
python3 setup.py install --no-start
python3 setup.py enable
```

安装不触发测试短信；注册等待超时后保留已安装文件，可用 `logs` 排查，不必循环重装。

## 文件位置

| 位置 | 内容 |
| --- | --- |
| `/data/adb/sp970-wfc/bin/` | 自有运行程序和只读 SIM / XFRM 适配器 |
| `/data/adb/sp970-wfc/psm-direct-tun.json` | 私有节点配置，0600 |
| `/data/adb/sp970-wfc/forward/` | 转发配置和持久任务队列，目录 0700 |
| `/data/adb/sp970-wfc/sms/` | 私有 IMS 收件记录 |
| `/data/adb/sp970-wfc/log/` | 当前和上一轮日志 |
| `/data/adb/sp970-wfc/backups/` | 本机原生页面及安装文件备份 |
| `/data/adb/service.d/90-sp970-wfc.sh` | Magisk 开机入口 |
| `/system/var/www/wfc-panel.html` | 独立控制面板 |

升级重新运行新安装包的安装命令。会保留短信、会话令牌、转发配置和任务队列，运行文件替换前停止自有服务。每次安装有独立备份，首次安装备份用于卸载。

## 卸载

```sh
python3 setup.py uninstall
```

恢复首次安装前的原生 Nginx 配置和页面，移除新增运行文件与开机入口；若安装前已有原型程序，则恢复其原文件与原启用状态。**短信、转发配置、队列和本机备份保留**，可在确认不再需要后自行删除私有数据目录。

失败时，安装器会显示本机恢复脚本的准确位置。通过 ADB 执行该 `rollback.sh` 即可恢复对应安装前的文件，不需要刷机。

原生页面与 Nginx 目录需要可写。不同固件可能需要先由设备所有者将 `/system` 以可写方式挂载；本工具不会关闭 SELinux 或签名校验。
