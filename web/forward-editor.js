/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function () {
  "use strict";

  var COPY = {
    zh: {
      enableTitle: "启用 HTTPS 转发",
      loading: "读取中…",
      countsFmt: "已转发 {ok} · 待重试 {pending}",
      unsaved: "待保存更改",
      curlLabel: "cURL 命令",
      curlPlaceholder: "curl -X POST 'https://example.com/webhook' -H 'Authorization: Bearer TOKEN' --data-urlencode 'from={{sms-from}}' --data-urlencode 'body={{sms-body}}'",
      varsPrefix: "变量：",
      save: "保存",
      saving: "保存中…",
      saved: "转发设置已保存",
      waitLoad: "正在读取设置，请稍候",
      requireCurl: "请填写以 curl 开头的 HTTPS 命令",
      lastErrorPrefix: "最近错误："
    },
    en: {
      enableTitle: "Enable HTTPS Forwarding",
      loading: "Loading…",
      countsFmt: "Delivered {ok} · Pending {pending}",
      unsaved: "Unsaved",
      curlLabel: "cURL Command",
      curlPlaceholder: "curl -X POST 'https://example.com/webhook' -H 'Authorization: Bearer TOKEN' --data-urlencode 'from={{sms-from}}' --data-urlencode 'body={{sms-body}}'",
      varsPrefix: "Variables: ",
      save: "Save",
      saving: "Saving…",
      saved: "Forwarding saved",
      waitLoad: "Please wait for settings to load",
      requireCurl: "Enter a valid HTTPS curl command",
      lastErrorPrefix: "Last error: "
    }
  };

  function Editor(root, api, options) {
    var opts = typeof options === "function" ? { onState: options } : (options || {});
    this.root = root;
    this.api = api;
    this.onState = opts.onState || function () {};
    this.translateMsg = opts.translateMsg || function (m) { return m; };
    this.lang = opts.lang === "en" ? "en" : "zh";
    this.dirty = false;
    this.saving = false;
    this.loaded = false;
    this.data = null;
    this.noticeState = null;

    root.classList.add("wfc-forward-editor");
    root.innerHTML =
      "<form novalidate>" +
        '<div class="panel-header">' +
          '<div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">' +
            '<span class="panel-title" data-wfe="enableTitle"></span>' +
            '<span data-wfe="counts" class="panel-subtitle tabular"></span>' +
          "</div>" +
          '<div style="display:flex;align-items:center;gap:10px">' +
            '<span class="status-tag warn" data-wfe="dirtyBadge" hidden></span>' +
            '<label class="switch">' +
              '<input data-field="enabled" type="checkbox">' +
              '<span class="switch-track"></span>' +
            "</label>" +
          "</div>" +
        "</div>" +
        '<div class="panel-body">' +
          '<div class="alert error" data-wfe="lastErrorBox" hidden>' +
            '<div class="alert-body"><span data-wfe="lastErrorText"></span></div>' +
          "</div>" +
          '<div class="form-field">' +
            '<label class="form-label" for="wfe-curl-input" data-wfe="curlLabel"></label>' +
            '<textarea id="wfe-curl-input" class="wfe-code" data-field="curl" rows="9" spellcheck="false" autocomplete="off" autocapitalize="off" autocorrect="off"></textarea>' +
            '<p class="wfe-vars"><span data-wfe="varsPrefix"></span><code>{{sms-from}}</code> <code>{{sms-body}}</code> <code>{{sms-time}}</code> <code>{{sms-id}}</code></p>' +
          "</div>" +
          '<div class="form-actions">' +
            '<button class="btn btn-primary" data-wfe="saveBtn" type="submit" disabled></button>' +
          "</div>" +
          '<div class="alert" data-wfe="noticeBox" role="status" hidden style="margin-top:12px;margin-bottom:0">' +
            '<div class="alert-body"><span data-wfe="noticeText"></span></div>' +
          "</div>" +
        "</div>" +
      "</form>";

    var self = this;
    this.form = root.querySelector("form");
    this.form.addEventListener("input", function () {
      self.dirty = true;
      self.noticeState = null;
      self.renderText();
    });
    this.form.addEventListener("change", function () {
      self.dirty = true;
      self.noticeState = null;
      self.renderText();
    });
    this.form.addEventListener("submit", function (event) {
      event.preventDefault();
      self.save();
    });
    this.renderText();
  }

  Editor.prototype.t = function (key) {
    return (COPY[this.lang] || COPY.zh)[key] || key;
  };

  Editor.prototype.node = function (name) {
    return this.root.querySelector('[data-wfe="' + name + '"]');
  };

  Editor.prototype.field = function (name) {
    return this.root.querySelector('[data-field="' + name + '"]');
  };

  Editor.prototype.setLang = function (lang) {
    this.lang = lang === "en" ? "en" : "zh";
    this.renderText();
  };

  Editor.prototype.setNotice = function (kind, keyOrRaw, isKey) {
    this.noticeState = keyOrRaw ? { kind: kind, value: keyOrRaw, isKey: !!isKey } : null;
    this.renderText();
  };

  Editor.prototype.renderText = function () {
    this.node("enableTitle").textContent = this.t("enableTitle");
    this.node("curlLabel").textContent = this.t("curlLabel");
    this.field("curl").placeholder = this.t("curlPlaceholder");
    this.node("varsPrefix").textContent = this.t("varsPrefix");
    this.node("dirtyBadge").textContent = this.t("unsaved");
    this.node("dirtyBadge").hidden = !this.dirty;
    this.node("saveBtn").textContent = this.saving ? this.t("saving") : this.t("save");
    this.field("enabled").setAttribute("aria-label", this.t("enableTitle"));

    if (!this.data) {
      this.node("counts").textContent = this.t("loading");
      this.node("lastErrorBox").hidden = true;
    } else {
      this.node("counts").textContent = this.t("countsFmt")
        .replace("{ok}", String(this.data.delivered || 0))
        .replace("{pending}", String(this.data.pending || 0));
      if (this.data.last_error) {
        this.node("lastErrorBox").hidden = false;
        this.node("lastErrorText").textContent =
          this.t("lastErrorPrefix") + this.translateMsg(this.data.last_error, this.lang);
      } else {
        this.node("lastErrorBox").hidden = true;
      }
    }

    var noticeBox = this.node("noticeBox");
    if (!this.noticeState) {
      noticeBox.hidden = true;
    } else {
      noticeBox.hidden = false;
      noticeBox.className = "alert " + (this.noticeState.kind === "error" ? "error" : "ok");
      this.node("noticeText").textContent = this.noticeState.isKey
        ? this.t(this.noticeState.value)
        : this.translateMsg(this.noticeState.value, this.lang);
    }
  };

  Editor.prototype.display = function (data, fill) {
    this.data = data;
    if (fill) {
      this.field("enabled").checked = !!data.enabled;
      this.field("curl").value = data.curl || "";
      this.loaded = true;
      this.dirty = false;
      this.node("saveBtn").disabled = false;
    }
    this.renderText();
    this.onState(data);
  };

  Editor.prototype.load = function (fill) {
    var self = this;
    return this.api("forward").then(function (data) {
      self.display(data, fill);
    }).catch(function (error) {
      self.setNotice("error", error.message, false);
    });
  };

  Editor.prototype.save = function () {
    if (!this.loaded) {
      this.setNotice("error", "waitLoad", true);
      return;
    }
    var enabled = this.field("enabled").checked;
    var source = this.field("curl").value.trim();
    if (!source && (enabled || (this.data && this.data.curl))) {
      this.setNotice("error", "requireCurl", true);
      return;
    }
    var self = this;
    var button = this.node("saveBtn");
    this.saving = true;
    button.disabled = true;
    this.setNotice(null);
    this.api("forward", { enabled: enabled, curl: source }).then(function (data) {
      self.saving = false;
      self.display(data, true);
      self.setNotice("ok", "saved", true);
    }).catch(function (error) {
      self.saving = false;
      self.setNotice("error", error.message, false);
    }).then(function () {
      button.disabled = false;
      self.renderText();
    });
  };

  window.WFCForwardEditor = Editor;
}());
