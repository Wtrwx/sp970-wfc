// WFC transport for the original SP970 SMS page. Credentials remain in the
// existing login flow; only a temporary WFC session token is retained.
(function () {
    "use strict";
    var originalSend = window.sendMessage;
    var lastReceived = null;
    var isSMS = /\/sms-device\.html$/.test(location.pathname);

    function request(path, body, token) {
        var headers = {"Content-Type": "application/json"};
        if (token) headers.Authorization = "Bearer " + token;
        var controller = typeof AbortController === "function" ? new AbortController() : null;
        var timeout = controller ? setTimeout(function () { controller.abort(); }, path === "login" ? 5000 : 90000) : null;
        return fetch("/wfc/" + path, {
            method: body === undefined ? "GET" : "POST",
            headers: headers,
            body: body === undefined ? undefined : JSON.stringify(body),
            credentials: "same-origin",
            signal: controller ? controller.signal : undefined,
            cache: "no-store"
        }).then(function (response) {
            return response.json().catch(function () { return {}; }).then(function (data) {
                if (!response.ok) throw new Error(data.error || "WFC 暂不可用");
                return data;
            });
        }).then(function (data) { clearTimeout(timeout); return data; }, function (error) { clearTimeout(timeout); throw error; });
    }

    function show(text, bad) {
        if (!isSMS) return;
        var box = document.getElementById("wfc-state");
        if (!box) {
            box = document.createElement("div");
            box.id = "wfc-state";
            box.style.cssText = "padding:10px 16px;margin:12px 0;border:1px solid #ddd;border-radius:4px;background:#f7fafb";
            var anchor = document.querySelector("#smslist-main") || document.querySelector(".main") || document.body;
            anchor.insertBefore(box, anchor.firstChild);
        }
        box.textContent = text;
        box.style.color = bad ? "#a33" : "#176b40";
        if (bad) {
            var link = document.createElement("a");
            link.href = "login.html";
            link.textContent = " 重新登录后台";
            box.appendChild(link);
        }
    }

    function login(params) {
        return request("login", {Account: params.account, Password: params.password}).then(function (data) {
            sessionStorage.setItem("wfc_session", data.token);
        });
    }

    if (typeof originalSend === "function") {
        window.sendMessage = function (socket, type, params) {
            if (type === "login" && params) {
                var receiver = this, args = arguments;
                // Finish the WFC session before the native login navigates away.
                return login(params).catch(function () {
                    sessionStorage.removeItem("wfc_session");
                }).then(function () { return originalSend.apply(receiver, args); });
            }
            if (isSMS && type === "sendSMS") {
                var session = sessionStorage.getItem("wfc_session");
                if (!session) {
                    if (window.loading) loading.hide();
                    show("请重新登录后台以启用 WFC 发短信。", true);
                    return;
                }
                var sequence = Promise.resolve();
                (params.phone || []).forEach(function (phone) {
                    sequence = sequence.then(function () {
                        return request("send", {To: String(phone).replace(/[\s().-]/g, ""), Text: params.message}, session);
                    });
                });
                sequence.then(function () {
                    if (window.messageCallback) messageCallback({data: JSON.stringify({type: "sendSMS", data: {irc: params.irc, status: true, report: 0}})});
                    originalSend(socket, "getSMSList", {irc: params.irc});
                    show("Wi-Fi Calling 已注册 · 短信已提交运营商", false);
                }).catch(function (error) {
                    if (window.loading) loading.hide();
                    show(error.message, true);
                    if (window.setGlobalInfo) setGlobalInfo("error", error.message);
                });
                return;
            }
            return originalSend.apply(this, arguments);
        };
    }

    function update() {
        var session = sessionStorage.getItem("wfc_session");
        if (!session) { show("重新登录后台后可使用 Wi-Fi Calling 收发短信。", true); return; }
        request("status", undefined, session).then(function (status) {
            show(status.registered ? "Wi-Fi Calling 已注册 · SIM 23487 · 已收 " + status.received + " 条 · 已发 " + status.submitted + " 条" : "Wi-Fi Calling 尚未注册", !status.registered);
            if (lastReceived !== null && lastReceived !== status.received && window.ws && window.token) originalSend(window.ws, "getSMSList", {irc: hex_md5(window.token)});
            lastReceived = status.received;
        }).catch(function (error) { show(error.message, true); });
    }
    if (isSMS) {
        function ready() {
            var menu = document.querySelector(".nav-list ul");
            if (menu && !menu.querySelector('a[href="wfc-panel.html#forward"]')) {
                var item = document.createElement("li"), link = document.createElement("a");
                link.href = "wfc-panel.html#forward";
                link.textContent = "短信转发";
                item.appendChild(link); menu.appendChild(item);
            }
            update();
        }
        if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", ready);
        else ready();
        setInterval(update, 10000);
    }
}());
