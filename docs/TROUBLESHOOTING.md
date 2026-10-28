# 故障排除

先运行 `python3 setup.py status` 和 `python3 setup.py doctor`。日志只贴相关失败阶段，避免公开原始短信、完整 SIM 身份和配置文件。

| 阶段或现象 | 检查 |
| --- | --- |
| 没有 ADB 设备 | USB 调试授权、线缆和 ADB server 地址；设备不需要关闭飞行模式 |
| `waiting: Android boot` | 等待 Android 框架启动；服务不会阻塞系统开机 |
| `waiting: Wi-Fi gateway` | 从原生后台连 Wi-Fi，确认保存密码；部分固件的强制热点行为需要先修复 |
| `waiting: UK proxy tunnel` | 自己的节点是否在线、密码/SNI/pin、UDP 可用性、Wi-Fi 出口是否有网 |
| `paused: flight mode is off` | 从原生后台开启飞行模式；服务不会自动开启蜂窝射频 |
| `flight-mode guard failed` | SIM 读取被保护条件拒绝，先检查飞行模式状态 |
| SIM 策略不是 `23487` | 此版本不支持当前策略，不会尝试伪造身份或改写 SIM |
| `temporary IMS interface already exists` | 检查是否同时运行了其他原型程序；服务使用一轮一个进程释放旧内核的 TUN 读引用 |
| `IKE response timeout` | ePDG UDP 4500、代理 UDP 支持与出口；只发任意 UDP 到 500/4500 后没回应不能证明端口被封 |
| AKA 失败 | SIM 是否有效、是否兼容此 vendor QMI ABI、运营商是否允许当前服务 |
| REGISTER 401 | 正常鉴权挑战，后续应有 AKA 和 REGISTER 200；单条 401 不代表失败 |
| REGISTER 403 | 运营商拒绝；检查 SIM 策略和设备兼容性，不能靠强制 UI 开关保证解决 |
| DNS 或 HTTPS 错误 | 节点、实际时间、当前 Wi-Fi 的 DNS 和证书包；静态运行器优先读取 Android 的网络 DNS，无可用属性时才回退至 1.1.1.1；不要关闭 HTTPS 验证来绕过 |
| 面板 502 / 暂不可用 | 检查 `admin.log` 与 `nginx.log`；管理 API 独立于 IMS，WFC 未注册时仍应能登录和查看短信 |
| 登录失败 | 使用原生管理后台账号；检查该固件是否采用 `com.pingbao` 账号数据库 |
| 收件箱有短信，转发为 0 | 仅转发开启之后的新短信；历史收件箱和旧运营商重投不会回放 |
| 转发失败 | 检查自己的 HTTPS 地址、Authorization 和服务返回值；面板检查连接不发短信 |

服务仅管理自身 PID、`hypsm0`、`spwfc0` 和 metric 27027 的代理路由，不执行全局路由或 XFRM flush。检测到另一个程序占用接口时会等待，不会直接删除它。

短信收到后会先落盘，再处理 RP-ACK。运营商 RP-ACK 互通尚需更多实网验证，可能出现运营商重投；原生收件与转发队列均做去重。不能把当前版本当作已经过运营商认证的短信网关。
