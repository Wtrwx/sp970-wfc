/* SPDX-License-Identifier: AGPL-3.0-or-later */
(function () {
    "use strict";
    var formTemplate = "from={{sms-from}}\ntime={{sms-time}}\nbody={{sms-body-escaped}}\nufi_escaped=1";
    var presets = {
        form: {method:"POST", format:"form", body:formTemplate, success:"accepted"},
        json: {method:"POST", format:"json", body:JSON.stringify({from:"{{sms-from}}", time:"{{sms-time}}", text:"{{sms-body}}", id:"{{sms-id}}"}, null, 2), success:"http"},
        text: {method:"POST", format:"text", body:"来自 {{sms-from}}\n时间 {{sms-time}}\n\n{{sms-body}}", success:"http"},
        get: {method:"GET", format:"none", body:"", success:"http"}
    };
    function option(value, label) { return '<option value="'+value+'">'+label+'</option>'; }
    function field(name, label, input, help) { return '<label class="wfe-label">'+label+input+'</label>'+(help?'<p class="wfe-help">'+help+'</p>':''); }
    function input(name, type, extra) { return '<input data-field="'+name+'" type="'+(type||'text')+'" '+(extra||'')+'>'; }
    function select(name, values) { return '<select data-field="'+name+'">'+values.map(function (v) { return option(v[0],v[1]); }).join('')+'</select>'; }
    function area(name, rows, extra) { return '<textarea data-field="'+name+'" rows="'+rows+'" spellcheck="false" '+(extra||'')+'></textarea>'; }
    function object(value, title) {
        var data;
        try { data=JSON.parse(value.trim()||"{}"); } catch (_) { throw new Error(title+"必须是 JSON 对象"); }
        if (!data || Array.isArray(data) || typeof data!=="object" || Object.keys(data).some(function(k){return typeof data[k]!=="string";})) throw new Error(title+"的名称和值都应为字符串");
        return data;
    }
    function Editor(root, api, onState) {
        this.root=root; this.api=api; this.onState=onState||function(){}; this.authMask=""; this.dirty=false; this.loaded=false;
        if (!document.getElementById("wfe-styles")) {
            var style=document.createElement("style"); style.id="wfe-styles";
            style.textContent='.wfc-forward-editor{max-width:900px;color:#263943;line-height:1.6}.wfc-forward-editor *{box-sizing:border-box}.wfe-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 18px}.wfe-label{display:block;font-size:14px;font-weight:600;margin:14px 0 5px}.wfc-forward-editor input:not([type=checkbox]),.wfc-forward-editor select,.wfc-forward-editor textarea{display:block;width:100%;border:1px solid #bdcdd0;border-radius:5px;background:#fff;color:#263943;padding:9px 11px;font-size:14px;font-family:inherit;line-height:1.5;margin-top:5px}.wfc-forward-editor textarea,.wfe-preview{font-family:ui-monospace,SFMono-Regular,monospace!important}.wfe-help{font-size:12px;color:#667780;margin:5px 0}.wfe-toggle{font-weight:500;margin:12px 0;display:block}.wfe-toggle input{margin-right:6px}.wfe-variables{display:flex;flex-wrap:wrap;gap:5px;margin:8px 0}.wfc-forward-editor button{border:1px solid #b9cdcf;border-radius:5px;color:#225b61;background:#f0f6f6;cursor:pointer;padding:8px 12px;font:inherit;margin:0}.wfc-forward-editor button:disabled{opacity:.5;cursor:wait}.wfe-variables button{font-size:12px;padding:4px 7px}.wfe-actions{display:flex;gap:8px;flex-wrap:wrap;margin:18px 0 10px}.wfc-forward-editor button.wfe-primary{background:#426eaa;color:#fff;border-color:#426eaa}.wfc-forward-editor details{border-top:1px solid #e1e8e9;margin-top:18px;padding-top:15px}.wfc-forward-editor summary{font-weight:600;cursor:pointer}.wfe-notice{font-size:14px;min-height:22px;margin:10px 0;color:#176b40}.wfe-notice.wfe-error{color:#a33}.wfe-counts{font-size:14px;padding:10px 13px;background:#f3f7f8;border-radius:5px}.wfe-preview{white-space:pre-wrap;overflow-wrap:anywhere;background:#f4f7fa;border:1px solid #dbe5e8;padding:14px;border-radius:6px;font-size:12px;max-height:420px;overflow:auto}.wfc-forward-editor [hidden]{display:none!important}@media(max-width:650px){.wfe-grid{grid-template-columns:1fr}}';
            document.head.appendChild(style);
        }
        root.classList.add("wfc-forward-editor");
        root.innerHTML='<form data-form="editor">'+
            '<label class="wfe-toggle"><input data-field="enabled" type="checkbox"> 转发启用后新收到的短信</label>'+
            '<div class="wfe-grid">'+field('preset','请求预设',select('preset',[["custom","自定义"],["form","兼容表单"],["json","通用 JSON"],["text","纯文本"],["get","URL / GET"]]))+
            field('method','请求方式',select('method',[["POST","POST"],["GET","GET"],["PUT","PUT"],["PATCH","PATCH"],["DELETE","DELETE"]]))+'</div>'+
            field('endpoint','接收地址',input('endpoint','text','placeholder="https://example.com/webhook" autocomplete="off"'),'地址可使用变量；查询参数自动进行 URL 编码。')+
            field('authorization','Authorization',input('authorization','text','autocomplete="off" placeholder="可选，如 Bearer …"'),'已保存的鉴权用星号显示，留空保留。也可引用 {{secret:token}}。')+
            '<label class="wfe-toggle wfe-help"><input data-field="clear_authorization" type="checkbox"> 保存时清除已有 Authorization</label>'+
            field('format','请求体格式',select('format',[["form","表单 · 每行 字段=值"],["json","JSON · 自动转义变量"],["text","纯文本"],["none","无请求体"]]))+
            field('body','请求体模板',area('body',6),'JSON 中把变量放在字符串中；表单值无需手动 URL 编码。')+
            '<div class="wfe-variables">'+[['sms-from','发件号码'],['sms-time','时间'],['sms-body','正文'],['sms-id','消息 ID'],['sms-body-escaped','转义正文']].map(function(v){return '<button type="button" data-variable="'+v[0]+'">'+v[1]+' {{'+v[0]+'}}</button>';}).join('')+'</div>'+
            '<details><summary>高级设置：请求头、密钥变量、成功判定</summary>'+
            field('headers','自定义请求头（JSON）',area('headers',3,'placeholder=\'{"Content-Type":"application/json"}\''),'Content-Type 可在这里覆盖。鉴权、Token、Cookie 等敏感头自动遮盖。')+
            field('secret_headers','其他敏感请求头',input('secret_headers','text','placeholder="X-Credential, X-Custom-Key"'),'用逗号分隔，保存后对应值显示为星号。')+
            field('secrets','密钥变量（JSON）',area('secrets',3,'placeholder=\'{"token":"自己的密钥"}\''),'在地址、请求头或模板中使用 {{secret:token}}。已保存的星号保留原值；删除键即移除变量。')+
            '<div class="wfe-grid">'+field('success','成功判定',select('success',[["accepted","HTTP 2xx 且 accepted = true"],["http","任意 HTTP 2xx"],["json","HTTP 2xx 且 JSON 字段匹配"]]))+
            field('timeout_seconds','请求超时（秒）',input('timeout_seconds','number','min="3" max="60" value="15"'))+'</div>'+
            '<div data-group="success-json" class="wfe-grid" hidden>'+field('success_path','JSON 字段路径',input('success_path','text','placeholder="data.code"'))+
            field('success_value','期望值（JSON）',input('success_value','text','placeholder=\'0、true 或 "ok"\''))+'</div>'+
            field('check_endpoint','只读检查地址（可选）',input('check_endpoint','text','placeholder="https://example.com/health"'),'应是同站点、不会触发通知的 GET 地址。检查使用已保存的配置。')+'</details>'+
            '<div class="wfe-actions"><button class="wfe-primary" data-action="save" type="submit">保存设置</button><button data-action="preview" type="button">预览请求</button><button data-action="check" type="button">检查已保存配置</button></div>'+
            '<p class="wfe-notice" role="status"></p><pre class="wfe-preview" hidden></pre></form>'+
            '<p class="wfe-counts">正在读取转发状态…</p><p class="wfe-help">新短信先保存在本机，再进入转发队列；失败自动重试，历史收件箱不会补发。预览只渲染示例内容，不发送通知。</p>';
        var self=this;
        this.form=root.querySelector('form');
        this.form.addEventListener('input',function(){self.dirty=true;});
        this.form.addEventListener('change',function(){self.dirty=true;self.layout();});
        this.field('preset').addEventListener('change',function(){var p=presets[this.value];if(p){Object.keys(p).forEach(function(k){self.field(k).value=p[k];});self.layout();self.notice('已填入请求结构，保存后生效。');}});
        this.field('authorization').addEventListener('focus',function(){if(this.value===self.authMask)this.value='';this.type='password';});
        this.field('authorization').addEventListener('blur',function(){if(this.value===''){this.type='text';this.value=self.authMask;}});
        this.form.addEventListener('submit',function(event){event.preventDefault();self.action('save');});
        ['preview','check'].forEach(function(name){root.querySelector('[data-action="'+name+'"]').addEventListener('click',function(){self.action(name);});});
        root.querySelectorAll('[data-variable]').forEach(function(button){button.addEventListener('click',function(){var body=self.field('body'),value='{{'+button.dataset.variable+'}}',start=body.selectionStart,end=body.selectionEnd;body.value=body.value.slice(0,start)+value+body.value.slice(end);body.focus();body.selectionStart=body.selectionEnd=start+value.length;self.dirty=true;});});
    }
    Editor.prototype.field=function(name){return this.root.querySelector('[data-field="'+name+'"]');};
    Editor.prototype.notice=function(text,bad){var e=this.root.querySelector('.wfe-notice');e.textContent=text||'';e.className='wfe-notice'+(bad?' wfe-error':'');};
    Editor.prototype.layout=function(){this.field('body').disabled=this.field('format').value==='none';this.root.querySelector('[data-group="success-json"]').hidden=this.field('success').value!=='json';};
    Editor.prototype.display=function(data,fill){
        if(fill){var self=this;['endpoint','method','format','body','success','success_path','success_value','check_endpoint','timeout_seconds'].forEach(function(k){self.field(k).value=data[k]===undefined?'':data[k];});
            this.field('enabled').checked=!!data.enabled;this.field('preset').value='custom';this.field('method').value=data.method||'POST';this.field('format').value=data.format||'form';this.field('body').value=data.body===undefined?formTemplate:data.body;this.field('success').value=data.success||'accepted';this.field('timeout_seconds').value=data.timeout_seconds||15;
            this.authMask=data.authorization_masked||(data.authorization_configured?'****************':'');this.field('authorization').type='text';this.field('authorization').value=this.authMask;this.field('clear_authorization').checked=false;
            this.field('headers').value=JSON.stringify(data.headers||{},null,2);this.field('secrets').value=JSON.stringify(data.secrets||{},null,2);this.field('secret_headers').value=(data.secret_headers||[]).join(', ');this.dirty=false;this.loaded=true;this.layout();}
        this.root.querySelector('.wfe-counts').textContent=(data.enabled?'已开启':'已关闭')+' · 已转发 '+data.delivered+' 条 · 等待重试 '+data.pending+' 条';
        if(data.last_error)this.notice(data.last_error,true);this.onState(data);
    };
    Editor.prototype.config=function(){
        if(!this.loaded)throw new Error('请等待读取当前设置，或重新登录后台。');
        var self=this,c={};['endpoint','method','format','body','success','success_path','success_value','check_endpoint'].forEach(function(k){c[k]=self.field(k).value;});
        c.enabled=this.field('enabled').checked;c.authorization=this.field('authorization').value===this.authMask?'':this.field('authorization').value;c.clear_authorization=this.field('clear_authorization').checked;c.timeout_seconds=Number(this.field('timeout_seconds').value);
        c.headers=object(this.field('headers').value,'请求头');c.secrets=object(this.field('secrets').value,'密钥变量');c.secret_headers=this.field('secret_headers').value.split(',').map(function(v){return v.trim();}).filter(Boolean);return c;
    };
    Editor.prototype.load=function(fill){var self=this;return this.api('forward').then(function(data){self.display(data,fill);}).catch(function(error){self.notice(error.message,true);});};
    Editor.prototype.action=function(action){
        var self=this,button=this.root.querySelector('[data-action="'+action+'"]'),body;
        try{body=action==='check'?{}:this.config();}catch(error){this.notice(error.message,true);return;}
        button.disabled=true;var path=action==='save'?'forward':'forward/'+action;
        this.api(path,body).then(function(data){
            if(action==='save'){self.display(data,true);self.notice('设置已保存。');}
            else if(action==='preview'){var pre=self.root.querySelector('.wfe-preview');pre.textContent=data.method+' '+data.endpoint+'\n'+Object.keys(data.headers).map(function(k){return k+': '+data.headers[k];}).join('\n')+'\n\n'+data.body;pre.hidden=false;self.notice('示例预览已生成，未向接收服务发送请求；密钥已遮盖。');}
            else self.notice('连接检查通过，未发送短信。');
        }).catch(function(error){self.notice(error.message,true);}).then(function(){button.disabled=false;});
    };
    window.WFCForwardEditor=Editor;
}());
