/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function () {
    "use strict";

    function Editor(root, api, onState) {
        this.root = root;
        this.api = api;
        this.onState = onState || function () {};
        this.dirty = false;
        this.loaded = false;
        if (!document.getElementById("wfe-styles")) {
            var style = document.createElement("style");
            style.id = "wfe-styles";
            style.textContent =
                '.wfc-forward-editor{max-width:900px;color:#263943;line-height:1.6}' +
                '.wfc-forward-editor *{box-sizing:border-box}' +
                '.wfe-toggle{display:flex;align-items:center;gap:8px;font-size:15px;font-weight:600;margin:4px 0 18px}' +
                '.wfe-toggle input{margin:0}' +
                '.wfe-label{display:block;font-size:14px;font-weight:600;margin:12px 0 6px}' +
                '.wfc-forward-editor .wfe-code{display:block;width:100%;border:1px solid #c5d1d7;border-radius:7px;background:#f7f9fc;color:#263943;font:13px/1.8 ui-monospace,SFMono-Regular,Consolas,monospace;min-height:235px;padding:15px;margin-top:6px;tab-size:2;resize:vertical;overflow-wrap:anywhere}' +
                '.wfe-actions{display:flex;align-items:center;gap:14px;flex-wrap:wrap;margin:16px 0 8px}' +
                '.wfe-save{border:1px solid #426eaa;border-radius:6px;background:#426eaa;color:#fff;cursor:pointer;padding:9px 22px;font:inherit;font-size:14px;line-height:1.5}' +
                '.wfe-save:disabled{opacity:.5;cursor:wait}' +
                '.wfe-counts{font-size:12px;color:#687980}' +
                '.wfe-notice{font-size:13px;color:#176b40;margin:10px 0 0}' +
                '.wfe-notice:empty{display:none}.wfe-notice.wfe-error{color:#a33}';
            document.head.appendChild(style);
        }
        root.classList.add("wfc-forward-editor");
        root.innerHTML =
            '<form>' +
            '<label class="wfe-toggle"><input data-field="enabled" type="checkbox"> 启用短信转发</label>' +
            '<label class="wfe-label">cURL 命令<textarea class="wfe-code" data-field="curl" rows="9" spellcheck="false" autocomplete="off" autocapitalize="off" autocorrect="off" aria-label="cURL 命令"></textarea></label>' +
            '<div class="wfe-actions"><button class="wfe-save" type="submit" disabled>保存</button><span class="wfe-counts">正在读取…</span></div>' +
            '<p class="wfe-notice" role="status"></p>' +
            '</form>';
        var self = this;
        this.form = root.querySelector("form");
        this.form.addEventListener("input", function () { self.dirty = true; });
        this.form.addEventListener("change", function () { self.dirty = true; });
        this.form.addEventListener("submit", function (event) { event.preventDefault(); self.save(); });
    }

    Editor.prototype.field = function (name) { return this.root.querySelector('[data-field="' + name + '"]'); };
    Editor.prototype.notice = function (text, bad) {
        var node = this.root.querySelector(".wfe-notice");
        node.textContent = text || "";
        node.className = "wfe-notice" + (bad ? " wfe-error" : "");
    };
    Editor.prototype.display = function (data, fill) {
        if (fill) {
            this.field("enabled").checked = !!data.enabled;
            this.field("curl").value = data.curl || "";
            this.loaded = true;
            this.dirty = false;
            this.root.querySelector(".wfe-save").disabled = false;
        }
        this.root.querySelector(".wfe-counts").textContent = "已转发 " + data.delivered + " 条 · 待重试 " + data.pending + " 条";
        if (data.last_error) this.notice(data.last_error, true);
        this.onState(data);
    };
    Editor.prototype.load = function (fill) {
        var self = this;
        return this.api("forward").then(function (data) { self.display(data, fill); }).catch(function (error) { self.notice(error.message, true); });
    };
    Editor.prototype.save = function () {
        if (!this.loaded) { this.notice("请等待读取设置，或重新登录后台。", true); return; }
        var source = this.field("curl").value.trim();
        if (!source) { this.notice("请先粘贴 cURL 命令。", true); return; }
        var self = this, button = this.root.querySelector(".wfe-save");
        button.disabled = true;
        this.api("forward", { enabled: this.field("enabled").checked, curl: source }).then(function (data) {
            self.display(data, true);
            self.notice("已保存。");
        }).catch(function (error) { self.notice(error.message, true); }).then(function () { button.disabled = false; });
    };
    window.WFCForwardEditor = Editor;
}());
