# SP970 原厂网络状态记忆补丁

原厂 `com.pingbao` 开机时会强制开启热点及 USB 网络共享。本补丁将这两个常量替换为已保存状态，并恢复上次的 Wi-Fi 开关。Tailscale 由 Magisk 启动脚本单独开启，不参与状态记忆。

`NetworkMemory.java` 监听系统 Wi-Fi、热点、网络共享广播，只在开关进入确定状态后同步保存到应用私有的 `sp970_network_memory` SharedPreferences。启动恢复期间忽略中间广播，关机广播后停止保存，避免把关机过程中的断网误记为用户设置。原厂热点状态文件 `/cache/wifi_state` 同时更新，供原厂后台服务读取。

补丁基于原厂 APK SHA-256 `0f2fdd45c845cc5728d1d8fb90ec38498ef6a9d8cdfc7c5b36f30a2cb19b5a7d`。只修改 `MainActivity.initWifiAp`、`MainActivity.onCreate` 和 `ZyScreenService.setWifiApEnabled`，加入 `NetworkMemory` 类；原清单、权限、资源及其他业务代码保持不变。

复现构建流程：

1. 使用 Apktool 3.0.1 的 `d --no-res` 解码匹配版本的原 APK，应用上一级的 `native-network-memory.patch`。
2. 使用 Android SDK 的 `android.jar` 编译本目录 Java 源码，Java 目标版本 8；用 D8 `--min-api 19` 转为 DEX。
3. 将生成的 `NetworkMemory` 及内部类反汇编为 smali，合并到原 APK 的 smali 目录。用 Apktool `b --no-apk` 重建 DEX。
4. 复制原 APK 所有非签名条目，只替换 `classes.dex`，进行 zipalign，再用设备认可的原厂平台证书签名。签名私钥不在本仓库中。
5. 校验 Android 4.4 支持的 v1 签名及证书身份，备份原 APK。本设备的 `/data/app` 更新包会在重启后消失，最终通过 Magisk 22.1 模块持久加载：将签名 APK 放到 `/data/adb/modules/sp970-network-memory/system/app/pingbao.apk`，并加入 `module.prop`。模块在系统扫描应用前挂载补丁，原系统 APK 保留。
6. 模块目录使用 0755，APK 和 `module.prop` 使用 0644、root 所有者。使用 BusyBox 的 `chown`、`chmod` 设置权限；使用 Magisk root 上下文逐个为模块的 `system`、`system/app` 和 APK 设置 `u:object_r:system_file:s0` 标签。此固件的 Magisk 自动挂载后曾出现异常所有者和权限，因此模块须包含空文件 `skip_mount`，并将本目录的 `post-fs-data.sh` 以 0755 安装到模块根目录，在系统扫描应用前修复元数据并直接绑定 APK。
7. 重启后核对 `/system/app/pingbao.apk` 的 SHA-256、原厂程序状态记忆日志及三项实际状态。需要回退时，在模块目录创建 `disable` 文件后重启，可恢复原厂 APK。

首次安装可以从当前状态初始化记忆。此次设备部署将测试前的 Wi-Fi 开、热点关、USB 共享关状态写入初始记忆；以后保存用户实际切换后的状态，不固定为这组值。
