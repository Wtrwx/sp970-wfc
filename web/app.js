/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function () {
  "use strict";

  var LANG_KEY = "sp970_lang";
  var SESSION_KEY = "wfc_session";
  var ACCOUNT_KEY = "wfc_account";

  var I18N = {
    zh: {
      pageTitleApp: "设备设置",
      pageTitleLogin: "登录 - 设备设置",
      common: {
        langSwitch: "语言切换",
        connecting: "正在连接…",
        openMenu: "打开菜单",
        closeMenu: "关闭菜单",
        close: "关闭",
        refresh: "刷新",
        show: "显示",
        hide: "隐藏",
        cancel: "取消",
        confirm: "确认",
        save: "保存",
        saving: "保存中…",
        busyWait: "操作正在执行，请稍候",
        serviceUnavailable: "设备服务暂不可用，请稍后刷新",
        relogin: "登录已过期，请重新登录",
        requestFailed: "请求失败",
        enabled: "已开启",
        disabled: "已关闭",
        on: "已开启",
        off: "已关闭",
        connected: "已连接",
        disconnected: "未连接"
      },
      login: {
        title: "SP970 设备管理",
        account: "账号",
        password: "密码",
        showPassword: "显示密码",
        hidePassword: "隐藏密码",
        submit: "登录",
        submitting: "正在登录…",
        emptyFields: "请输入账号和密码"
      },
      account: {
        signedInAs: "当前账号",
        logout: "退出登录"
      },
      nav: {
        ariaLabel: "设置导航",
        settingsBrand: "设置",
        groupConn: "连接",
        groupSystem: "系统",
        connect: "连接与开关",
        wifi: "WLAN",
        hotspot: "个人热点",
        cellular: "移动网络与 eSIM",
        vpn: "VPN 与代理",
        sms: "信息",
        about: "关于本机"
      },
      views: {
        connect: "连接与开关",
        wifi: "WLAN",
        hotspot: "个人热点",
        cellular: "移动网络与 eSIM",
        vpn: "VPN 与代理",
        sms: "信息",
        about: "关于本机"
      },
      supervisor: {
        stopped: "已停止",
        boot: "等待启动",
        flightOff: "飞行模式关闭",
        wifiGateway: "等待网络",
        proxyTunnel: "等待代理",
        wfcProcess: "正在启动",
        running: "运行中",
        proxyOwner: "网卡占用",
        imsOwner: "IMS 占用",
        assetsError: "组件异常"
      },
      switches: {
        flight: "飞行模式",
        wifi: "WLAN",
        hotspot: "个人热点",
        hotspot_broadcast: "热点广播",
        usb_tether: "USB 网络共享",
        mobile_data: "移动数据",
        data_roaming: "数据漫游",
        wfc: "Wi-Fi 通话",
        tailscale: "Tailscale"
      },
      wifi: {
        otherBtn: "其他网络…",
        scan: "扫描",
        scanning: "扫描中…",
        turnOn: "开启 WLAN",
        offText: "WLAN 已关闭",
        offHotspotText: "个人热点已开启，WLAN 已关闭",
        ssid: "名称",
        security: "安全性",
        wpa2: "WPA / WPA2",
        open: "无",
        pwdPlaceholder: "输入密码",
        join: "加入",
        joining: "加入中…",
        disconnect: "断开",
        forget: "移除",
        groupCurrent: "当前网络",
        groupSaved: "已保存网络",
        groupOther: "其他网络",
        noNearby: "附近暂无其他网络",
        obtainingIp: "正在获取地址…",
        unsupported: "不支持",
        disconnectedNotice: "已断开当前 WLAN",
        forgottenNotice: "已移除该网络",
        connectStarted: "正在加入网络…",
        forgetTitle: "移除网络？",
        forgetBodyFmt: "设备将不再自动加入“{ssid}”。",
        forgetBtn: "移除"
      },
      hotspot: {
        ssid: "名称",
        wpa2: "WPA2 个人",
        password: "热点密码",
        pwdPlaceholder: "8–63 位字符",
        clientsFmt: "{n} 台设备",
        savedNotice: "个人热点设置已保存"
      },
      cellular: {
        wfcSection: "Wi-Fi 通话",
        simPolicy: "SIM 卡",
        epdg: "ePDG 服务器",
        epdgSaved: "ePDG 服务器已更新",
        wfcRegistered: "已注册",
        wfcNotRegistered: "未注册",
        wfcPaused: "已暂停",
        simNotReady: "未就绪",
        simAbsent: "无 SIM 卡",
        simLocked: "已锁定",
        simNonPolicy: "非 23487"
      },
      euicc: {
        sectionTitle: "eSIM",
        addBtn: "添加 eSIM",
        codeLabel: "激活码",
        qrScan: "扫码添加",
        qrTitle: "扫码添加 eSIM",
        qrCamera: "打开相机",
        qrImage: "选择二维码图片",
        qrPhotoImage: "拍照或选择二维码图片",
        qrChoose: "打开相机扫码，或选择二维码图片。",
        qrChoosePhoto: "拍照或选择相册里的 eSIM 二维码。",
        qrPrivacy: "二维码仅在浏览器内识别，确认后才会安装。",
        qrCameraOpening: "正在打开相机…",
        qrAim: "将 eSIM 二维码放入取景框。",
        qrPaused: "相机已暂停，可重新打开或选择图片。",
        qrCameraError: "无法使用相机，请选择二维码图片。",
        qrImageType: "请选择 20 MB 以内的二维码图片。",
        qrReading: "正在识别二维码…",
        qrNotFound: "未识别到二维码，请选择更清晰的图片。",
        qrNotActivation: "这不是 eSIM 激活码二维码，请重新选择。",
        qrReadError: "无法识别图片，请换一张图片或手动填写激活码。",
        qrLoadError: "扫码组件加载失败，请刷新后重试。",
        qrRecognized: "二维码已识别，请确认后安装。",
        nicknameOptional: "名称（可选）",
        confirmCodeOptional: "确认码（可选）",
        downloadBtn: "下载并安装",
        downloadingBtn: "正在安装…",
        codeRequired: "请填写 LPA 激活码",
        readingProgress: "正在读取 eSIM…",
        switchingProgressFmt: "正在切换“{name}”…",
        disablingProgressFmt: "正在停用“{name}”…",
        renamingProgress: "正在保存名称…",
        deletingProgressFmt: "正在删除“{name}”…",
        downloadingProgress: "正在下载并安装 eSIM…",
        empty: "暂无 eSIM 配置",
        stateActive: "已启用",
        enableBtn: "启用",
        disableBtn: "停用",
        renameBtn: "改名",
        deleteBtn: "删除",
        enableTitle: "启用 eSIM？",
        enableBodyFmt: "启用“{name}”（{iccid}）？",
        disableTitle: "停用 eSIM？",
        disableBodyFmt: "停用“{name}”后将暂停当前移动卡连接。",
        renameTitle: "修改配置名称",
        renameBodyFmt: "ICCID：{iccid}",
        renamePlaceholder: "输入名称（留空恢复默认）",
        deleteTitle: "删除 eSIM？",
        deleteBodyFmt: "永久从芯片删除“{name}”（{iccid}），此操作不可恢复。",
        switchedDefault: "eSIM 已切换",
        disabledNotice: "eSIM 已停用",
        renamedNotice: "配置名称已更新",
        deletedNotice: "eSIM 已删除",
        downloadedNotice: "eSIM 已下载并安装"
      },
      vpn: {
        sshTitle: "普通 SSH",
        sshLogin: "登录方式",
        sshAuth: "root · SSH 公钥",
        sshPort: "已开启 · 端口 22",
        sshStarting: "正在启动…",
        tsAddress: "节点地址",
        tsNode: "设备与网络",
        openLoginUrl: "打开登录页",
        tsLogin: "登录",
        tsLoggingIn: "正在获取…",
        tsUseKey: "使用 Auth Key",
        tsLogout: "退出登录",
        tsConnected: "已连接",
        tsNeedsLogin: "待登录",
        tsStarting: "正在启动…",
        tsNotInstalled: "未安装",
        tsSSHNeedsTailscale: "请先开启 Tailscale",
        tsSSHUnavailable: "无法读取状态",
        tsAuthReady: "请打开链接完成 Tailscale 登录",
        tsLoggedIn: "Tailscale 已连接",
        tsLoggedOut: "已退出 Tailscale 登录",
        tsLogoutTitle: "退出 Tailscale？",
        tsLogoutBody: "退出后需重新验证账号才能连接 Tailnet。",
        tsKeyRequired: "请输入 Auth Key",
        proxyTitle: "网络代理",
        modeDirect: "直连",
        modeProxy: "代理",
        nodeLabel: "代理节点",
        wfcOnly: "仅代理 Wi-Fi Calling",
        wfcOnlyHint: "UDP 500/4500 走代理，其余直连",
        ipSyncTitle: "节点订阅",
        ipSyncAuto: "自动更新节点地址",
        ipSyncSubscription: "订阅链接",
        ipSyncHint: "支持 Mihomo YAML、sing-box JSON 和 SS 链接订阅，自动匹配当前 SS 节点。",
        ipSyncInterval: "更新间隔",
        ipSyncMinute: "1 分钟",
        ipSyncFiveMinutes: "5 分钟",
        ipSyncFifteenMinutes: "15 分钟",
        ipSyncHour: "1 小时",
        ipSyncLastCheck: "上次检查",
        ipSyncCheck: "立即更新",
        ipSyncSaved: "节点订阅已保存",
        ipSyncChecked: "已完成订阅检查",
        ipSyncStates: { unconfigured: "未配置", paused: "已暂停", waiting: "等待检查", unchanged: "地址未变化", updated: "IP 已更新", available: "发现新地址", skipped: "当前节点不匹配", error: "更新失败", stale: "检查未完成" },
        reset: "还原",
        savedNotice: "网络代理设置已保存"
      },
      confirmations: {
        toggleSaved: "设置已更新",
        flightOffTitle: "关闭飞行模式？",
        flightOffBody: "关闭飞行模式将开启蜂窝射频，并暂停 Wi-Fi 通话。",
        turnOffBtn: "关闭",
        usbOffTitle: "关闭 USB 网络共享？",
        usbOffBody: "若当前通过 USB 访问管理页，关闭后连接将断开。",
        hotspotOffTitle: "关闭个人热点？",
        hotspotOffBody: "当前连接此热点的设备将断开网络。",
        wifiOnHotspotTitle: "开启 WLAN？",
        wifiOnHotspotBody: "开启 WLAN 将自动关闭个人热点。",
        turnOnBtn: "开启",
        hotspotOnWifiTitle: "开启个人热点？",
        hotspotOnWifiBody: "开启个人热点将自动断开当前 WLAN。"
      },
      sms: {
        inbox: "收件箱",
        sent: "已发送",
        archive: "已归档",
        forwardTab: "短信转发",
        composeOpen: "写短信",
        composeTitle: "新信息",
        recipientLabel: "收件人",
        messageLabel: "内容",
        sendBtn: "发送",
        sendingBtn: "发送中…",
        sentNotice: "短信已发送",
        emptyInbox: "收件箱为空",
        emptySent: "暂无已发送短信",
        emptyArchive: "暂无归档短信",
        backToList: "返回",
        dirIncoming: "收件",
        dirOutgoing: "已发送",
        tagUnread: "未读",
        tagArchived: "已归档",
        reply: "回复",
        markRead: "标为已读",
        archiveBtn: "归档",
        deleteBtn: "删除",
        deleteTitle: "删除这条短信？",
        deleteBody: "将从设备短信中删除此记录，面板内无法撤销。",
        deletedNotice: "短信已删除",
        restoreInbox: "移回收件箱",
        restoreSent: "移回已发送"
      },
      device: {
        model: "型号",
        android: "Android 版本",
        firmware: "固件版本",
        sim: "SIM 卡",
        wlanIp: "WLAN / 热点地址",
        usbIp: "USB 地址",
        tsIp: "Tailscale 地址",
        uptime: "运行时间",
        uptimeFmt: "{h} 小时 {m} 分钟",
        time: "系统时间"
      }
    },

    en: {
      pageTitleApp: "Settings",
      pageTitleLogin: "Sign In - Device Settings",
      common: {
        langSwitch: "Language selector",
        connecting: "Connecting…",
        openMenu: "Open menu",
        closeMenu: "Close menu",
        close: "Dismiss",
        refresh: "Refresh",
        show: "Show",
        hide: "Hide",
        cancel: "Cancel",
        confirm: "Confirm",
        save: "Save",
        saving: "Saving…",
        busyWait: "Operation in progress, please wait",
        serviceUnavailable: "Service temporarily unavailable",
        relogin: "Session expired, please sign in again",
        requestFailed: "Request failed",
        enabled: "On",
        disabled: "Off",
        on: "On",
        off: "Off",
        connected: "Connected",
        disconnected: "Not Connected"
      },
      login: {
        title: "SP970 Admin",
        account: "Username",
        password: "Password",
        showPassword: "Show password",
        hidePassword: "Hide password",
        submit: "Sign In",
        submitting: "Signing in…",
        emptyFields: "Enter username and password"
      },
      account: {
        signedInAs: "Signed in as",
        logout: "Sign out"
      },
      nav: {
        ariaLabel: "Settings navigation",
        settingsBrand: "Settings",
        groupConn: "Connectivity",
        groupSystem: "System",
        connect: "Connectivity",
        wifi: "Wi-Fi",
        hotspot: "Personal Hotspot",
        cellular: "Mobile & eSIM",
        vpn: "VPN & Proxy",
        sms: "Messages",
        about: "About"
      },
      views: {
        connect: "Connectivity",
        wifi: "Wi-Fi",
        hotspot: "Personal Hotspot",
        cellular: "Mobile & eSIM",
        vpn: "VPN & Proxy",
        sms: "Messages",
        about: "About"
      },
      supervisor: {
        stopped: "Stopped",
        boot: "Booting",
        flightOff: "Airplane Mode off",
        wifiGateway: "Waiting for Wi-Fi",
        proxyTunnel: "Waiting for proxy",
        wfcProcess: "Starting",
        running: "Running",
        proxyOwner: "Interface busy",
        imsOwner: "IMS busy",
        assetsError: "Asset error"
      },
      switches: {
        flight: "Airplane Mode",
        wifi: "Wi-Fi",
        hotspot: "Personal Hotspot",
        hotspot_broadcast: "SSID Broadcast",
        usb_tether: "USB Tethering",
        mobile_data: "Mobile Data",
        data_roaming: "Data Roaming",
        wfc: "Wi-Fi Calling",
        tailscale: "Tailscale"
      },
      wifi: {
        otherBtn: "Other…",
        scan: "Scan",
        scanning: "Scanning…",
        turnOn: "Turn On Wi-Fi",
        offText: "Wi-Fi is Off",
        offHotspotText: "Personal Hotspot is on, Wi-Fi is off",
        ssid: "Name",
        security: "Security",
        wpa2: "WPA / WPA2",
        open: "None",
        pwdPlaceholder: "Password",
        join: "Join",
        joining: "Joining…",
        disconnect: "Disconnect",
        forget: "Forget",
        groupCurrent: "Current Network",
        groupSaved: "Saved Networks",
        groupOther: "Other Networks",
        noNearby: "No other networks found",
        obtainingIp: "Obtaining IP…",
        unsupported: "Unsupported",
        disconnectedNotice: "Disconnected from Wi-Fi",
        forgottenNotice: "Network removed",
        connectStarted: "Joining Wi-Fi…",
        forgetTitle: "Forget Network?",
        forgetBodyFmt: "This device will no longer automatically join \"{ssid}\".",
        forgetBtn: "Forget"
      },
      hotspot: {
        ssid: "Name",
        wpa2: "WPA2 Personal",
        password: "Wi-Fi Password",
        pwdPlaceholder: "8–63 characters",
        clientsFmt: "{n} connected",
        savedNotice: "Hotspot settings saved"
      },
      cellular: {
        wfcSection: "Wi-Fi Calling",
        simPolicy: "SIM",
        epdg: "ePDG Server",
        epdgSaved: "ePDG server updated",
        wfcRegistered: "Registered",
        wfcNotRegistered: "Not Registered",
        wfcPaused: "Paused",
        simNotReady: "Not Ready",
        simAbsent: "No SIM",
        simLocked: "SIM Locked",
        simNonPolicy: "Non-23487"
      },
      euicc: {
        sectionTitle: "eSIM",
        addBtn: "Add eSIM",
        codeLabel: "Activation Code",
        qrScan: "Scan QR Code",
        qrTitle: "Scan eSIM QR Code",
        qrCamera: "Open Camera",
        qrImage: "Choose QR Image",
        qrPhotoImage: "Take Photo or Choose QR Image",
        qrChoose: "Open the camera or choose a QR code image.",
        qrChoosePhoto: "Take a photo or choose an eSIM QR code from your photos.",
        qrPrivacy: "QR codes are decoded in your browser. Installation requires confirmation.",
        qrCameraOpening: "Opening camera…",
        qrAim: "Point the camera at your eSIM QR code.",
        qrPaused: "Camera paused. Reopen it or choose an image.",
        qrCameraError: "Camera unavailable. Please choose a QR code image.",
        qrImageType: "Choose a QR code image under 20 MB.",
        qrReading: "Reading QR code…",
        qrNotFound: "No QR code found. Please choose a clearer image.",
        qrNotActivation: "This QR code is not an eSIM activation code.",
        qrReadError: "Unable to read the image. Try another image or enter the code manually.",
        qrLoadError: "Scanner failed to load. Refresh and try again.",
        qrRecognized: "QR code recognized. Review it before installing.",
        nicknameOptional: "Label (Optional)",
        confirmCodeOptional: "Confirmation Code (Optional)",
        downloadBtn: "Download & Install",
        downloadingBtn: "Installing…",
        codeRequired: "Enter an LPA activation code",
        readingProgress: "Reading eSIM profiles…",
        switchingProgressFmt: "Enabling \"{name}\"…",
        disablingProgressFmt: "Disabling \"{name}\"…",
        renamingProgress: "Updating profile label…",
        deletingProgressFmt: "Deleting \"{name}\"…",
        downloadingProgress: "Downloading & installing eSIM…",
        empty: "No eSIM profiles",
        stateActive: "Active",
        enableBtn: "Enable",
        disableBtn: "Disable",
        renameBtn: "Rename",
        deleteBtn: "Delete",
        enableTitle: "Enable eSIM?",
        enableBodyFmt: "Enable \"{name}\" ({iccid})?",
        disableTitle: "Disable eSIM?",
        disableBodyFmt: "Disabling \"{name}\" will disconnect the active SIM profile.",
        renameTitle: "Rename Profile",
        renameBodyFmt: "ICCID: {iccid}",
        renamePlaceholder: "Profile name (leave blank to clear)",
        deleteTitle: "Delete eSIM?",
        deleteBodyFmt: "Permanently delete \"{name}\" ({iccid}) from this eUICC? This cannot be undone.",
        switchedDefault: "eSIM profile enabled",
        disabledNotice: "eSIM profile disabled",
        renamedNotice: "Profile name updated",
        deletedNotice: "eSIM profile deleted",
        downloadedNotice: "eSIM downloaded and installed"
      },
      vpn: {
        sshTitle: "SSH",
        sshLogin: "Sign-in Method",
        sshAuth: "root · SSH Public Key",
        sshPort: "On · Port 22",
        sshStarting: "Starting…",
        tsAddress: "Address",
        tsNode: "Node & Tailnet",
        openLoginUrl: "Open Login Page",
        tsLogin: "Sign In",
        tsLoggingIn: "Starting…",
        tsUseKey: "Use Auth Key",
        tsLogout: "Sign Out",
        tsConnected: "Connected",
        tsNeedsLogin: "Needs Login",
        tsStarting: "Starting…",
        tsNotInstalled: "Not Installed",
        tsSSHNeedsTailscale: "Turn on Tailscale first",
        tsSSHUnavailable: "Status unavailable",
        tsAuthReady: "Open the login link to authorize Tailscale",
        tsLoggedIn: "Tailscale connected",
        tsLoggedOut: "Signed out of Tailscale",
        tsLogoutTitle: "Sign Out of Tailscale?",
        tsLogoutBody: "You will need to re-authenticate to reconnect this device.",
        tsKeyRequired: "Enter an Auth Key",
        proxyTitle: "Proxy",
        modeDirect: "Direct",
        modeProxy: "Proxy",
        nodeLabel: "Proxy Node",
        wfcOnly: "Proxy Wi-Fi Calling Only",
        wfcOnlyHint: "Proxy UDP 500/4500; use the local network for other traffic",
        ipSyncTitle: "Node Subscription",
        ipSyncAuto: "Update Node Address Automatically",
        ipSyncSubscription: "Subscription URL",
        ipSyncHint: "Supports Mihomo YAML, sing-box JSON and SS link subscriptions. Matches the current SS node automatically.",
        ipSyncInterval: "Update Interval",
        ipSyncMinute: "1 minute",
        ipSyncFiveMinutes: "5 minutes",
        ipSyncFifteenMinutes: "15 minutes",
        ipSyncHour: "1 hour",
        ipSyncLastCheck: "Last Checked",
        ipSyncCheck: "Update Now",
        ipSyncSaved: "Node subscription saved",
        ipSyncChecked: "Subscription check completed",
        ipSyncStates: { unconfigured: "Not configured", paused: "Paused", waiting: "Waiting", unchanged: "Address unchanged", updated: "IP updated", available: "New address available", skipped: "Different node selected", error: "Update failed", stale: "Check overdue" },
        reset: "Revert",
        savedNotice: "Proxy configuration saved"
      },
      confirmations: {
        toggleSaved: "Setting updated",
        flightOffTitle: "Turn Off Airplane Mode?",
        flightOffBody: "This turns on the cellular radio and pauses Wi-Fi Calling.",
        turnOffBtn: "Turn Off",
        usbOffTitle: "Turn Off USB Tethering?",
        usbOffBody: "If you are accessing this page over USB, your connection will drop.",
        hotspotOffTitle: "Turn Off Personal Hotspot?",
        hotspotOffBody: "Connected Wi-Fi clients will disconnect.",
        wifiOnHotspotTitle: "Turn On Wi-Fi?",
        wifiOnHotspotBody: "Turning on Wi-Fi will turn off Personal Hotspot.",
        turnOnBtn: "Turn On",
        hotspotOnWifiTitle: "Turn On Personal Hotspot?",
        hotspotOnWifiBody: "Turning on Personal Hotspot will disconnect Wi-Fi."
      },
      sms: {
        inbox: "Inbox",
        sent: "Sent",
        archive: "Archived",
        forwardTab: "Forwarding",
        composeOpen: "Compose",
        composeTitle: "New Message",
        recipientLabel: "To",
        messageLabel: "Message",
        sendBtn: "Send",
        sendingBtn: "Sending…",
        sentNotice: "Message sent",
        emptyInbox: "No messages",
        emptySent: "No sent messages",
        emptyArchive: "No archived messages",
        backToList: "Back",
        dirIncoming: "Received",
        dirOutgoing: "Sent",
        tagUnread: "Unread",
        tagArchived: "Archived",
        reply: "Reply",
        markRead: "Mark Read",
        archiveBtn: "Archive",
        deleteBtn: "Delete",
        deleteTitle: "Delete This Message?",
        deleteBody: "This removes the message from the device. It cannot be undone in the panel.",
        deletedNotice: "Message deleted",
        restoreInbox: "Move to Inbox",
        restoreSent: "Move to Sent"
      },
      device: {
        model: "Model",
        android: "Android Version",
        firmware: "Firmware",
        sim: "SIM",
        wlanIp: "WLAN / Hotspot IP",
        usbIp: "USB IP",
        tsIp: "Tailscale IP",
        uptime: "Uptime",
        uptimeFmt: "{h}h {m}m",
        time: "System Time"
      }
    }
  };

  var SERVER_MSG_EN = {
    "设备服务暂不可用，请稍后重试": "Device service temporarily unavailable, please retry shortly",
    "账号或密码错误": "Incorrect username or password",
    "无法保存登录会话": "Failed to persist login session",
    "请登录后台": "Authentication required, please sign in",
    "login required": "Authentication required, please sign in",
    "来源不允许": "Cross-origin request rejected",
    "需要 JSON 请求": "JSON request required",
    "请求无效": "Invalid request",
    "不支持的操作": "Unsupported operation",
    "Android 设置服务未就绪": "Android settings bridge is not ready",
    "Android 设置操作失败": "Android settings operation failed",
    "Android Wi-Fi 服务不可用": "Android Wi-Fi service unavailable",
    "请先开启飞行模式；操作已停止": "Please enable Airplane Mode first; operation aborted",
    "飞行模式必须保持开启": "Airplane Mode must remain enabled",
    "WFC 服务操作失败": "WFC service control failed",
    "WFC 尚未停止，请稍后重试": "WFC is still stopping, please retry shortly",
    "SIM 策略未通过 23487 检查，WFC 保持关闭": "SIM policy did not match 23487; WFC remains disabled",
    "Hysteria2 链接无效": "Invalid Hysteria2 URI",
    "代理节点链接无效": "Invalid proxy node URI",
    "代理链接需以 socks5://、hy2:// 或 ss:// 开头": "Proxy URI must start with socks5://, hy2://, or ss://",
    "SOCKS5 链接无效（格式：socks5://[用户名:密码@]IP:端口）": "Invalid SOCKS5 URI (expected socks5://[user:pass@]host:port)",
    "SOCKS5 账号或密码长度超出限制": "SOCKS5 username or password is too long",
    "Shadowsocks 链接无效（支持 aes-128-gcm / aes-256-gcm）": "Invalid Shadowsocks URI (aes-128-gcm / aes-256-gcm supported)",
    "Shadowsocks 链接无效": "Invalid Shadowsocks URI",
    "Shadowsocks 支持 aes-128-gcm、aes-256-gcm 和 SS2022": "Shadowsocks supports aes-128-gcm, aes-256-gcm and SS2022",
    "不支持的 SS2022 加密方式": "Unsupported SS2022 cipher",
    "SS2022 密钥无效": "Invalid SS2022 key",
    "SS2022 ChaCha20 不支持多用户密钥链": "SS2022 ChaCha20 does not support identity key chains",
    "SS2022 密钥必须是匹配加密方式的 Base64 密钥（AES-128 为 16 字节，其余为 32 字节）": "SS2022 requires a Base64 key: 16 bytes for AES-128, 32 bytes for AES-256 or ChaCha20",
    "暂不支持带插件参数的 ss:// 链接": "SIP003 plugins in ss:// URIs are not supported",
    "SOCKS5 代理配置无效": "Invalid SOCKS5 proxy configuration",
    "Shadowsocks 代理配置无效": "Invalid Shadowsocks proxy configuration",
    "无法删除短信": "Could not delete message",
    "短信已删除，但归档标记清理失败": "Message deleted, but its archive marker could not be cleared",
    "仅代理 WFC 目前支持 SOCKS5 / Shadowsocks": "WFC-only routing currently supports SOCKS5 / Shadowsocks",
    "密码中的冒号需要 URL 编码": "Colons in password must be URL-encoded",
    "无法解析节点 IPv4，请使用 IPv4 节点链接": "Unable to resolve node IPv4; please use an IPv4 address",
    "此设备的节点需提供 IPv4": "Node must provide an IPv4 address",
    "证书指纹需为 64 位 SHA256": "Certificate pin must be a 64-character SHA256 hex string",
    "跳过 CA 验证时必须提供 pinSHA256": "pinSHA256 is required when insecure=1 is set",
    "不支持该混淆参数": "Unsupported obfuscation parameter",
    "请选择直连或代理": "Please choose Direct or Proxy",
    "请选择直连或 Hysteria2": "Please choose Direct or Proxy",
    "ePDG 地址无效": "Invalid ePDG address",
    "请填写代理节点链接（支持 socks5://、hy2://、ss://）": "Enter a proxy node URI (socks5://, hy2://, or ss://)",
    "请粘贴 Hysteria2 节点链接": "Enter a proxy node URI",
    "请重新填写 IPv4 节点链接": "Please re-enter an IPv4 node URI",
    "代理配置保存失败，已恢复旧配置": "Failed to save proxy settings; previous configuration restored",
    "转发参数无效": "Invalid forwarding parameters",
    "转发地址必须是 HTTPS 地址": "Forwarding endpoint must use HTTPS",
    "转发服务未就绪": "Forwarding service is not ready",
    "无法读取转发设置": "Failed to read forwarding configuration",
    "转发设置无效": "Saved forwarding configuration is invalid",
    "保存转发设置失败": "Failed to save forwarding settings",
    "无法连接转发服务，将自动重试": "Unable to reach forwarding endpoint; will retry automatically",
    "转发服务未确认接收": "Forwarding endpoint did not acknowledge receipt",
    "cURL 命令过长": "cURL command is too long",
    "命令末尾的反斜杠不完整": "Trailing backslash at end of command",
    "请使用 {{短信变量}}，不支持 Shell 变量或命令替换": "Use {{sms-*}} placeholders; shell variables are not supported",
    "请只粘贴一条 cURL 命令；地址和数据用引号包住": "Paste a single cURL command and quote URLs or data payloads",
    "cURL 命令的引号未闭合": "Unclosed quote in cURL command",
    "请粘贴以 curl 开头的命令": "Command must start with curl",
    "只支持单个请求地址": "Only a single request URL is supported",
    "请求头应为 名称: 值": "Header must be formatted as Name: Value",
    "--max-time 应为 3–60 的整数秒": "--max-time must be an integer between 3 and 60 seconds",
    "请直接粘贴数据，不支持读取本地文件": "Inline payload required; reading local files (@file) is not supported",
    "cURL 命令缺少 HTTPS 接收地址": "cURL command is missing an HTTPS URL",
    "JSON 请求不能混用 --data-urlencode": "Cannot mix --data-urlencode with JSON request",
    "--data-urlencode 应为 字段=值": "--data-urlencode must be formatted as key=value",
    "纯文本数据请用 -H 'Content-Type: text/plain'；表单请用 字段=值": "Add -H 'Content-Type: text/plain' for plain text, or use key=value for form data",
    "表单编码无效，请使用 --data-urlencode '字段=值'": "Invalid form encoding; use --data-urlencode 'key=value'",
    "纯文本请求不能混用 --data-urlencode": "Cannot mix --data-urlencode with plain text request",
    "--get 请与表单数据一起使用": "--get must be used with form fields",
    "请求地址无效": "Invalid request URL",
    "修改接收站点时请填写完整鉴权参数，不能使用掩码星号": "Provide full credentials when changing the endpoint URL",
    "短信标识无效": "Invalid SMS identifier",
    "短信分类无效": "Invalid SMS folder",
    "短信收件箱暂不可用": "SMS database temporarily unavailable",
    "无法标记短信": "Unable to mark SMS as read",
    "开关参数无效": "Invalid toggle parameter",
    "代理参数无效": "Invalid proxy parameters",
    "Wi-Fi 参数无效": "Invalid Wi-Fi parameters",
    "配置参数无效": "Invalid profile parameter",
    "激活码参数无效": "Invalid activation code parameter",
    "登录参数无效": "Invalid login parameter",
    "无法保存飞行模式": "Failed to update Airplane Mode setting",
    "飞行模式广播失败": "Airplane Mode broadcast failed",
    "无法关闭设备热点": "Unable to disable Personal Hotspot",
    "无法开启 Wi-Fi": "Unable to enable Wi-Fi",
    "无法关闭 Wi-Fi": "Unable to turn off Wi-Fi",
    "无法断开当前 Wi-Fi": "Unable to disconnect from Wi-Fi",
    "无法移除已保存的 Wi-Fi": "Unable to forget saved network",
    "无法开启无线热点": "Unable to turn on Personal Hotspot",
    "无法关闭无线热点": "Unable to turn off Personal Hotspot",
    "热点名称无效": "Invalid hotspot name (1–32 UTF-8 bytes)",
    "热点密码需为 8–63 字节": "Hotspot password must be 8–63 characters",
    "热点参数无效": "Invalid hotspot parameters",
    "无法保存热点配置": "Unable to save hotspot settings",
    "USB 网络共享切换失败": "Unable to toggle USB tethering",
    "数据漫游设置失败": "Unable to update Data Roaming",
    "已保存的网络不存在": "Saved Wi-Fi network not found",
    "Wi-Fi 名称无效": "Invalid Wi-Fi SSID (1–32 UTF-8 bytes)",
    "开放网络不需要密码": "Open networks do not use a password",
    "Wi-Fi 密码需为 8–63 字节": "Wi-Fi password must be 8–63 bytes",
    "此 Android 版本仅支持开放或 WPA/WPA2 个人网络": "Only Open and WPA/WPA2 Personal networks are supported",
    "无法保存 Wi-Fi 网络": "Failed to save Wi-Fi configuration",
    "无法开始连接": "Unable to initiate Wi-Fi connection",
    "已开始连接，网络切换时页面可能短暂断开": "Connecting to Wi-Fi; console may briefly reconnect",
    "无法打开 eUICC 管理通道": "Unable to open eUICC management channel",
    "小白卡未提供可用的 ISD-R 管理通道": "Card does not expose a usable ISD-R management channel",
    "逻辑通道响应无效": "Invalid logical channel response",
    "eUICC 通道已断开": "eUICC channel disconnected",
    "eUICC 没有响应": "No response from eUICC",
    "eUICC 指令失败": "eUICC APDU command failed",
    "eUICC 响应无效": "Invalid eUICC response",
    "eSIM 配置标识无效": "Invalid eSIM profile identifier",
    "无法读取当前 eSIM 配置": "Failed to read current eSIM profiles",
    "这张卡上没有该配置": "Target profile not found on this card",
    "未找到指定的 eSIM 配置": "Target eSIM profile not found",
    "配置名称不能超过 64 字节": "Profile name must not exceed 64 UTF-8 bytes",
    "请先停用该 eSIM 配置后再删除": "Disable this eSIM profile before deleting it",
    "eSIM 激活码参数过长": "Activation code is too long",
    "激活码格式无效（示例：LPA:1$smdp.example.com$MATCHING-ID）": "Invalid activation code (e.g. LPA:1$smdp.example.com$MATCHING-ID)",
    "SM-DP+ 服务器地址无效": "Invalid SM-DP+ server address",
    "请填写 LPA 激活码或 SM-DP+ 服务器地址": "Enter an LPA activation code or SM-DP+ address",
    "IMEI 格式无效（需为 14–15 位有效数字）": "Invalid IMEI (must be 14–15 digits)",
    "此 eSIM 需要填写确认码（Confirmation Code）": "Confirmation Code is required for this eSIM",
    "eSIM 已切换，Wi-Fi 通话已关闭": "eSIM switched; Wi-Fi Calling is off",
    "eSIM 已切换；当前卡非 23487，Wi-Fi 通话保持关闭": "eSIM switched; non-23487 SIM, Wi-Fi Calling remains off",
    "eSIM 已切换，Wi-Fi 通话正在重新注册": "eSIM switched; Wi-Fi Calling is re-registering",
    "设备未安装 Tailscale 组件": "Tailscale binaries are not installed on this device",
    "设备未安装 SSH 组件": "SSH is not installed on this device",
    "无法保存 SSH 开关状态": "Failed to save SSH preference",
    "设置 SSH 失败，请检查设备日志": "Failed to update SSH; check the device log",
    "SSH 状态未更新，请重试": "SSH status did not change; try again",
    "无法创建 Tailscale 状态目录": "Failed to create Tailscale state directory",
    "无法打开 Tailscale 日志文件": "Failed to open Tailscale log file",
    "无法启动 tailscaled 守护进程": "Failed to start tailscaled daemon",
    "Tailscale 服务启动超时": "Timed out waiting for Tailscale daemon",
    "无法保存 Tailscale 开关状态": "Failed to save Tailscale preference",
    "无法保存 Tailscale 状态": "Failed to save Tailscale state",
    "请先开启 Tailscale": "Turn on Tailscale first",
    "无法读取 Tailscale SSH 状态": "Failed to read Tailscale SSH status",
    "设置 Tailscale SSH 失败": "Failed to update Tailscale SSH",
    "无法准备 Tailscale SSH 兼容命令": "Failed to prepare Tailscale SSH compatibility tools",
    "Tailscale SSH 状态未更新，请重试": "Tailscale SSH status did not change; try again",
    "Auth Key 格式无效": "Invalid Auth Key format",
    "Tailscale Auth Key 登录失败，请检查密钥与网络连接": "Tailscale Auth Key login failed; check key and internet connectivity",
    "无法发起 Tailscale 登录": "Failed to initiate Tailscale login",
    "无法获取 Tailscale 登录链接，请确认设备已连接互联网": "Timed out obtaining Tailscale login URL; check internet access",
    "退出 Tailscale 登录失败": "Failed to sign out of Tailscale"
  };

  var SERVER_MSG_ZH = {
    "invalid SMS": "短信请求参数无效",
    "invalid SMS recipient": "收件号码格式无效（3–15 位数字，可带 + 号）",
    "SMS must contain between one and six parts": "短信长度超出限制（最多 6 段）",
    "SIM SMSC is unavailable": "无法从 SIM 卡读取短信中心号码",
    "login required": "请重新登录后台"
  };

  function translateServerMsg(msg, lang) {
    if (!msg) return "";
    var target = lang || currentLang;
    var trimmed = String(msg).trim();
    if (target === "en") {
      if (Object.prototype.hasOwnProperty.call(SERVER_MSG_EN, trimmed)) {
        return SERVER_MSG_EN[trimmed];
      }
      if (trimmed.indexOf("不支持的节点参数：") === 0) {
        return "Unsupported proxy parameter: " + trimmed.slice(9);
      }
      if (trimmed.indexOf("不支持的代理协议：") === 0) {
        return "Unsupported proxy scheme: " + trimmed.slice(9);
      }
      if (trimmed.indexOf("Shadowsocks 仅支持 ") === 0) {
        return "Shadowsocks supports aes-128-gcm or aes-256-gcm only";
      }
      if (trimmed.indexOf("暂不支持 cURL 参数：") === 0) {
        return "Unsupported cURL option: " + trimmed.slice(13);
      }
      if (trimmed.indexOf("cURL 参数缺少值：") === 0) {
        return "Missing value for cURL option: " + trimmed.slice(11);
      }
      if (trimmed.indexOf("无法读取 eSIM 配置列表：") === 0) {
        return "Failed to read eSIM profiles: " + trimmed.slice(15);
      }
      if (trimmed.indexOf("eSIM 切换失败：") === 0) {
        return "eSIM switch failed: " + trimmed.slice(10);
      }
      if (trimmed.indexOf("停用 eSIM 配置失败：") === 0) {
        return "Failed to disable eSIM profile: " + trimmed.slice(13);
      }
      if (trimmed.indexOf("修改配置名称失败：") === 0) {
        return "Failed to rename eSIM profile: " + trimmed.slice(9);
      }
      if (trimmed.indexOf("删除 eSIM 配置失败：") === 0) {
        return "Failed to delete eSIM profile: " + trimmed.slice(13);
      }
      if (trimmed.indexOf("下载安装 eSIM 失败：") === 0) {
        return "Failed to download eSIM: " + trimmed.slice(13);
      }
      if (trimmed.indexOf("转发服务返回 HTTP ") === 0) {
        return "Forwarding endpoint returned HTTP " + trimmed.slice(12);
      }
      return trimmed;
    }
    if (Object.prototype.hasOwnProperty.call(SERVER_MSG_ZH, trimmed)) {
      return SERVER_MSG_ZH[trimmed];
    }
    if (trimmed.indexOf("IMS SMS rejected: ") === 0) {
      return "运营商拒绝短信发送：" + trimmed.slice(18);
    }
    return trimmed;
  }

  var currentLang = localStorage.getItem(LANG_KEY) === "en" ? "en" : "zh";

  function lookup(lang, path) {
    var parts = path.split(".");
    var cur = I18N[lang] || I18N.zh;
    for (var i = 0; i < parts.length; i++) {
      if (cur == null) return path;
      cur = cur[parts[i]];
    }
    return cur == null ? path : cur;
  }

  function t(path, vars) {
    var str = lookup(currentLang, path);
    if (typeof str !== "string") return str;
    if (vars) {
      Object.keys(vars).forEach(function (k) {
        str = str.replace(new RegExp("\\{" + k + "\\}", "g"), vars[k]);
      });
    }
    return str;
  }

  function el(id) {
    return document.getElementById(id);
  }

  function setText(id, val) {
    var node = el(id);
    if (node) node.textContent = val == null || val === "" ? "—" : val;
  }

  function normalizeRoute(rawKey) {
    var k = String(rawKey || "").replace(/^#/, "").trim();
    if (!k || k === "overview") return { view: "connect", smsBox: null };
    if (k === "network") return { view: "wifi", smsBox: null };
    if (k === "euicc") return { view: "cellular", smsBox: null };
    if (k === "device") return { view: "about", smsBox: null };
    if (k === "forward") return { view: "sms", smsBox: "forward" };
    if (Object.prototype.hasOwnProperty.call(I18N.zh.views, k)) {
      return { view: k, smsBox: null };
    }
    return { view: "connect", smsBox: null };
  }

  function applyStaticI18n() {
    document.documentElement.lang = currentLang === "en" ? "en" : "zh-CN";
    var isLogin = document.body.dataset.page === "login";
    document.title = t(isLogin ? "pageTitleLogin" : "pageTitleApp");

    document.querySelectorAll(".lang-switch button[data-lang]").forEach(function (btn) {
      var active = btn.dataset.lang === currentLang;
      btn.classList.toggle("active", active);
      btn.setAttribute("aria-pressed", active ? "true" : "false");
    });

    document.querySelectorAll("[data-i18n]").forEach(function (node) {
      node.textContent = t(node.dataset.i18n);
    });
    document.querySelectorAll("[data-i18n-placeholder]").forEach(function (node) {
      node.placeholder = t(node.dataset.i18nPlaceholder);
    });
    document.querySelectorAll("[data-i18n-aria]").forEach(function (node) {
      node.setAttribute("aria-label", t(node.dataset.i18nAria));
    });
  }

  function bindSecretToggle(btnId, inputId, ariaShowKey, ariaHideKey) {
    var btn = el(btnId);
    var input = el(inputId);
    if (!btn || !input) return null;
    function sync() {
      var hidden = input.type === "password";
      btn.textContent = t(hidden ? "common.show" : "common.hide");
      if (ariaShowKey && ariaHideKey) {
        btn.setAttribute("aria-label", t(hidden ? ariaShowKey : ariaHideKey));
      }
    }
    btn.addEventListener("click", function () {
      input.type = input.type === "password" ? "text" : "password";
      sync();
    });
    sync();
    return sync;
  }

  var token = sessionStorage.getItem(SESSION_KEY);
  var busyState = false;
  var noticeState = null;

  function setNotice(kind, textOrKey, isKey, vars) {
    if (!textOrKey) {
      noticeState = null;
    } else {
      noticeState = { kind: kind || "info", value: textOrKey, isKey: !!isKey, vars: vars || null };
    }
    renderNotice();
  }

  function renderNotice() {
    if (document.body.dataset.page === "login") {
      var box = el("login-error");
      var txt = el("login-error-text");
      if (!box || !txt) return;
      if (!noticeState) {
        box.hidden = true;
        txt.textContent = "";
      } else {
        box.hidden = false;
        txt.textContent = noticeState.isKey
          ? t(noticeState.value, noticeState.vars)
          : translateServerMsg(noticeState.value, currentLang);
      }
      return;
    }
    var alertBox = el("notice");
    var alertTxt = el("notice-text");
    if (!alertBox || !alertTxt) return;
    if (!noticeState) {
      alertBox.hidden = true;
      alertTxt.textContent = "";
      return;
    }
    alertBox.hidden = false;
    alertBox.className = "alert " + (noticeState.kind === "error" ? "error" : noticeState.kind === "warn" ? "warn" : "ok");
    alertTxt.textContent = noticeState.isKey
      ? t(noticeState.value, noticeState.vars)
      : translateServerMsg(noticeState.value, currentLang);
  }

  function loginTarget() {
    var view = location.hash.replace("#", "") || "connect";
    return "/login.html?next=" + encodeURIComponent(view);
  }

  function request(path, body, ims, timeoutMs) {
    var controller = new AbortController();
    var timer = setTimeout(function () {
      controller.abort();
    }, timeoutMs || 90000);

    return fetch((ims ? "/wfc/" : "/wfc-admin/") + path, {
      method: body === undefined ? "GET" : "POST",
      credentials: "same-origin",
      cache: "no-store",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + (token || "")
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal
    }).then(function (r) {
      return r.json().catch(function () {
        return { error: t("common.serviceUnavailable") };
      }).then(function (data) {
        if (r.status === 401 && !ims && path !== "login") {
          sessionStorage.removeItem(SESSION_KEY);
          location.replace(loginTarget());
          throw new Error(t("common.relogin"));
        }
        if (!r.ok) {
          throw new Error(data.error || t("common.requestFailed"));
        }
        return data;
      });
    }).finally(function () {
      clearTimeout(timer);
    });
  }

  function busy(operation) {
    if (busyState) {
      return Promise.reject(new Error(t("common.busyWait")));
    }
    busyState = true;
    var controls = document.querySelectorAll(
      "form button, form input, form select, form textarea, [data-switch], #wifi-picker button, #wifi-picker input, #wifi-scan, #wifi-other-btn, #cell-epdg, #euicc-add-btn, #euicc-refresh, #euicc-list button, #ts-login-btn, #ts-key-toggle, #ts-logout-btn"
    );
    controls.forEach(function (node) {
      node.dataset.wasDisabled = node.disabled ? "1" : "0";
      node.disabled = true;
    });
    return operation().finally(function () {
      busyState = false;
      controls.forEach(function (node) {
        node.disabled = node.dataset.wasDisabled === "1";
        delete node.dataset.wasDisabled;
      });
    });
  }

  /* =========================================================
     Login Page Controller
     ========================================================= */
  if (document.body.dataset.page === "login") {
    var syncLoginPwd = bindSecretToggle("show-password", "password", "login.showPassword", "login.hidePassword");
    var loginSubmitting = false;

    function syncLoginLang(lang) {
      currentLang = lang === "en" ? "en" : "zh";
      localStorage.setItem(LANG_KEY, currentLang);
      applyStaticI18n();
      if (syncLoginPwd) syncLoginPwd();
      el("login-submit").textContent = t(loginSubmitting ? "login.submitting" : "login.submit");
      renderNotice();
    }

    document.querySelectorAll(".lang-switch button[data-lang]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        syncLoginLang(this.dataset.lang);
      });
    });

    el("login-form").addEventListener("submit", function (e) {
      e.preventDefault();
      var acc = el("account").value.trim();
      var pwd = el("password").value;
      if (!acc || !pwd) {
        setNotice("error", "login.emptyFields", true);
        return;
      }
      setNotice(null);
      loginSubmitting = true;
      el("login-submit").textContent = t("login.submitting");
      busy(function () {
        return request("login", { Account: acc, Password: pwd });
      }).then(function (data) {
        sessionStorage.setItem(SESSION_KEY, data.token);
        sessionStorage.setItem(ACCOUNT_KEY, acc);
        el("password").value = "";
        var next = new URLSearchParams(location.search).get("next") || "";
        var dest = next === "forward" ? "forward" : normalizeRoute(next).view;
        location.replace("/index.html#" + dest);
      }).catch(function (err) {
        setNotice("error", err.message, false);
      }).finally(function () {
        loginSubmitting = false;
        el("login-submit").textContent = t("login.submit");
      });
    });

    syncLoginLang(currentLang);
    return;
  }

  /* =========================================================
     Main Settings Console Controller
     ========================================================= */
  if (!token) {
    location.replace(loginTarget());
    return;
  }

  var state = null;
  var imsState = null;
  var currentView = "connect";
  var smsBox = "inbox";
  var smsData = null;
  var selectedSmsId = null;
  var mobileReading = false;
  var esimQR = window.WfcEsimQR ? window.WfcEsimQR.create({
    t: t,
    onResult: function (code) {
      el("euicc-add-drawer").hidden = false;
      el("euicc-code").value = code;
      el("euicc-code").focus();
      setNotice("ok", "euicc.qrRecognized", true);
    }
  }) : null;
  var scannedWifi = null;
  var autoScannedOnce = false;
  var expandedWifiSsid = "";
  var hotspotDirty = false;
  var euiccData = null;
  var euiccEid = "";
  var euiccLoading = null; // null | "read" | "switch" | "disable" | "rename" | "delete" | "download"
  var euiccTargetName = "";
  var euiccError = null;
  var confirmState = null;
  var tsAuthPollTimer = null;

  var editor = new window.WFCForwardEditor(
    el("forward-editor"),
    function (path, body) { return request(path, body); },
    { lang: currentLang, translateMsg: translateServerMsg }
  );

  /* =========================================================
     OS Settings Quick List (#connect) & Global Switches
     ========================================================= */
  var CONNECT_ROWS = [
    {
      key: "flight",
      jump: "",
      icon: '<path d="M17.8 19.2 16 11l3.5-3.5C21 6 21.5 4 21 3c-1-.5-3 0-4.5 1.5L13 8 4.8 6.2c-.5-.1-.9.1-1.1.5l-.3.5c-.2.5-.1 1 .3 1.3L9 12l-2 3H4l-1 1 3 2 2 3 1-1v-3l3-2 3.5 5.3c.3.4.8.5 1.3.3l.5-.2c.4-.3.6-.7.5-1.2z"></path>'
    },
    {
      key: "wifi",
      jump: "#wifi",
      icon: '<path d="M5 12.55a11 11 0 0 1 14.08 0"></path><path d="M1.42 9a16 16 0 0 1 21.16 0"></path><path d="M8.53 16.11a6 6 0 0 1 6.95 0"></path><circle cx="12" cy="20" r="1"></circle>'
    },
    {
      key: "hotspot",
      jump: "#hotspot",
      icon: '<circle cx="12" cy="12" r="2"></circle><path d="M16.24 7.76a6 6 0 0 1 0 8.49"></path><path d="M7.76 16.24a6 6 0 0 1 0-8.49"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path><path d="M4.93 19.07a10 10 0 0 1 0-14.14"></path>'
    },
    {
      key: "hotspot_broadcast",
      jump: "",
      icon: '<path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7-11-7-11-7z"></path><circle cx="12" cy="12" r="3"></circle>'
    },
    {
      key: "usb_tether",
      jump: "",
      icon: '<line x1="12" y1="3" x2="12" y2="19"></line><polyline points="9 6 12 3 15 6"></polyline><circle cx="12" cy="20" r="1.5"></circle><path d="M12 15l-4-3V9"></path><circle cx="8" cy="8" r="1"></circle><path d="M12 13l4-3V8"></path><rect x="15" y="6" width="2" height="2"></rect>'
    },
    {
      key: "mobile_data",
      jump: "",
      icon: '<path d="m21 16-4 4-4-4"></path><path d="M17 20V4"></path><path d="m3 8 4-4 4 4"></path><path d="M7 4v16"></path>'
    },
    {
      key: "data_roaming",
      jump: "",
      icon: '<circle cx="12" cy="12" r="10"></circle><line x1="2" y1="12" x2="22" y2="12"></line><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>'
    },
    {
      key: "wfc",
      jump: "#cellular",
      icon: '<path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.12.96.36 1.9.7 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.91.34 1.85.58 2.81.7A2 2 0 0 1 22 16.92z"></path><path d="M14.5 3.5a5 5 0 0 1 6 6"></path>'
    },
    {
      key: "tailscale",
      jump: "#vpn",
      icon: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>'
    }
  ];

  function mountConnectList() {
    var container = el("connect-list");
    if (!container) return;
    container.innerHTML = "";
    CONNECT_ROWS.forEach(function (item) {
      var row = document.createElement("div");
      row.className = "settings-row";
      row.dataset.connectRow = item.key;

      var left = document.createElement("div");
      left.className = "settings-row-left" + (item.jump ? " is-link" : "");
      if (item.jump) {
        left.addEventListener("click", function () {
          location.hash = item.jump;
        });
      }

      var iconWrap = document.createElement("span");
      iconWrap.className = "row-icon";
      iconWrap.innerHTML = '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">' + item.icon + "</svg>";

      var titleEl = document.createElement("span");
      titleEl.className = "row-title";
      titleEl.dataset.switchLabel = item.key;
      titleEl.textContent = t("switches." + item.key);

      left.appendChild(iconWrap);
      left.appendChild(titleEl);

      var right = document.createElement("div");
      right.className = "settings-row-right";

      if (item.jump) {
        var jumpLink = document.createElement("a");
        jumpLink.className = "row-jump";
        jumpLink.href = item.jump;
        var valSpan = document.createElement("span");
        valSpan.dataset.connectVal = item.key;
        jumpLink.appendChild(valSpan);
        var chev = document.createElement("span");
        chev.innerHTML = '<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><polyline points="9 18 15 12 9 6"></polyline></svg>';
        jumpLink.appendChild(chev.firstChild);
        right.appendChild(jumpLink);
      } else {
        var plainVal = document.createElement("span");
        plainVal.className = "row-value";
        plainVal.dataset.connectVal = item.key;
        right.appendChild(plainVal);
      }

      var swLabel = document.createElement("label");
      swLabel.className = "switch";
      var swInput = document.createElement("input");
      swInput.type = "checkbox";
      swInput.dataset.switch = item.key;
      swInput.setAttribute("aria-label", t("switches." + item.key));
      var swTrack = document.createElement("span");
      swTrack.className = "switch-track";
      swLabel.appendChild(swInput);
      swLabel.appendChild(swTrack);

      right.appendChild(swLabel);
      row.appendChild(left);
      row.appendChild(right);
      container.appendChild(row);
    });
  }

  mountConnectList();

  /* Bind all [data-switch] inputs across #connect, #wifi, #hotspot, #cellular, #vpn */
  document.querySelectorAll("[data-switch]").forEach(function (sw) {
    sw.addEventListener("change", function () {
      handleSwitchChange(sw.dataset.switch, sw.checked, sw);
    });
  });

  function syncSwitchLabels() {
    CONNECT_ROWS.forEach(function (item) {
      var labelTxt = t("switches." + item.key);
      document.querySelectorAll('[data-switch-label="' + item.key + '"]').forEach(function (n) {
        n.textContent = labelTxt;
      });
      document.querySelectorAll('[data-switch="' + item.key + '"]').forEach(function (n) {
        n.setAttribute("aria-label", labelTxt);
      });
    });
  }

  function updateSwitchState(key, checked, valueText, disabled) {
    document.querySelectorAll('[data-connect-row="' + key + '"]').forEach(function (row) {
      row.classList.toggle("is-on", !!checked);
    });
    document.querySelectorAll('[data-switch="' + key + '"]').forEach(function (sw) {
      sw.checked = !!checked;
      if (!busyState) {
        sw.disabled = !!disabled;
      }
    });
    document.querySelectorAll('[data-connect-val="' + key + '"]').forEach(function (v) {
      v.textContent = valueText || "";
    });
  }

  function executeSwitchMutation(kind, enabled) {
    var reqPath = "";
    var reqBody = { enabled: enabled };
    if (kind === "wifi") reqPath = "wifi";
    else if (kind === "hotspot") reqPath = "hotspot";
    else if (kind === "hotspot_broadcast") {
      reqPath = "hotspot/config";
      reqBody = { broadcast: enabled };
    } else if (kind === "usb_tether") reqPath = "usb-tether";
    else if (kind === "wfc") reqPath = "wfc";
    else if (kind === "flight") reqPath = "flight";
    else if (kind === "mobile_data") reqPath = "mobile-data";
    else if (kind === "data_roaming") reqPath = "data-roaming";
    else if (kind === "tailscale") reqPath = "tailscale";
    else if (kind === "tailscale_ssh") reqPath = "tailscale/ssh";
    else if (kind === "ssh") reqPath = "ssh";

    return busy(function () {
      return request(reqPath, reqBody);
    }).then(function () {
      if (kind === "wifi" && !enabled) {
        scannedWifi = null;
        expandedWifiSsid = "";
      }
      setNotice("ok", "confirmations.toggleSaved", true);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      refreshStatus(false).then(function () {
        if (kind === "wifi" && enabled && state && state.wifi_enabled) {
          triggerWifiScan(true);
        }
      }).catch(function () {});
    });
  }

  function handleSwitchChange(kind, enabled, sourceInput) {
    function revert() {
      document.querySelectorAll('[data-switch="' + kind + '"]').forEach(function (sw) {
        sw.checked = !enabled;
      });
      if (sourceInput) sourceInput.checked = !enabled;
    }

    if (kind === "flight" && !enabled) {
      openConfirm({
        titleKey: "confirmations.flightOffTitle",
        bodyKey: "confirmations.flightOffBody",
        okKey: "confirmations.turnOffBtn",
        danger: true,
        onConfirm: function () { executeSwitchMutation(kind, enabled); },
        onCancel: revert
      });
      return;
    }

    if (kind === "usb_tether" && !enabled) {
      openConfirm({
        titleKey: "confirmations.usbOffTitle",
        bodyKey: "confirmations.usbOffBody",
        okKey: "confirmations.turnOffBtn",
        onConfirm: function () { executeSwitchMutation(kind, enabled); },
        onCancel: revert
      });
      return;
    }

    if (kind === "hotspot" && !enabled) {
      openConfirm({
        titleKey: "confirmations.hotspotOffTitle",
        bodyKey: "confirmations.hotspotOffBody",
        okKey: "confirmations.turnOffBtn",
        onConfirm: function () { executeSwitchMutation(kind, enabled); },
        onCancel: revert
      });
      return;
    }

    if (kind === "wifi" && enabled && state && state.hotspot_enabled) {
      openConfirm({
        titleKey: "confirmations.wifiOnHotspotTitle",
        bodyKey: "confirmations.wifiOnHotspotBody",
        okKey: "confirmations.turnOnBtn",
        onConfirm: function () { executeSwitchMutation(kind, enabled); },
        onCancel: revert
      });
      return;
    }

    if (kind === "hotspot" && enabled && state && state.wifi_enabled) {
      openConfirm({
        titleKey: "confirmations.hotspotOnWifiTitle",
        bodyKey: "confirmations.hotspotOnWifiBody",
        okKey: "confirmations.turnOnBtn",
        onConfirm: function () { executeSwitchMutation(kind, enabled); },
        onCancel: revert
      });
      return;
    }

    executeSwitchMutation(kind, enabled);
  }

  /* Account menu & mobile drawer */
  var savedAccount = sessionStorage.getItem(ACCOUNT_KEY) || "admin";
  setText("account-name", savedAccount);
  setText("account-dropdown-user", savedAccount);

  function closeAccountMenu() {
    var dd = el("account-dropdown");
    var btn = el("account-btn");
    if (dd && !dd.hidden) {
      dd.hidden = true;
      btn.setAttribute("aria-expanded", "false");
    }
  }

  el("account-btn").addEventListener("click", function (e) {
    e.stopPropagation();
    var dd = el("account-dropdown");
    var open = dd.hidden;
    dd.hidden = !open;
    this.setAttribute("aria-expanded", open ? "true" : "false");
  });

  document.addEventListener("click", function (e) {
    if (!e.target.closest(".account-menu")) {
      closeAccountMenu();
    }
  });

  function openDrawer() {
    el("sidebar").classList.add("open");
    el("sidebar-backdrop").hidden = false;
    document.body.classList.add("drawer-open");
  }

  function closeDrawer() {
    el("sidebar").classList.remove("open");
    el("sidebar-backdrop").hidden = true;
    document.body.classList.remove("drawer-open");
  }

  el("menu-toggle").addEventListener("click", openDrawer);
  el("drawer-close").addEventListener("click", closeDrawer);
  el("sidebar-backdrop").addEventListener("click", closeDrawer);

  /* Accessible Confirmation & Input Modal */
  function openConfirm(opts) {
    confirmState = opts;
    var inputWrap = el("confirm-input-wrap");
    var inputNode = el("confirm-input");
    if (opts.inputMode) {
      inputWrap.hidden = false;
      inputNode.value = opts.inputValue || "";
    } else {
      inputWrap.hidden = true;
      inputNode.value = "";
    }
    renderConfirm();
    el("confirm-modal").hidden = false;
    if (opts.inputMode) {
      inputNode.focus();
      inputNode.select();
    } else {
      el("confirm-cancel").focus();
    }
  }

  function closeConfirm(accepted) {
    var current = confirmState;
    var inputVal = el("confirm-input").value.trim();
    confirmState = null;
    el("confirm-modal").hidden = true;
    el("confirm-input-wrap").hidden = true;
    if (current) {
      if (accepted && current.onConfirm) current.onConfirm(inputVal);
      if (!accepted && current.onCancel) current.onCancel();
    }
  }

  function renderConfirm() {
    if (!confirmState) return;
    el("confirm-title").textContent = t(confirmState.titleKey, confirmState.vars);
    el("confirm-body").textContent = t(confirmState.bodyKey, confirmState.vars);
    el("confirm-cancel").textContent = t("common.cancel");
    var okBtn = el("confirm-ok");
    okBtn.textContent = t(confirmState.okKey || "common.confirm");
    okBtn.className = "btn " + (confirmState.danger ? "btn-danger" : "btn-primary");
    if (confirmState.inputMode) {
      el("confirm-input").placeholder = confirmState.placeholderKey ? t(confirmState.placeholderKey) : "";
    }
  }

  el("confirm-cancel").addEventListener("click", function () { closeConfirm(false); });
  el("confirm-ok").addEventListener("click", function () { closeConfirm(true); });
  el("confirm-input").addEventListener("keydown", function (e) {
    if (e.key === "Enter" && confirmState && confirmState.inputMode) {
      e.preventDefault();
      closeConfirm(true);
    }
  });
  el("confirm-modal").addEventListener("click", function (e) {
    if (e.target === el("confirm-modal")) closeConfirm(false);
  });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      if (confirmState) {
        closeConfirm(false);
      } else {
        closeAccountMenu();
        closeDrawer();
      }
    }
  });

  el("notice-close").addEventListener("click", function () {
    setNotice(null);
  });

  /* Helpers */
  function describeSupervisor(raw) {
    var val = (raw || "").trim();
    if (!val || val === "stopped") return t("supervisor.stopped");
    if (val === "waiting: Android boot") return t("supervisor.boot");
    if (val === "paused: flight mode is off") return t("supervisor.flightOff");
    if (val === "waiting: Wi-Fi gateway") return t("supervisor.wifiGateway");
    if (val === "waiting: UK proxy tunnel") return t("supervisor.proxyTunnel");
    if (val === "waiting: WFC process") return t("supervisor.wfcProcess");
    if (val === "running: proxy and WFC supervised") return t("supervisor.running");
    if (val === "waiting: proxy interface has another owner") return t("supervisor.proxyOwner");
    if (val === "waiting: IMS interface has another owner") return t("supervisor.imsOwner");
    if (val.indexOf("error:") === 0) return t("supervisor.assetsError");
    return val;
  }

  function describeSim(s) {
    if (s.sim_prefix === "23487") {
      return "23487 · Lebara UK";
    }
    if (s.sim_prefix) {
      return s.sim_prefix + " (" + t("cellular.simNonPolicy") + ")";
    }
    if (s.sim_state === 1) return t("cellular.simAbsent");
    if (s.sim_state === 2 || s.sim_state === 3 || s.sim_state === 4) return t("cellular.simLocked");
    return t("cellular.simNotReady");
  }

  function isWifiConnected(s) {
    return !!(s && s.wifi_enabled && s.ssid && s.connection === "COMPLETED" && s.ipv4 && s.ipv4 !== "0.0.0.0");
  }

  function selectedProxyMode() {
    var checked = document.querySelector('input[name="proxy-mode"]:checked');
    return checked && checked.value === "direct" ? "direct" : "proxy";
  }

  function setProxyRadio(mode) {
    var target = mode === "direct" ? "direct" : "proxy";
    document.querySelectorAll('input[name="proxy-mode"]').forEach(function (r) {
      r.checked = r.value === target;
    });
  }

  function isProxyFormDirty() {
    if (!state || !state.proxy) return false;
    var savedMode = state.proxy.mode === "direct" ? "direct" : "proxy";
    var savedUri = state.proxy.uri || "";
    return selectedProxyMode() !== savedMode || el("proxy-uri").value.trim() !== savedUri
      || proxyWFCOnly() !== !!state.proxy.wfc_only;
  }

  function proxySupportsWFCOnly() {
    return /^(ss|socks5|socks5h|socks):\/\//i.test(el("proxy-uri").value.trim());
  }

  function proxyWFCOnly() {
    return proxySupportsWFCOnly() && el("proxy-wfc-only").checked;
  }

  function syncProxyControls() {
    el("proxy-wfc-only-row").hidden = selectedProxyMode() !== "proxy" || !proxySupportsWFCOnly();
    var dirty = isProxyFormDirty();
    el("proxy-reset-btn").hidden = !dirty;
  }

  function syncHotspotControls() {
    var isOpen = el("hotspot-security").value === "open";
    el("hotspot-pwd-row").hidden = isOpen;
    el("hotspot-password").placeholder = t("hotspot.pwdPlaceholder");
  }

  function populateHotspotForm(s) {
    if (!s) return;
    el("hotspot-ssid").value = s.hotspot_ssid || "";
    el("hotspot-security").value = s.hotspot_security === "open" ? "open" : "wpa2";
    el("hotspot-password").value = s.hotspot_password || "";
    hotspotDirty = false;
    syncHotspotControls();
  }

  function syncWifiSecurityControls() {
    var isOpen = el("wifi-security").value === "open";
    el("wifi-password-field").hidden = isOpen;
    var grid = el("wifi-form").querySelector(".drawer-grid");
    if (grid) grid.classList.toggle("no-pwd", isOpen);
  }

  /* =========================================================
     WLAN View (#wifi): Current + Saved (Always Shown) + Other
     ========================================================= */
  function rssiToBars(rssi) {
    if (typeof rssi !== "number" || rssi <= -120) return 0;
    if (rssi >= -55) return 4;
    if (rssi >= -67) return 3;
    if (rssi >= -78) return 2;
    return 1;
  }

  function createWifiSignalSvg(bars, isSavedFallback) {
    var lvl = bars > 0 ? bars : (isSavedFallback ? 4 : 1);
    var cls = "wifi-signal" + (bars === 0 && isSavedFallback ? " is-saved" : "");
    var a2 = lvl >= 2 ? "" : ' class="arc-off"';
    var a3 = lvl >= 3 ? "" : ' class="arc-off"';
    var a4 = lvl >= 4 ? "" : ' class="arc-off"';
    var span = document.createElement("span");
    span.innerHTML =
      '<svg class="' + cls + '" viewBox="0 0 20 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
        '<path' + a4 + ' d="M1.5 5.2a12.3 12.3 0 0 1 17 0"></path>' +
        '<path' + a3 + ' d="M4.4 8.1a8.2 8.2 0 0 1 11.2 0"></path>' +
        '<path' + a2 + ' d="M7.3 11a4.1 4.1 0 0 1 5.4 0"></path>' +
        '<circle cx="10" cy="13.6" r="1.1" fill="currentColor" stroke="none"></circle>' +
      "</svg>";
    return span.firstChild;
  }

  function createLockSvg() {
    var span = document.createElement("span");
    span.innerHTML =
      '<svg class="icon wifi-lock" viewBox="0 0 24 24" aria-hidden="true">' +
        '<rect x="4" y="11" width="16" height="10" rx="2"></rect>' +
        '<path d="M8 11V7a4 4 0 0 1 8 0v4"></path>' +
      "</svg>";
    return span.firstChild;
  }

  function createChevronSvg() {
    var span = document.createElement("span");
    span.innerHTML =
      '<svg class="icon wifi-chevron" viewBox="0 0 24 24" aria-hidden="true">' +
        '<polyline points="6 9 12 15 18 9"></polyline>' +
      "</svg>";
    return span.firstChild;
  }

  function createWifiSection(titleText) {
    var wrap = document.createElement("div");
    wrap.className = "wifi-section";
    var hdr = document.createElement("div");
    hdr.className = "section-header";
    var ttl = document.createElement("span");
    ttl.className = "section-title";
    ttl.textContent = titleText;
    hdr.appendChild(ttl);
    var card = document.createElement("section");
    card.className = "settings-card";
    wrap.appendChild(hdr);
    wrap.appendChild(card);
    return { wrap: wrap, card: card };
  }

  function renderWifiPicker() {
    var picker = el("wifi-picker");
    if (!picker || !state) return;
    picker.innerHTML = "";

    var s = state;
    var wifiOk = isWifiConnected(s);
    var savedList = Array.isArray(s.saved) ? s.saved : [];

    var scanMap = {};
    var scanOrdered = [];
    if (Array.isArray(scannedWifi)) {
      scannedWifi.slice().sort(function (a, b) {
        return (b.rssi || -127) - (a.rssi || -127);
      }).forEach(function (n) {
        if (!n || !n.ssid) return;
        if (!scanMap[n.ssid]) {
          scanMap[n.ssid] = {
            ssid: n.ssid,
            rssi: n.rssi,
            frequency: n.frequency || 0,
            is5g: (n.frequency || 0) > 4900,
            security: n.security || ""
          };
          scanOrdered.push(scanMap[n.ssid]);
        } else if ((n.frequency || 0) > 4900) {
          scanMap[n.ssid].is5g = true;
        }
      });
    }

    var activeConnected = s.wifi_enabled && s.ssid && (wifiOk || s.connection === "COMPLETED");
    var currentSsid = activeConnected ? s.ssid : "";
    var currentNetId = activeConnected && typeof s.network_id === "number" ? s.network_id : -1;

    /* 1. Current Network */
    if (activeConnected) {
      var curSec = createWifiSection(t("wifi.groupCurrent"));
      var curItem = document.createElement("div");
      curItem.className = "wifi-item";
      var curRow = document.createElement("div");
      curRow.className = "wifi-row is-current";

      var curLeft = document.createElement("div");
      curLeft.className = "wifi-row-left";
      var curScan = scanMap[s.ssid];
      var curRssi = typeof s.rssi === "number" && s.rssi > -120 ? s.rssi : (curScan ? curScan.rssi : -55);
      var curSig = createWifiSignalSvg(rssiToBars(curRssi), true);
      if (typeof curRssi === "number" && curRssi > -120) curSig.title = curRssi + " dBm";
      curLeft.appendChild(curSig);

      var curNameWrap = document.createElement("div");
      curNameWrap.className = "wifi-name-wrap";
      var curName = document.createElement("span");
      curName.className = "wifi-name";
      curName.textContent = s.ssid;
      curNameWrap.appendChild(curName);
      if (curScan && curScan.is5g) {
        var b5 = document.createElement("span");
        b5.className = "wifi-band-tag";
        b5.textContent = "5G";
        curNameWrap.appendChild(b5);
      }
      var curIp = document.createElement("span");
      curIp.className = "wifi-meta-ip";
      curIp.textContent = wifiOk ? s.ipv4 : t("wifi.obtainingIp");
      curNameWrap.appendChild(curIp);
      curLeft.appendChild(curNameWrap);

      var curRight = document.createElement("div");
      curRight.className = "wifi-row-right";
      var matchedSaved = null;
      for (var i = 0; i < savedList.length; i++) {
        if ((currentNetId >= 0 && savedList[i].id === currentNetId) || savedList[i].ssid === s.ssid) {
          matchedSaved = savedList[i];
          break;
        }
      }
      if ((matchedSaved && matchedSaved.security !== "open") || (curScan && /WPA|PSK|SAE/.test(curScan.security))) {
        curRight.appendChild(createLockSvg());
      }

      var discBtn = document.createElement("button");
      discBtn.type = "button";
      discBtn.className = "btn btn-sm";
      discBtn.disabled = busyState;
      discBtn.textContent = t("wifi.disconnect");
      discBtn.addEventListener("click", function () {
        disconnectWifi();
      });
      curRight.appendChild(discBtn);

      curRow.appendChild(curLeft);
      curRow.appendChild(curRight);
      curItem.appendChild(curRow);
      curSec.card.appendChild(curItem);
      picker.appendChild(curSec.wrap);
    }

    /* 2. Saved Networks (ALWAYS displayed when savedList has entries, including when Wi-Fi is off or connected) */
    var savedSsidMap = {};
    savedList.forEach(function (net) {
      if (net && net.ssid) savedSsidMap[net.ssid] = net;
    });

    if (savedList.length > 0) {
      var savedSec = createWifiSection(t("wifi.groupSaved"));
      savedList.forEach(function (net) {
        if (!net || !net.ssid) return;
        var isCurrentSaved = activeConnected && ((currentNetId >= 0 && net.id === currentNetId) || net.ssid === currentSsid);

        var item = document.createElement("div");
        item.className = "wifi-item";

        var row = document.createElement("div");
        row.className = "wifi-row" + (!isCurrentSaved ? " is-clickable" : "");

        var left = document.createElement("div");
        left.className = "wifi-row-left";
        var inRange = scanMap[net.ssid];
        var sigBars = isCurrentSaved
          ? rssiToBars(typeof s.rssi === "number" && s.rssi > -120 ? s.rssi : (inRange ? inRange.rssi : -55))
          : (inRange ? rssiToBars(inRange.rssi) : 0);
        var sig = createWifiSignalSvg(sigBars, true);
        left.appendChild(sig);

        var nameWrap = document.createElement("div");
        nameWrap.className = "wifi-name-wrap";
        var nameEl = document.createElement("span");
        nameEl.className = "wifi-name";
        nameEl.textContent = net.ssid;
        nameWrap.appendChild(nameEl);

        if (inRange && inRange.is5g) {
          var tag5 = document.createElement("span");
          tag5.className = "wifi-band-tag";
          tag5.textContent = "5G";
          nameWrap.appendChild(tag5);
        }
        if (isCurrentSaved) {
          var connTag = document.createElement("span");
          connTag.className = "status-tag ok";
          connTag.textContent = t("common.connected");
          nameWrap.appendChild(connTag);
        }
        left.appendChild(nameWrap);

        var right = document.createElement("div");
        right.className = "wifi-row-right";
        if (net.security !== "open") {
          right.appendChild(createLockSvg());
        }

        var forgetBtn = document.createElement("button");
        forgetBtn.type = "button";
        forgetBtn.className = "btn btn-sm btn-ghost";
        forgetBtn.disabled = busyState;
        forgetBtn.textContent = t("wifi.forget");
        forgetBtn.addEventListener("click", function (e) {
          e.stopPropagation();
          forgetWifi(net);
        });
        right.appendChild(forgetBtn);

        if (!isCurrentSaved) {
          var joinPayload = { ssid: net.ssid, saved: true };
          if (typeof net.id === "number" && net.id >= 0) {
            joinPayload.network_id = net.id;
          }
          var joinBtn = document.createElement("button");
          joinBtn.type = "button";
          joinBtn.className = "btn btn-sm";
          joinBtn.disabled = busyState;
          joinBtn.textContent = t("wifi.join");
          joinBtn.addEventListener("click", function (e) {
            e.stopPropagation();
            connectWifiWithConfirm(joinPayload);
          });
          right.appendChild(joinBtn);

          row.addEventListener("click", function () {
            if (busyState) return;
            connectWifiWithConfirm(joinPayload);
          });
        }

        row.appendChild(left);
        row.appendChild(right);
        item.appendChild(row);
        savedSec.card.appendChild(item);
      });

      picker.appendChild(savedSec.wrap);
    }

    /* 3. Other Networks (when Wi-Fi is On) or Off Notice Card */
    if (!s.wifi_enabled) {
      var offCard = document.createElement("section");
      offCard.className = "settings-card";
      var offRow = document.createElement("div");
      offRow.className = "settings-row";
      var offLeft = document.createElement("span");
      offLeft.className = "text-secondary";
      offLeft.textContent = s.hotspot_enabled ? t("wifi.offHotspotText") : t("wifi.offText");
      var turnOnBtn = document.createElement("button");
      turnOnBtn.type = "button";
      turnOnBtn.className = "btn btn-sm";
      turnOnBtn.disabled = busyState;
      turnOnBtn.textContent = t("wifi.turnOn");
      turnOnBtn.addEventListener("click", function () {
        handleSwitchChange("wifi", true, null);
      });
      offRow.appendChild(offLeft);
      offRow.appendChild(turnOnBtn);
      offCard.appendChild(offRow);
      picker.appendChild(offCard);
      return;
    }

    var otherNetworks = scanOrdered.filter(function (n) {
      return n.ssid !== currentSsid && !savedSsidMap[n.ssid];
    });

    if (otherNetworks.length > 0 || scannedWifi !== null) {
      var otherSec = createWifiSection(t("wifi.groupOther"));
      if (!otherNetworks.length) {
        var emptyRow = document.createElement("div");
        emptyRow.className = "wifi-empty-row";
        emptyRow.textContent = t("wifi.noNearby");
        otherSec.card.appendChild(emptyRow);
      } else {
        otherNetworks.forEach(function (n) {
          var secRaw = n.security || "";
          var supported = !/WEP|EAP/.test(secRaw) && (!/SAE/.test(secRaw) || /PSK/.test(secRaw));
          var secure = /WPA|PSK|SAE/.test(secRaw);
          var isExpanded = supported && secure && expandedWifiSsid === n.ssid;

          var item = document.createElement("div");
          item.className = "wifi-item" + (isExpanded ? " is-expanded" : "");

          var row = document.createElement("div");
          row.className = "wifi-row" + (supported ? " is-clickable" : "");

          var left = document.createElement("div");
          left.className = "wifi-row-left";
          var sig = createWifiSignalSvg(rssiToBars(n.rssi), false);
          if (typeof n.rssi === "number") sig.title = n.rssi + " dBm";
          left.appendChild(sig);

          var nameWrap = document.createElement("div");
          nameWrap.className = "wifi-name-wrap";
          var nameEl = document.createElement("span");
          nameEl.className = "wifi-name";
          nameEl.textContent = n.ssid;
          nameWrap.appendChild(nameEl);

          if (n.is5g) {
            var b5 = document.createElement("span");
            b5.className = "wifi-band-tag";
            b5.textContent = "5G";
            nameWrap.appendChild(b5);
          }
          if (!supported) {
            var unsup = document.createElement("span");
            unsup.className = "status-tag";
            unsup.textContent = t("wifi.unsupported");
            nameWrap.appendChild(unsup);
          }
          left.appendChild(nameWrap);

          var right = document.createElement("div");
          right.className = "wifi-row-right";
          if (secure) {
            right.appendChild(createLockSvg());
          }
          if (supported && secure) {
            right.appendChild(createChevronSvg());
          } else if (supported && !secure) {
            var openJoinBtn = document.createElement("button");
            openJoinBtn.type = "button";
            openJoinBtn.className = "btn btn-sm";
            openJoinBtn.disabled = busyState;
            openJoinBtn.textContent = t("wifi.join");
            openJoinBtn.addEventListener("click", function (e) {
              e.stopPropagation();
              connectWifiWithConfirm({ ssid: n.ssid, security: "open", password: "" });
            });
            right.appendChild(openJoinBtn);
          }

          if (supported) {
            row.addEventListener("click", function () {
              if (busyState) return;
              if (!secure) {
                connectWifiWithConfirm({ ssid: n.ssid, security: "open", password: "" });
                return;
              }
              expandedWifiSsid = expandedWifiSsid === n.ssid ? "" : n.ssid;
              renderWifiPicker();
              if (expandedWifiSsid === n.ssid) {
                var pwdInput = picker.querySelector("[data-inline-wifi-pwd]");
                if (pwdInput) pwdInput.focus();
              }
            });
          }

          row.appendChild(left);
          row.appendChild(right);
          item.appendChild(row);

          if (isExpanded) {
            var joinForm = document.createElement("form");
            joinForm.className = "wifi-inline-join";
            joinForm.noValidate = true;

            var pwd = document.createElement("input");
            pwd.type = "text";
            pwd.className = "input mono";
            pwd.autocomplete = "off";
            pwd.maxLength = 63;
            pwd.placeholder = t("wifi.pwdPlaceholder");
            pwd.setAttribute("data-inline-wifi-pwd", "1");

            var cancelBtn = document.createElement("button");
            cancelBtn.type = "button";
            cancelBtn.className = "btn btn-sm";
            cancelBtn.textContent = t("common.cancel");
            cancelBtn.addEventListener("click", function () {
              expandedWifiSsid = "";
              renderWifiPicker();
            });

            var submitBtn = document.createElement("button");
            submitBtn.type = "submit";
            submitBtn.className = "btn btn-sm btn-primary";
            submitBtn.textContent = t("wifi.join");

            joinForm.addEventListener("submit", function (e) {
              e.preventDefault();
              connectWifiWithConfirm({
                ssid: n.ssid,
                security: "wpa2",
                password: pwd.value
              });
            });

            joinForm.appendChild(pwd);
            joinForm.appendChild(cancelBtn);
            joinForm.appendChild(submitBtn);
            item.appendChild(joinForm);
          }

          otherSec.card.appendChild(item);
        });
      }

      picker.appendChild(otherSec.wrap);
    }
  }

  /* =========================================================
     Tailscale UI Renderer (#vpn)
     ========================================================= */
  function renderTailscale(ts) {
    var info = ts || { installed: false, enabled: false, running: false, backend_state: "Stopped" };
    var dot = el("ts-dot");
    var stateTxt = el("ts-state-text");
    var ipRow = el("ts-ip-row");
    var accRow = el("ts-account-row");
    var authRow = el("ts-auth-url-row");
    var actionsRow = el("ts-actions-row");
    var loginBtn = el("ts-login-btn");
    var keyToggle = el("ts-key-toggle");
    var logoutBtn = el("ts-logout-btn");

    if (!info.installed) {
      dot.className = "dot";
      stateTxt.textContent = t("vpn.tsNotInstalled");
      ipRow.hidden = true;
      accRow.hidden = true;
      authRow.hidden = true;
      actionsRow.hidden = true;
      updateSwitchState("tailscale", false, t("vpn.tsNotInstalled"), true);
      updateSwitchState("tailscale_ssh", false, t("vpn.tsNotInstalled"), true);
      setText("dev-ts-ip", "—");
      stopTailscaleAuthPoll();
      return;
    }

    actionsRow.hidden = false;
    var connected = !!(info.running && info.backend_state === "Running");
    var needsLogin = !!(info.auth_url || info.backend_state === "NeedsLogin");

    if (!info.running || info.backend_state === "Stopped") {
      dot.className = "dot";
      stateTxt.textContent = t("common.off");
    } else if (connected) {
      dot.className = "dot ok";
      stateTxt.textContent = t("vpn.tsConnected");
    } else if (needsLogin) {
      dot.className = "dot warn";
      stateTxt.textContent = t("vpn.tsNeedsLogin");
    } else if (info.backend_state === "Starting") {
      dot.className = "dot warn";
      stateTxt.textContent = t("vpn.tsStarting");
    } else {
      dot.className = "dot warn";
      stateTxt.textContent = info.backend_state || "—";
    }

    var tsSwitchVal = !info.running || info.backend_state === "Stopped"
      ? t("common.off")
      : connected
        ? (info.ip || t("vpn.tsConnected"))
        : (needsLogin ? t("vpn.tsNeedsLogin") : t("vpn.tsStarting"));
    updateSwitchState("tailscale", !!(info.enabled && info.running), tsSwitchVal, false);
    var sshAvailable = !!(info.running && info.ssh_available);
    var sshValue = sshAvailable
      ? t(info.ssh_enabled ? "common.on" : "common.off")
      : t(info.running ? "vpn.tsSSHUnavailable" : "vpn.tsSSHNeedsTailscale");
    updateSwitchState("tailscale_ssh", !!info.ssh_enabled, sshValue, !sshAvailable);
    setText("dev-ts-ip", info.ip || "—");

    if (info.ip) {
      ipRow.hidden = false;
      setText("ts-ip-val", info.ip);
    } else {
      ipRow.hidden = true;
    }

    var nodeParts = [];
    if (info.account) nodeParts.push(info.account);
    if (info.dns_name) nodeParts.push(info.dns_name);
    else if (info.hostname && info.hostname !== "localhost") nodeParts.push(info.hostname);
    if (nodeParts.length > 0 && info.running && (connected || info.account)) {
      accRow.hidden = false;
      setText("ts-account-val", nodeParts.join(" · "));
    } else {
      accRow.hidden = true;
    }

    if (info.auth_url) {
      authRow.hidden = false;
      el("ts-auth-link").href = info.auth_url;
      el("ts-auth-link").textContent = info.auth_url;
      el("ts-auth-open-btn").href = info.auth_url;
    } else {
      authRow.hidden = true;
    }

    loginBtn.hidden = connected;
    keyToggle.hidden = connected;
    if (connected) {
      el("ts-key-drawer").hidden = true;
    }
    logoutBtn.hidden = !(connected || info.account);

    if (info.auth_url && !connected) {
      startTailscaleAuthPoll();
    } else {
      stopTailscaleAuthPoll();
    }
  }

  function startTailscaleAuthPoll() {
    if (tsAuthPollTimer) return;
    tsAuthPollTimer = setInterval(function () {
      if (busyState || document.hidden) return;
      request("status").then(function (s) {
        var prevNeeds = !!(state && state.tailscale && state.tailscale.auth_url);
        renderStatus(s, false);
        if (prevNeeds && s.tailscale && s.tailscale.backend_state === "Running") {
          setNotice("ok", "vpn.tsLoggedIn", true);
          stopTailscaleAuthPoll();
        }
      }).catch(function () {});
    }, 4000);
  }

  function stopTailscaleAuthPoll() {
    if (tsAuthPollTimer) {
      clearInterval(tsAuthPollTimer);
      tsAuthPollTimer = null;
    }
  }

  /* =========================================================
     Render Device & Service Status Across All Views
     ========================================================= */
  function renderStatus(s, fillForm) {
    state = s;
    el("app-loading").hidden = true;
    el("app").hidden = false;

    var wifiOk = isWifiConnected(s);
    var imsReg = !!(s.wfc_enabled && s.flight_mode && imsState && imsState.registered === true);
    var supText = describeSupervisor(s.service_state);
    var simText = describeSim(s);
    var ts = s.tailscale || { installed: false, enabled: false, running: false, backend_state: "Stopped" };

    /* 1. #connect OS Switch List */
    updateSwitchState("flight", !!s.flight_mode, "", false);

    var wifiVal = wifiOk
      ? s.ssid
      : s.wifi_enabled
        ? (s.ssid && s.connection === "COMPLETED" ? s.ssid : t("common.disconnected"))
        : t("common.off");
    updateSwitchState("wifi", !!s.wifi_enabled, wifiVal, false);

    var hsVal = s.hotspot_enabled
      ? (s.hotspot_clients > 0 ? t("hotspot.clientsFmt", { n: s.hotspot_clients }) : (s.hotspot_ssid || t("common.on")))
      : t("common.off");
    updateSwitchState("hotspot", !!s.hotspot_enabled, hsVal, false);

    updateSwitchState("hotspot_broadcast", s.hotspot_broadcast !== false, "", false);

    var usbVal = s.usb_tether_enabled
      ? (s.usb_ip || "")
      : (s.usb_available === false ? "—" : "");
    updateSwitchState("usb_tether", !!s.usb_tether_enabled, usbVal, !s.usb_tether_enabled && s.usb_available === false);

    updateSwitchState("mobile_data", !!s.mobile_data_enabled, "", false);
    updateSwitchState("data_roaming", !!s.data_roaming_enabled, "", false);

    /* Honest WFC status: NEVER inferred from wfc_enabled alone */
    var wfcVal = !s.wfc_enabled
      ? t("common.off")
      : !s.flight_mode
        ? t("cellular.wfcPaused")
        : imsReg
          ? t("cellular.wfcRegistered")
          : t("cellular.wfcNotRegistered");
    updateSwitchState("wfc", !!s.wfc_enabled, wfcVal, false);

    /* 2. #wifi View */
    renderWifiPicker();

    /* 3. #hotspot View */
    var hsMeta = "";
    if (s.hotspot_enabled) {
      hsMeta = (s.hotspot_ip || "") + (s.hotspot_clients > 0 ? (s.hotspot_ip ? " · " : "") + t("hotspot.clientsFmt", { n: s.hotspot_clients }) : "");
    }
    el("hotspot-live-meta").textContent = hsMeta;
    if (fillForm || !hotspotDirty) {
      populateHotspotForm(s);
    } else {
      syncHotspotControls();
    }

    /* 4. #cellular View (Wi-Fi Calling + ePDG) */
    var cellDot = "";
    var cellState = "";
    if (!s.wfc_enabled) {
      cellDot = "";
      cellState = t("common.off");
    } else if (!s.flight_mode) {
      cellDot = "warn";
      cellState = t("cellular.wfcPaused");
    } else if (imsReg) {
      cellDot = "ok";
      cellState = t("cellular.wfcRegistered");
    } else {
      cellDot = "warn";
      cellState = t("cellular.wfcNotRegistered") + " · " + supText;
    }
    el("cell-wfc-dot").className = "dot " + cellDot;
    setText("cell-wfc-state", cellState);
    setText("cell-sim-state", simText);

    var proxyInfo = s.proxy || { mode: "proxy", epdg: "84.246.115.151", uri: "" };
    if (!busyState) {
      el("cell-epdg").value = proxyInfo.epdg || "84.246.115.151";
    }

    /* 5. #vpn View (Tailscale + Multi-Protocol Proxy) */
    renderTailscale(ts);
    var sshInfo = s.ssh || {};
    var sshValue = !sshInfo.installed ? t("vpn.tsNotInstalled")
      : sshInfo.running ? t("vpn.sshPort")
      : sshInfo.enabled ? t("vpn.sshStarting") : t("common.off");
    updateSwitchState("ssh", !!sshInfo.running, sshValue, !sshInfo.installed);
    if (fillForm || !isProxyFormDirty()) {
      setProxyRadio(proxyInfo.mode === "direct" ? "direct" : "proxy");
      el("proxy-uri").value = proxyInfo.uri || "";
      el("proxy-wfc-only").checked = !!proxyInfo.wfc_only;
    }
    syncProxyControls();
    renderIPSync(s.ip_sync || {}, fillForm);

    /* 7. #about View */
    setText("dev-model", s.model || "SP970-B");
    setText("dev-android", s.android ? "Android " + s.android + " (SDK " + s.sdk + ")" : "—");
    setText("dev-firmware", s.firmware || "—");
    setText("dev-sim", simText);
    setText("dev-wlan-ip", wifiOk ? s.ipv4 : (s.hotspot_enabled && s.hotspot_ip ? s.hotspot_ip + " (AP)" : "—"));
    setText("dev-usb-ip", s.usb_tether_enabled && s.usb_ip ? s.usb_ip : "—");
    setText("dev-ts-ip", ts && ts.ip ? ts.ip : "—");
    setText(
      "dev-uptime",
      s.uptime_seconds == null
        ? "—"
        : t("device.uptimeFmt", {
            h: Math.floor(s.uptime_seconds / 3600),
            m: Math.floor((s.uptime_seconds % 3600) / 60)
          })
    );
    setText("dev-time", s.device_time ? new Date(s.device_time).toLocaleString(currentLang === "en" ? "en-GB" : "zh-CN", { hour12: false }) : "—");
  }

  function refreshStatus(fillForm) {
    return request("status").then(function (s) {
      if (!s.wfc_enabled || !s.flight_mode) {
        imsState = null;
        renderStatus(s, fillForm);
        return;
      }
      return request("status", undefined, true, 15000)
        .then(function (ims) {
          imsState = ims;
        }, function () {
          imsState = null;
        })
        .then(function () {
          renderStatus(s, fillForm);
        });
    });
  }

  /* =========================================================
     Messages (#sms): Inbox / Sent / Archive / Forwarding Tab
     ========================================================= */
  function formatShortTime(ts) {
    if (!ts) return "—";
    var d = new Date(ts);
    var pad = function (n) { return n < 10 ? "0" + n : String(n); };
    return pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
  }

  function formatFullTime(ts) {
    if (!ts) return "—";
    return new Date(ts).toLocaleString(currentLang === "en" ? "en-GB" : "zh-CN", { hour12: false });
  }

  function syncSmsTabsUI() {
    document.querySelectorAll("[data-sms-box]").forEach(function (tab) {
      tab.classList.toggle("active", tab.dataset.smsBox === smsBox);
    });
    var isForward = smsBox === "forward";
    el("sms-box-pane").hidden = isForward;
    el("sms-forward-pane").hidden = !isForward;
    el("compose-open").hidden = isForward;
    if (isForward) {
      el("compose-panel").hidden = true;
    }
  }

  function renderSMS(data) {
    smsData = data;
    var messages = data.messages || [];
    var unread = data.unread || 0;

    el("sms-badge").hidden = !unread;
    setText("sms-badge", String(unread));
    syncSmsTabsUI();

    if (smsBox === "forward") return;

    var emptyEl = el("sms-empty");
    var workspace = el("sms-workspace");

    if (!messages.length) {
      workspace.hidden = true;
      emptyEl.hidden = false;
      emptyEl.textContent = t(
        smsBox === "sent" ? "sms.emptySent" : smsBox === "archive" ? "sms.emptyArchive" : "sms.emptyInbox"
      );
      selectedSmsId = null;
      return;
    }

    emptyEl.hidden = true;
    workspace.hidden = false;

    var selectedMsg = null;
    for (var i = 0; i < messages.length; i++) {
      if (messages[i].id === selectedSmsId) {
        selectedMsg = messages[i];
        break;
      }
    }
    if (!selectedMsg) {
      selectedMsg = messages[0];
      selectedSmsId = selectedMsg.id;
      mobileReading = false;
    }

    workspace.classList.toggle("mobile-reading", mobileReading);

    var listPane = el("sms-list");
    listPane.innerHTML = "";
    messages.forEach(function (m) {
      var isUnread = !m.read && m.type === 1;
      var btn = document.createElement("button");
      btn.type = "button";
      btn.setAttribute("role", "option");
      btn.setAttribute("aria-selected", m.id === selectedSmsId ? "true" : "false");
      btn.className = "sms-row" + (m.id === selectedSmsId ? " active" : "") + (isUnread ? " unread" : "");

      var top = document.createElement("div");
      top.className = "sms-row-top";

      var addrWrap = document.createElement("span");
      addrWrap.className = "sms-row-addr";
      if (isUnread) {
        var dot = document.createElement("i");
        dot.className = "sms-unread-dot";
        addrWrap.appendChild(dot);
      }
      var addrTxt = document.createElement("span");
      addrTxt.textContent = m.address || "—";
      addrWrap.appendChild(addrTxt);

      var timeEl = document.createElement("time");
      timeEl.className = "sms-row-time";
      timeEl.textContent = formatShortTime(m.date);

      top.appendChild(addrWrap);
      top.appendChild(timeEl);

      var snippet = document.createElement("div");
      snippet.className = "sms-row-snippet";
      snippet.textContent = m.body || "";

      btn.appendChild(top);
      btn.appendChild(snippet);

      btn.addEventListener("click", function () {
        selectedSmsId = m.id;
        mobileReading = true;
        renderSMS(smsData);
      });

      listPane.appendChild(btn);
    });

    setText("sms-reader-addr", selectedMsg.address || "—");
    var meta = el("sms-reader-meta");
    meta.innerHTML = "";

    var dirTag = document.createElement("span");
    dirTag.className = "status-tag";
    dirTag.textContent = selectedMsg.type === 2 ? t("sms.dirOutgoing") : t("sms.dirIncoming");
    meta.appendChild(dirTag);

    if (!selectedMsg.read && selectedMsg.type === 1) {
      var unreadTag = document.createElement("span");
      unreadTag.className = "status-tag info";
      unreadTag.textContent = t("sms.tagUnread");
      meta.appendChild(unreadTag);
    }

    if (selectedMsg.archived) {
      var archTag = document.createElement("span");
      archTag.className = "status-tag warn";
      archTag.textContent = t("sms.tagArchived");
      meta.appendChild(archTag);
    }

    var fullDate = document.createElement("time");
    fullDate.className = "tabular";
    fullDate.textContent = formatFullTime(selectedMsg.date);
    meta.appendChild(fullDate);

    var actions = el("sms-reader-actions");
    actions.innerHTML = "";

    if (selectedMsg.address) {
      var replyBtn = document.createElement("button");
      replyBtn.type = "button";
      replyBtn.className = "btn btn-sm";
      replyBtn.textContent = t("sms.reply");
      replyBtn.addEventListener("click", function () {
        el("compose-panel").hidden = false;
        el("recipient").value = selectedMsg.address;
        el("message").focus();
      });
      actions.appendChild(replyBtn);
    }

    if (!selectedMsg.read) {
      var readBtn = document.createElement("button");
      readBtn.type = "button";
      readBtn.className = "btn btn-sm";
      readBtn.textContent = t("sms.markRead");
      readBtn.addEventListener("click", function () {
        request("sms/read", { id: selectedMsg.id })
          .then(loadSMS)
          .catch(function (err) { setNotice("error", err.message, false); });
      });
      actions.appendChild(readBtn);
    }

    var archBtn = document.createElement("button");
    archBtn.type = "button";
    archBtn.className = "btn btn-sm";
    archBtn.textContent = selectedMsg.archived
      ? t(selectedMsg.type === 2 ? "sms.restoreSent" : "sms.restoreInbox")
      : t("sms.archiveBtn");
    archBtn.addEventListener("click", function () {
      request("sms/archive", { id: selectedMsg.id, archived: !selectedMsg.archived })
        .then(function () {
          mobileReading = false;
          return loadSMS();
        })
        .catch(function (err) { setNotice("error", err.message, false); });
    });
    actions.appendChild(archBtn);

    var deleteBtn = document.createElement("button");
    deleteBtn.type = "button";
    deleteBtn.className = "btn btn-sm btn-danger";
    deleteBtn.textContent = t("sms.deleteBtn");
    deleteBtn.addEventListener("click", function () {
      var messageId = selectedMsg.id;
      openConfirm({
        titleKey: "sms.deleteTitle",
        bodyKey: "sms.deleteBody",
        okKey: "sms.deleteBtn",
        danger: true,
        onConfirm: function () {
          busy(function () {
            return request("sms/delete", { id: messageId });
          }).then(function () {
            if (selectedSmsId === messageId) selectedSmsId = null;
            mobileReading = false;
            return loadSMS();
          }).then(function () {
            setNotice("ok", "sms.deletedNotice", true);
          }).catch(function (err) {
            return loadSMS().catch(function () {}).then(function () {
              setNotice("error", err.message, false);
            });
          });
        }
      });
    });
    actions.appendChild(deleteBtn);

    el("sms-reader-body").textContent = selectedMsg.body || "";
  }

  function loadSMS() {
    syncSmsTabsUI();
    if (smsBox === "forward") {
      return editor.load(!editor.loaded);
    }
    var archived = smsBox === "archive";
    var box = smsBox === "sent" ? "sent" : "inbox";
    return request("sms?box=" + (archived ? "" : box) + "&archived=" + (archived ? "1" : "0"))
      .then(function (data) {
        renderSMS(data);
      });
  }

  /* =========================================================
     eSIM Profiles Controller (#cellular): List / Add / Rename / Disable / Delete
     ========================================================= */
  function applyEuiccState(data) {
    if (data && Array.isArray(data.profiles)) {
      euiccData = data.profiles;
      if (typeof data.eid === "string" && data.eid) {
        euiccEid = data.eid;
      }
    }
  }

  function renderProfiles() {
    var prog = el("euicc-progress");
    var progTxt = el("euicc-progress-text");
    var listEl = el("euicc-list");
    var emptyEl = el("euicc-empty");
    var refreshBtn = el("euicc-refresh");
    var addBtn = el("euicc-add-btn");

    refreshBtn.disabled = !!euiccLoading || busyState;
    addBtn.disabled = !!euiccLoading || busyState;
    el("euicc-eid").textContent = euiccEid ? "EID " + euiccEid : "";

    if (euiccLoading === "read") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.readingProgress");
    } else if (euiccLoading === "switch") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.switchingProgressFmt", { name: euiccTargetName });
    } else if (euiccLoading === "disable") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.disablingProgressFmt", { name: euiccTargetName });
    } else if (euiccLoading === "rename") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.renamingProgress");
    } else if (euiccLoading === "delete") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.deletingProgressFmt", { name: euiccTargetName });
    } else if (euiccLoading === "download") {
      prog.hidden = false;
      progTxt.textContent = t("euicc.downloadingProgress");
    } else {
      prog.hidden = true;
    }

    if (euiccError && !euiccData) {
      listEl.hidden = true;
      emptyEl.hidden = false;
      emptyEl.textContent = translateServerMsg(euiccError, currentLang);
      return;
    }

    if (!euiccData) {
      listEl.hidden = true;
      emptyEl.hidden = true;
      return;
    }

    if (!euiccData.length) {
      listEl.hidden = true;
      emptyEl.hidden = false;
      emptyEl.textContent = t("euicc.empty");
      return;
    }

    emptyEl.hidden = true;
    listEl.hidden = false;
    listEl.innerHTML = "";

    euiccData.forEach(function (p) {
      var row = document.createElement("div");
      row.className = "esim-row" + (p.enabled ? " is-active" : "");

      var info = document.createElement("div");
      info.className = "esim-info";

      var titleLine = document.createElement("div");
      titleLine.className = "esim-title-line";

      var nameEl = document.createElement("span");
      nameEl.className = "esim-name";
      nameEl.textContent = p.name || "eSIM";
      titleLine.appendChild(nameEl);

      if (p.enabled) {
        var activeTag = document.createElement("span");
        activeTag.className = "status-tag ok";
        var dot = document.createElement("i");
        dot.className = "dot ok";
        var tagTxt = document.createElement("span");
        tagTxt.textContent = t("euicc.stateActive");
        activeTag.appendChild(dot);
        activeTag.appendChild(tagTxt);
        titleLine.appendChild(activeTag);
      }

      if (p.provider && p.provider !== p.name) {
        var provEl = document.createElement("span");
        provEl.className = "esim-provider";
        provEl.textContent = p.provider;
        titleLine.appendChild(provEl);
      }

      var iccidEl = document.createElement("div");
      iccidEl.className = "esim-iccid";
      iccidEl.textContent = p.iccid || "—";

      info.appendChild(titleLine);
      info.appendChild(iccidEl);

      var actions = document.createElement("div");
      actions.className = "esim-actions";
      var btnDisabled = !!euiccLoading || busyState;

      /* Rename button */
      var renameBtn = document.createElement("button");
      renameBtn.type = "button";
      renameBtn.className = "btn btn-sm";
      renameBtn.disabled = btnDisabled;
      renameBtn.textContent = t("euicc.renameBtn");
      renameBtn.addEventListener("click", function () {
        openConfirm({
          titleKey: "euicc.renameTitle",
          bodyKey: "euicc.renameBodyFmt",
          okKey: "common.save",
          inputMode: true,
          inputValue: p.nickname || p.name || "",
          placeholderKey: "euicc.renamePlaceholder",
          vars: { iccid: p.iccid || "" },
          onConfirm: function (newNickname) {
            euiccLoading = "rename";
            renderProfiles();
            busy(function () {
              return request("euicc/nickname", { id: p.id, iccid: p.iccid, nickname: newNickname });
            }).then(function (res) {
              applyEuiccState(res);
              setNotice("ok", "euicc.renamedNotice", true);
            }).catch(function (err) {
              setNotice("error", err.message, false);
            }).finally(function () {
              euiccLoading = null;
              renderProfiles();
            });
          }
        });
      });
      actions.appendChild(renameBtn);

      if (p.enabled) {
        /* Disable button */
        var disableBtn = document.createElement("button");
        disableBtn.type = "button";
        disableBtn.className = "btn btn-sm";
        disableBtn.disabled = btnDisabled;
        disableBtn.textContent = t("euicc.disableBtn");
        disableBtn.addEventListener("click", function () {
          openConfirm({
            titleKey: "euicc.disableTitle",
            bodyKey: "euicc.disableBodyFmt",
            okKey: "euicc.disableBtn",
            vars: { name: p.name || "eSIM" },
            onConfirm: function () {
              euiccLoading = "disable";
              euiccTargetName = p.name || "eSIM";
              renderProfiles();
              busy(function () {
                return request("euicc/disable", { id: p.id });
              }).then(function (res) {
                applyEuiccState(res);
                setNotice("ok", "euicc.disabledNotice", true);
                return refreshStatus(false);
              }).catch(function (err) {
                setNotice("error", err.message, false);
              }).finally(function () {
                euiccLoading = null;
                renderProfiles();
              });
            }
          });
        });
        actions.appendChild(disableBtn);
      } else {
        /* Enable button */
        var enableBtn = document.createElement("button");
        enableBtn.type = "button";
        enableBtn.className = "btn btn-sm btn-primary";
        enableBtn.disabled = btnDisabled;
        enableBtn.textContent = t("euicc.enableBtn");
        enableBtn.addEventListener("click", function () {
          openConfirm({
            titleKey: "euicc.enableTitle",
            bodyKey: "euicc.enableBodyFmt",
            okKey: "euicc.enableBtn",
            vars: { name: p.name || "eSIM", iccid: p.iccid || "" },
            onConfirm: function () {
              euiccLoading = "switch";
              euiccTargetName = p.name || "eSIM";
              renderProfiles();
              busy(function () {
                return request("euicc/enable", { id: p.id });
              }).then(function (result) {
                if (result && result.message) {
                  setNotice("ok", result.message, false);
                } else {
                  setNotice("ok", "euicc.switchedDefault", true);
                }
                return loadProfiles(true);
              }).then(function () {
                return refreshStatus(false);
              }).catch(function (err) {
                setNotice("error", err.message, false);
              }).finally(function () {
                euiccLoading = null;
                renderProfiles();
              });
            }
          });
        });
        actions.appendChild(enableBtn);

        /* Delete button */
        var deleteBtn = document.createElement("button");
        deleteBtn.type = "button";
        deleteBtn.className = "btn btn-sm btn-danger";
        deleteBtn.disabled = btnDisabled;
        deleteBtn.textContent = t("euicc.deleteBtn");
        deleteBtn.addEventListener("click", function () {
          openConfirm({
            titleKey: "euicc.deleteTitle",
            bodyKey: "euicc.deleteBodyFmt",
            okKey: "euicc.deleteBtn",
            danger: true,
            vars: { name: p.name || "eSIM", iccid: p.iccid || "" },
            onConfirm: function () {
              euiccLoading = "delete";
              euiccTargetName = p.name || "eSIM";
              renderProfiles();
              busy(function () {
                return request("euicc/delete", { id: p.id, iccid: p.iccid });
              }).then(function (res) {
                applyEuiccState(res);
                setNotice("ok", "euicc.deletedNotice", true);
              }).catch(function (err) {
                setNotice("error", err.message, false);
              }).finally(function () {
                euiccLoading = null;
                renderProfiles();
              });
            }
          });
        });
        actions.appendChild(deleteBtn);
      }

      row.appendChild(info);
      row.appendChild(actions);
      listEl.appendChild(row);
    });
  }

  function loadProfiles(force) {
    if (euiccLoading === "read") return Promise.resolve();
    if (!force && euiccData) {
      renderProfiles();
      return Promise.resolve();
    }
    euiccLoading = "read";
    euiccError = null;
    renderProfiles();
    return request("euicc").then(function (data) {
      applyEuiccState(data);
      if (!euiccData) euiccData = [];
      euiccLoading = null;
      renderProfiles();
    }).catch(function (err) {
      euiccLoading = null;
      euiccError = err.message;
      renderProfiles();
      if (currentView === "cellular") {
        setNotice("error", err.message, false);
      }
    });
  }

  /* =========================================================
     Routing & Language Switching
     ========================================================= */
  function updatePageHeading() {
    setText("page-title", lookup(currentLang, "views." + currentView));
  }

  function route() {
    if (esimQR) esimQR.close();
    var resolved = normalizeRoute(location.hash);
    currentView = resolved.view;
    if (resolved.smsBox) {
      smsBox = resolved.smsBox;
    }
    closeDrawer();
    closeAccountMenu();

    var contentMain = document.querySelector("main.content");
    if (contentMain) {
      contentMain.classList.toggle("is-wide", currentView === "sms");
    }

    document.querySelectorAll("[data-view]").forEach(function (viewEl) {
      viewEl.hidden = viewEl.dataset.view !== currentView;
    });
    document.querySelectorAll(".sidebar-nav [data-page]").forEach(function (link) {
      var active = link.dataset.page === currentView;
      link.classList.toggle("active", active);
      if (active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });

    updatePageHeading();
    setNotice(null);

    if (currentView === "wifi") {
      if (state && state.wifi_enabled && scannedWifi === null && !autoScannedOnce && !busyState) {
        autoScannedOnce = true;
        triggerWifiScan(true);
      }
    } else if (currentView === "cellular") {
      loadProfiles(false);
    } else if (currentView === "sms") {
      loadSMS().catch(function (err) { setNotice("error", err.message, false); });
    }
  }

  function setLang(lang) {
    currentLang = lang === "en" ? "en" : "zh";
    localStorage.setItem(LANG_KEY, currentLang);
    applyStaticI18n();
    syncSwitchLabels();
    updatePageHeading();
    syncHotspotControls();
    editor.setLang(currentLang);
    if (esimQR) esimQR.syncText();
    if (state) renderStatus(state, false);
    if (smsData) renderSMS(smsData);
    else syncSmsTabsUI();
    renderProfiles();
    renderConfirm();
    renderNotice();
  }

  document.querySelectorAll(".lang-switch button[data-lang]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      setLang(this.dataset.lang);
    });
  });

  /* =========================================================
     Event Bindings
     ========================================================= */
  el("logout").addEventListener("click", function () {
    closeAccountMenu();
    request("logout", {}).then(function () {
      sessionStorage.removeItem(SESSION_KEY);
      sessionStorage.removeItem(ACCOUNT_KEY);
      location.replace("/login.html");
    }).catch(function (err) {
      setNotice("error", err.message, false);
    });
  });

  el("refresh").addEventListener("click", function () {
    var icon = el("refresh-icon");
    icon.classList.add("spin");
    refreshStatus(!isProxyFormDirty() && !hotspotDirty).then(function () {
      if (currentView === "wifi" && state && state.wifi_enabled && !busyState) {
        return triggerWifiScan(true);
      }
      if (currentView === "cellular") return loadProfiles(true);
      if (currentView === "sms") {
        if (smsBox === "forward") return editor.load(!editor.dirty);
        return loadSMS();
      }
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      icon.classList.remove("spin");
    });
  });

  /* Hotspot Form (#hotspot) */
  ["hotspot-ssid", "hotspot-security", "hotspot-password"].forEach(function (id) {
    el(id).addEventListener("input", function () {
      hotspotDirty = true;
      syncHotspotControls();
    });
    el(id).addEventListener("change", function () {
      hotspotDirty = true;
      syncHotspotControls();
    });
  });

  el("hotspot-form").addEventListener("submit", function (e) {
    e.preventDefault();
    var saveBtn = el("hotspot-cfg-save");
    var sec = el("hotspot-security").value;
    saveBtn.textContent = t("common.saving");
    busy(function () {
      return request("hotspot/config", {
        ssid: el("hotspot-ssid").value.trim(),
        security: sec,
        password: sec === "open" ? "" : el("hotspot-password").value
      });
    }).then(function () {
      hotspotDirty = false;
      setNotice("ok", "hotspot.savedNotice", true);
      return refreshStatus(true);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      saveBtn.textContent = t("common.save");
    });
  });

  /* Cellular ePDG Selector (#cellular) */
  el("cell-epdg").addEventListener("change", function () {
    var newEpdg = el("cell-epdg").value;
    var curMode = (state && state.proxy && state.proxy.mode) || "proxy";
    busy(function () {
      return request("network", {
        mode: curMode,
        uri: "",
        epdg: newEpdg,
        wfc_only: !!(state && state.proxy && state.proxy.wfc_only)
      });
    }).then(function () {
      setNotice("ok", "cellular.epdgSaved", true);
      return refreshStatus(false);
    }).catch(function (err) {
      setNotice("error", err.message, false);
      if (state && state.proxy) {
        el("cell-epdg").value = state.proxy.epdg || "84.246.115.151";
      }
    });
  });

  /* eSIM Add & Refresh (#cellular) */
  el("euicc-add-btn").addEventListener("click", function () {
    var drawer = el("euicc-add-drawer");
    drawer.hidden = !drawer.hidden;
    if (!drawer.hidden) {
      el("euicc-code").focus();
    } else if (esimQR) esimQR.close();
  });

  el("euicc-qr-open").addEventListener("click", function () {
    if (esimQR) esimQR.open();
    else setNotice("error", "euicc.qrLoadError", true);
  });

  el("euicc-add-cancel").addEventListener("click", function () {
    if (esimQR) esimQR.close();
    el("euicc-add-drawer").hidden = true;
    el("euicc-code").value = "";
    el("euicc-new-nickname").value = "";
    el("euicc-confirm-code").value = "";
  });

  el("euicc-add-form").addEventListener("submit", function (e) {
    e.preventDefault();
    if (esimQR) esimQR.close();
    var code = el("euicc-code").value.trim();
    if (!code) {
      setNotice("error", "euicc.codeRequired", true);
      el("euicc-code").focus();
      return;
    }
    var submitBtn = el("euicc-add-submit");
    submitBtn.textContent = t("euicc.downloadingBtn");
    euiccLoading = "download";
    renderProfiles();
    busy(function () {
      return request("euicc/download", {
        code: code,
        nickname: el("euicc-new-nickname").value.trim(),
        confirmation_code: el("euicc-confirm-code").value.trim()
      }, false, 190000);
    }).then(function (res) {
      applyEuiccState(res);
      el("euicc-code").value = "";
      el("euicc-new-nickname").value = "";
      el("euicc-confirm-code").value = "";
      el("euicc-add-drawer").hidden = true;
      setNotice("ok", "euicc.downloadedNotice", true);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      euiccLoading = null;
      submitBtn.textContent = t("euicc.downloadBtn");
      renderProfiles();
    });
  });

  el("euicc-refresh").addEventListener("click", function () {
    loadProfiles(true);
  });

  /* Tailscale Controls (#vpn) */
  el("ts-login-btn").addEventListener("click", function () {
    var btn = el("ts-login-btn");
    btn.textContent = t("vpn.tsLoggingIn");
    busy(function () {
      return request("tailscale/login", {});
    }).then(function (tsRes) {
      if (state) state.tailscale = tsRes;
      renderTailscale(tsRes);
      if (tsRes && tsRes.auth_url) {
        setNotice("ok", "vpn.tsAuthReady", true);
      } else if (tsRes && tsRes.backend_state === "Running") {
        setNotice("ok", "vpn.tsLoggedIn", true);
      }
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      btn.textContent = t("vpn.tsLogin");
    });
  });

  el("ts-key-toggle").addEventListener("click", function () {
    var drawer = el("ts-key-drawer");
    drawer.hidden = !drawer.hidden;
    if (!drawer.hidden) {
      el("ts-auth-key").focus();
    }
  });

  el("ts-key-form").addEventListener("submit", function (e) {
    e.preventDefault();
    var authKey = el("ts-auth-key").value.trim();
    if (!authKey) {
      setNotice("error", "vpn.tsKeyRequired", true);
      el("ts-auth-key").focus();
      return;
    }
    var submitBtn = el("ts-key-submit");
    submitBtn.textContent = t("vpn.tsLoggingIn");
    busy(function () {
      return request("tailscale/login", { auth_key: authKey });
    }).then(function (tsRes) {
      el("ts-auth-key").value = "";
      el("ts-key-drawer").hidden = true;
      if (state) state.tailscale = tsRes;
      renderTailscale(tsRes);
      setNotice("ok", "vpn.tsLoggedIn", true);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      submitBtn.textContent = t("vpn.tsLogin");
    });
  });

  el("ts-logout-btn").addEventListener("click", function () {
    openConfirm({
      titleKey: "vpn.tsLogoutTitle",
      bodyKey: "vpn.tsLogoutBody",
      okKey: "vpn.tsLogout",
      danger: true,
      onConfirm: function () {
        busy(function () {
          return request("tailscale/logout", {});
        }).then(function (tsRes) {
          if (state) state.tailscale = tsRes;
          renderTailscale(tsRes);
          setNotice("ok", "vpn.tsLoggedOut", true);
        }).catch(function (err) {
          setNotice("error", err.message, false);
        });
      }
    });
  });

  /* Proxy Form (#vpn) */
  var ipSyncDirty = false;
  function renderIPSync(info, fillForm) {
    if (fillForm || !ipSyncDirty) {
      el("ip-sync-url").value = info.subscription_url || "";
      el("ip-sync-interval").value = String(info.interval_seconds || 60);
      el("ip-sync-enabled").checked = !!info.enabled;
    }
    setText("ip-sync-state", t("vpn.ipSyncStates." + (info.state || "unconfigured")));
    setText("ip-sync-last-check", info.last_checked_at ? formatFullTime(info.last_checked_at) : "—");
    setText("ip-sync-error", info.error || "");
    el("ip-sync-error-box").hidden = !info.error;
    if (!busyState) el("ip-sync-check").disabled = !info.configured || ipSyncDirty;
  }
  function dirtyIPSync() {
    ipSyncDirty = true;
    el("ip-sync-check").disabled = true;
  }
  el("ip-sync-form").addEventListener("input", dirtyIPSync);
  el("ip-sync-form").addEventListener("change", dirtyIPSync);
  el("ip-sync-form").addEventListener("submit", function (e) {
    e.preventDefault();
    busy(function () {
      return request("ip-sync", {
        enabled: el("ip-sync-enabled").checked,
        subscription_url: el("ip-sync-url").value.trim(),
        interval_seconds: Number(el("ip-sync-interval").value)
      });
    }).then(function (info) {
      ipSyncDirty = false;
      if (state) state.ip_sync = info;
      renderIPSync(info, true);
      setNotice("ok", "vpn.ipSyncSaved", true);
    }).catch(function (err) { setNotice("error", err.message, false); });
  });
  el("ip-sync-check").addEventListener("click", function () {
    busy(function () { return request("ip-sync/check", {}); }).then(function (info) {
      if (state) state.ip_sync = info;
      renderIPSync(info, false);
      setNotice("ok", "vpn.ipSyncChecked", true);
      return refreshStatus(false);
    }).catch(function (err) { setNotice("error", err.message, false); });
  });
  el("proxy-form").addEventListener("input", syncProxyControls);
  el("proxy-form").addEventListener("change", syncProxyControls);

  el("proxy-reset-btn").addEventListener("click", function () {
    if (!state || !state.proxy) return;
    setProxyRadio(state.proxy.mode === "direct" ? "direct" : "proxy");
    el("proxy-uri").value = state.proxy.uri || "";
    el("proxy-wfc-only").checked = !!state.proxy.wfc_only;
    syncProxyControls();
  });

  el("proxy-form").addEventListener("submit", function (e) {
    e.preventDefault();
    var saveBtn = el("proxy-save-btn");
    var epdgVal = (state && state.proxy && state.proxy.epdg) || el("cell-epdg").value || "84.246.115.151";
    saveBtn.textContent = t("common.saving");
    busy(function () {
      return request("network", {
        mode: selectedProxyMode(),
        uri: el("proxy-uri").value.trim(),
        epdg: epdgVal,
        wfc_only: proxyWFCOnly()
      });
    }).then(function () {
      setNotice("ok", "vpn.savedNotice", true);
      return refreshStatus(true);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      saveBtn.textContent = t("common.save");
      syncProxyControls();
    });
  });

  /* WLAN Actions (#wifi) */
  function connectWifi(body) {
    var btn = el("wifi-connect-btn");
    btn.textContent = t("wifi.joining");
    return busy(function () {
      return request("wifi/connect", body);
    }).then(function (result) {
      el("wifi-password").value = "";
      el("wifi-other-drawer").hidden = true;
      expandedWifiSsid = "";
      if (result && result.message) {
        setNotice("ok", result.message, false);
      } else {
        setNotice("ok", "wifi.connectStarted", true);
      }
      return refreshStatus(false).then(function () {
        setTimeout(function () {
          refreshStatus(false).catch(function () {});
        }, 4500);
      });
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      btn.textContent = t("wifi.join");
    });
  }

  function connectWifiWithConfirm(body) {
    if (state && state.hotspot_enabled) {
      openConfirm({
        titleKey: "confirmations.wifiOnHotspotTitle",
        bodyKey: "confirmations.wifiOnHotspotBody",
        okKey: "confirmations.turnOnBtn",
        onConfirm: function () { connectWifi(body); }
      });
      return;
    }
    connectWifi(body);
  }

  function disconnectWifi() {
    busy(function () {
      return request("wifi/disconnect", {});
    }).then(function () {
      setNotice("ok", "wifi.disconnectedNotice", true);
      return refreshStatus(false);
    }).catch(function (err) {
      setNotice("error", err.message, false);
    });
  }

  function forgetWifi(net) {
    openConfirm({
      titleKey: "wifi.forgetTitle",
      bodyKey: "wifi.forgetBodyFmt",
      okKey: "wifi.forgetBtn",
      danger: true,
      vars: { ssid: net.ssid || "" },
      onConfirm: function () {
        var payload = { ssid: net.ssid || "" };
        if (typeof net.id === "number" && net.id >= 0) {
          payload.network_id = net.id;
        }
        busy(function () {
          return request("wifi/forget", payload);
        }).then(function () {
          setNotice("ok", "wifi.forgottenNotice", true);
          return refreshStatus(false);
        }).catch(function (err) {
          setNotice("error", err.message, false);
        });
      }
    });
  }

  function triggerWifiScan(silentIfOff) {
    var scanIcon = el("wifi-scan-icon");
    var scanLbl = el("wifi-scan-label");
    scanIcon.classList.add("spin");
    scanLbl.textContent = t("wifi.scanning");
    return busy(function () {
      return request("wifi/scan");
    }).then(function (data) {
      if (data && typeof data.wifi_enabled === "boolean" && state) {
        state.wifi_enabled = data.wifi_enabled;
      }
      scannedWifi = data.networks || [];
      renderWifiPicker();
      if (!data.scan_started && !silentIfOff && state && !state.wifi_enabled) {
        setNotice("warn", "wifi.offText", true);
      }
    }).catch(function (err) {
      if (!silentIfOff) setNotice("error", err.message, false);
    }).finally(function () {
      scanIcon.classList.remove("spin");
      scanLbl.textContent = t("wifi.scan");
    });
  }

  el("wifi-other-btn").addEventListener("click", function () {
    var drawer = el("wifi-other-drawer");
    drawer.hidden = !drawer.hidden;
    if (!drawer.hidden) {
      syncWifiSecurityControls();
      el("wifi-ssid").focus();
    }
  });

  el("wifi-other-cancel").addEventListener("click", function () {
    el("wifi-other-drawer").hidden = true;
    el("wifi-password").value = "";
  });

  el("wifi-security").addEventListener("change", syncWifiSecurityControls);

  el("wifi-form").addEventListener("submit", function (e) {
    e.preventDefault();
    var sec = el("wifi-security").value;
    connectWifiWithConfirm({
      ssid: el("wifi-ssid").value.trim(),
      security: sec,
      password: sec === "open" ? "" : el("wifi-password").value
    });
  });

  el("wifi-scan").addEventListener("click", function () {
    triggerWifiScan(false);
  });

  /* Messages Events (#sms) */
  document.querySelectorAll("[data-sms-box]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      smsBox = this.dataset.smsBox;
      selectedSmsId = null;
      mobileReading = false;
      setNotice(null);
      loadSMS().catch(function (err) { setNotice("error", err.message, false); });
    });
  });

  el("sms-back").addEventListener("click", function () {
    mobileReading = false;
    el("sms-workspace").classList.remove("mobile-reading");
  });

  el("compose-open").addEventListener("click", function () {
    el("compose-panel").hidden = false;
    el("recipient").focus();
  });

  el("compose-close").addEventListener("click", function () {
    el("compose-panel").hidden = true;
  });

  el("send-form").addEventListener("submit", function (e) {
    e.preventDefault();
    var sendBtn = el("send-submit");
    sendBtn.textContent = t("sms.sendingBtn");
    busy(function () {
      return request(
        "send",
        {
          To: el("recipient").value.replace(/[\s().-]/g, ""),
          Text: el("message").value
        },
        true
      );
    }).then(function () {
      el("message").value = "";
      el("compose-panel").hidden = true;
      setNotice("ok", "sms.sentNotice", true);
      return loadSMS();
    }).catch(function (err) {
      setNotice("error", err.message, false);
    }).finally(function () {
      sendBtn.textContent = t("sms.sendBtn");
    });
  });

  /* Initial Boot */
  applyStaticI18n();
  syncSwitchLabels();
  syncWifiSecurityControls();
  syncHotspotControls();
  window.addEventListener("hashchange", route);

  var consoleReady = false;
  function refreshConsole() {
    return refreshStatus(!consoleReady).then(function () {
      if (consoleReady) return;
      consoleReady = true;
      route();
      if (currentView !== "sms" || smsBox === "forward") {
        request("sms?box=inbox&archived=0").then(function (data) {
          var unread = data.unread || 0;
          el("sms-badge").hidden = !unread;
          setText("sms-badge", String(unread));
        }).catch(function () {});
      }
    }).catch(function (err) {
      if (!consoleReady) {
        setText("app-loading-text", translateServerMsg(err.message, currentLang));
      }
    });
  }
  refreshConsole();

  setInterval(function () {
    if (!busyState && !document.hidden) {
      refreshConsole();
      if (currentView === "sms" && smsBox === "forward" && editor.loaded && !editor.dirty && !editor.saving) {
        editor.load(false);
      }
    }
  }, 20000);
}());
