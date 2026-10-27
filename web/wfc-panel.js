/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function () {
    "use strict";
    function el(id) { return document.getElementById(id); }
    var token = sessionStorage.getItem("wfc_session");
    function notice(text, bad) { el("notice").textContent = text; el("notice").className = bad ? "error" : ""; }
    function api(path, body) {
        var controller = typeof AbortController === "function" ? new AbortController() : null;
        var timer = controller ? setTimeout(function () { controller.abort(); }, path === "send" ? 90000 : 15000) : null;
        return fetch("/wfc/" + path, {
            method: body === undefined ? "GET" : "POST", cache: "no-store", credentials: "same-origin",
            headers: {"Content-Type": "application/json", "Authorization": "Bearer " + (token || "")},
            body: body === undefined ? undefined : JSON.stringify(body), signal: controller ? controller.signal : undefined
        }).then(function (response) {
            return response.json().catch(function () { return {error: "WFC 正在启动或重连，请稍后重试。"}; }).then(function (data) {
                if (response.status === 401 && path !== "login") { token = null; sessionStorage.removeItem("wfc_session"); showWorkspace(false); }
                if (!response.ok) throw new Error(data.error || "请求失败");
                return data;
            });
        }).then(function (data) { clearTimeout(timer); return data; }, function (error) { clearTimeout(timer); throw error; });
    }
    function showWorkspace(value) { el("workspace").hidden = !value; el("login-section").hidden = value; }
    function status() {
        fetch("/wfc-state", {cache: "no-store"}).then(function (r) { return r.ok ? r.text() : ""; }).then(function (text) { el("service-state").textContent = text; }).catch(function () {});
        if (!token) { el("connection").textContent = "等待登录"; return Promise.resolve(); }
        return api("status").then(function (data) {
            showWorkspace(true); el("connection").textContent = data.registered ? "Wi-Fi Calling 已注册" : "尚未注册";
            el("connection").className = "status" + (data.registered ? " ok" : "");
            el("sim").textContent = data.imsi_prefix; el("received").textContent = data.received; el("sent").textContent = data.submitted;
            el("registration-detail").textContent = "注册有效期 " + data.expires_seconds + " 秒 · 当前会话起于 " + new Date(data.started).toLocaleString() + " · 服务会自动重新注册";
        }).catch(function (error) { el("connection").textContent = "WFC 暂未连接"; el("connection").className = "status"; notice(error.message, true); });
    }
    var editor = new WFCForwardEditor(el("forward-editor"), api);
    function forwarding(fill) { return token ? editor.load(fill) : Promise.resolve(); }
    function busy(id, operation) { el(id).disabled = true; return operation.then(function (value) { el(id).disabled = false; return value; }, function (error) { el(id).disabled = false; throw error; }); }
    el("login-form").addEventListener("submit", function (event) { event.preventDefault(); busy("login-button", api("login", {Account: el("account").value, Password: el("password").value})).then(function (data) { token = data.token; sessionStorage.setItem("wfc_session", token); el("password").value = ""; notice("已登录", false); status(); forwarding(true).catch(function (e) { notice(e.message, true); }); }).catch(function (e) { notice(e.message, true); }); });
    el("refresh").addEventListener("click", function () { status(); forwarding(false).catch(function (e) { notice(e.message, true); }); });
    el("logout").addEventListener("click", function () { token = null; sessionStorage.removeItem("wfc_session"); showWorkspace(false); status(); notice("已退出本浏览器会话", false); });
    el("send-form").addEventListener("submit", function (event) { event.preventDefault(); busy("send-button", api("send", {To: el("recipient").value.replace(/[\s().-]/g, ""), Text: el("message").value})).then(function () { el("message").value = ""; notice("短信已提交运营商", false); status(); }).catch(function (e) { notice(e.message, true); }); });
    showWorkspace(!!token); status(); forwarding(true).catch(function () {});
    setInterval(function () { status(); if (!editor.dirty) forwarding(false).catch(function () {}); }, 15000);
}());
