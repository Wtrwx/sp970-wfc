// SPDX-License-Identifier: AGPL-3.0-or-later
(function () {
    "use strict";
    var token = sessionStorage.getItem("wfc_session"), busyState = false, state = null, currentView = "overview", smsBox = "inbox", editor = null, proxyDirty = false;
    var views = {overview:["设备概览","设备连接与服务状态"],network:["网络与 WFC","配置 Wi-Fi Calling、代理出口和 Wi-Fi 连接"],sms:["短信","通过 Wi-Fi Calling 收发短信"],forward:["短信转发","用 cURL 配置转发请求"],euicc:["eSIM 管理","管理小白卡中已安装的配置"],device:["设备信息","查看设备与系统状态"]};
    function el(id) { return document.getElementById(id); }
    function text(id,value) { var x=el(id); if(x)x.textContent=value==null?"—":value; }
    function notice(message,bad) { var x=el("notice")||el("login-error"); if(x){x.textContent=message||"";x.className="notice"+(bad?" error":"");} }
    function loginTarget() { return "/login.html?next="+encodeURIComponent(location.hash.replace("#","")||"overview"); }
    function request(path,body,ims) {
        var controller=new AbortController(),timer=setTimeout(function(){controller.abort();},90000);
        return fetch((ims?"/wfc/":"/wfc-admin/")+path,{method:body===undefined?"GET":"POST",credentials:"same-origin",cache:"no-store",headers:{"Content-Type":"application/json","Authorization":"Bearer "+(token||"")},body:body===undefined?undefined:JSON.stringify(body),signal:controller.signal}).then(function(r){return r.json().catch(function(){return {error:"设备服务暂不可用，请稍后刷新"};}).then(function(data){
            if(r.status===401&&!ims&&path!=="login"){sessionStorage.removeItem("wfc_session");location.replace(loginTarget());throw new Error("请重新登录");}
            if(!r.ok)throw new Error(data.error||"请求失败");return data;
        });}).finally(function(){clearTimeout(timer);});
    }
    function busy(operation) {
        if(busyState)return Promise.reject(new Error("请等待当前操作完成"));
        busyState=true;document.querySelectorAll("button,input,select,textarea").forEach(function(x){x.dataset.wasDisabled=x.disabled?"1":"0";x.disabled=true;});
        return operation().finally(function(){busyState=false;document.querySelectorAll("button,input,select,textarea").forEach(function(x){x.disabled=x.dataset.wasDisabled==="1";delete x.dataset.wasDisabled;});});
    }
    if(document.body.dataset.page==="login") {
        el("show-password").addEventListener("click",function(){var shown=el("password").type==="password";el("password").type=shown?"text":"password";this.textContent=shown?"隐藏":"显示";this.setAttribute("aria-label",shown?"隐藏密码":"显示密码");});
        el("login-form").addEventListener("submit",function(e){e.preventDefault();notice("");busy(function(){return request("login",{Account:el("account").value,Password:el("password").value});}).then(function(data){sessionStorage.setItem("wfc_session",data.token);el("password").value="";var next=new URLSearchParams(location.search).get("next");location.replace("/index.html#"+(Object.hasOwn(views,next)?next:"overview"));}).catch(function(e){notice(e.message,true);});});return;
    }
    if(!token){location.replace(loginTarget());return;}
    function renderStatus(s,fill) {
        var connected=s.connection==="COMPLETED"&&s.ipv4!=="0.0.0.0";
        state=s;el("app-loading").hidden=true;el("app").hidden=false;
        text("sidebar-device",s.model||"SP970-B");text("metric-wifi",s.ssid||"未连接");text("metric-ip",s.ipv4||"等待网络");text("metric-flight",s.flight_mode?"已开启":"已关闭");text("metric-sim",s.sim_prefix||"未就绪");text("metric-sim-sub",s.sim_prefix==="23487"?"Lebara UK":"检查 SIM 策略");
        text("metric-wfc",s.wfc_enabled?(s.flight_mode?(connected?"正在注册":"等待 Wi-Fi"):"已暂停"):"已关闭");text("metric-wfc-sub",s.wfc_enabled?"开机自动恢复":"服务未启用");
        document.querySelectorAll('[data-switch="wfc"]').forEach(function(x){x.checked=s.wfc_enabled;});document.querySelectorAll('[data-switch="flight"]').forEach(function(x){x.checked=s.flight_mode;});
        var proxy=s.proxy.mode==="direct"?"直连":s.proxy.configured?s.proxy.server:"尚未配置";
        text("overview-ssid",s.ssid||"未连接");text("overview-proxy",proxy);text("overview-sim",s.sim_prefix==="23487"?"Lebara UK · 23487":s.sim_prefix||"未就绪");text("overview-registration",s.wfc_enabled?(connected?"等待注册":"等待 Wi-Fi 连接"):"服务已关闭");
        text("overview-proxy-badge",s.proxy.mode==="direct"?"直连":"Hysteria2");text("proxy-badge",s.proxy.mode==="direct"?"直连":"Hysteria2");text("proxy-server",s.proxy.mode==="direct"?"使用上游网络":proxy);
        text("network-wfc-state",s.wfc_enabled?(s.flight_mode?(connected?"已开启，正在注册或重连":"等待 Wi-Fi 连接"):"等待开启飞行模式"):"已关闭");text("network-flight-state",s.flight_mode?"已开启，蜂窝射频关闭":"已关闭，WFC 暂停");
        text("wifi-connection",s.ssid&&s.connection==="COMPLETED"?"已连接 "+s.ssid+" · "+s.ipv4:"尚未连接 Wi-Fi");
        text("header-state",s.ssid?"设备在线":"等待 Wi-Fi");el("header-state").className="pill"+(s.ssid?" good":"");text("footer-state",connected?"已连接 "+s.ssid:"等待 Wi-Fi 连接");
        if(fill&&!proxyDirty){el("network-mode").value=s.proxy.mode;el("epdg").value=s.proxy.epdg;nodeField();}
        el("wifi-saved").replaceChildren();(s.saved||[]).forEach(function(n){var connected=n.id===s.network_id;el("wifi-saved").appendChild(item(n.ssid,connected?"当前连接":"已保存","连接",connected?null:function(){connectWifi({network_id:n.id});}));});
        text("device-model",s.model||"SP970-B");text("device-android",s.android?"Android "+s.android+" · SDK "+s.sdk:"—");text("device-firmware",s.firmware||"—");text("device-uptime",s.uptime_seconds==null?"—":Math.floor(s.uptime_seconds/3600)+" 小时 "+Math.floor(s.uptime_seconds%3600/60)+" 分钟");text("device-time",s.device_time?new Date(s.device_time).toLocaleString():"—");text("device-ip",s.ipv4);text("device-autostart",s.wfc_enabled?"已开启":"已关闭");
        el("metric-wfc").className="";el("metric-flight").className=s.flight_mode?"good":"";
        if(s.wfc_enabled)request("status",undefined,true).then(function(ims){if(!state||!state.wfc_enabled)return;text("metric-wfc",ims.registered?"已注册":"正在注册");el("metric-wfc").className=ims.registered?"good":"";text("metric-wfc-sub",ims.registered?"Wi-Fi Calling 在线":"自动注册与重连");text("overview-registration",ims.registered?"Wi-Fi Calling 已注册":"正在注册");text("network-wfc-state",ims.registered?"Wi-Fi Calling 已注册 · 自动续期":"正在注册…");}).catch(function(){});
    }
    function status(fill){return request("status").then(function(s){renderStatus(s,fill);});}
    function item(title,detail,label,action,badge) {
        var row=document.createElement("div"),copy=document.createElement("div"),name=document.createElement("div"),sub=document.createElement("p"),right=document.createElement("div");row.className="network-item";name.className="item-title";name.textContent=title;sub.className="item-detail";sub.textContent=detail;copy.append(name,sub);row.append(copy,right);
        if(badge){var b=document.createElement("span");b.className="pill good";b.textContent=badge;right.appendChild(b);}
        if(action){var button=document.createElement("button");button.type="button";button.className="button secondary small";button.textContent=label;button.addEventListener("click",action);right.appendChild(button);}return row;
    }
    function route() {
        var key=location.hash.slice(1)||"overview";if(!Object.hasOwn(views,key))key="overview";currentView=key;
        document.querySelectorAll("[data-view]").forEach(function(x){x.hidden=x.dataset.view!==key;});document.querySelectorAll("nav [data-page]").forEach(function(x){x.classList.toggle("active",x.dataset.page===key);if(x.dataset.page===key)x.setAttribute("aria-current","page");else x.removeAttribute("aria-current");});
        text("page-title",views[key][0]);text("breadcrumb",views[key][0]);text("page-description",views[key][1]);notice("");
        if(key==="sms")loadSMS().catch(function(e){notice(e.message,true);});if(key==="euicc")loadProfiles().catch(function(e){notice(e.message,true);});if(key==="forward")editor.load(!editor.loaded);
    }
    function nodeField(){el("node-field").hidden=el("network-mode").value!=="hysteria2";}
    function connectWifi(body){busy(function(){return request("wifi/connect",body);}).then(function(result){el("wifi-password").value="";notice(result.message||"已开始连接");setTimeout(function(){status(false).catch(function(){});},5000);}).catch(function(e){notice(e.message,true);});}
    function loadProfiles(){text("euicc-summary","正在读取小白卡…");return request("euicc").then(function(data){el("euicc-profiles").replaceChildren();text("euicc-summary",data.profiles.length+" 个已安装配置");data.profiles.forEach(function(p){var row=item(p.name,p.provider+" · "+p.iccid,"切换",p.enabled?null:function(){if(!confirm("切换到“"+p.name+"”？WFC 将暂停，切换后检查 SIM 策略再决定是否重新注册。"))return;busy(function(){return request("euicc/enable",{id:p.id});}).then(function(result){notice(result.message||"已切换");return loadProfiles();}).then(function(){return status(false);}).catch(function(e){notice(e.message,true);});},p.enabled?"使用中":null);row.className="profile-item";el("euicc-profiles").appendChild(row);});if(!data.profiles.length){var empty=document.createElement("p");empty.className="empty";empty.textContent="未发现已安装配置";el("euicc-profiles").appendChild(empty);}}).catch(function(e){text("euicc-summary",e.message);throw e;});}
    function loadSMS(){var archived=smsBox==="archive",box=smsBox==="sent"?"sent":"inbox";return request("sms?box="+(archived?"":box)+"&archived="+(archived?"1":"0")).then(function(data){el("sms-list").replaceChildren();text("sms-summary",data.messages.length+" 条短信"+(data.unread?" · "+data.unread+" 条未读":""));el("sms-badge").hidden=!data.unread;text("sms-badge",data.unread);
        data.messages.forEach(function(m){var entry=document.createElement("article"),heading=document.createElement("div"),sender=document.createElement("strong"),date=document.createElement("time"),body=document.createElement("p"),actions=document.createElement("div");entry.className="sms-item"+(!m.read?" unread":"");heading.className="sms-heading";sender.textContent=m.address;date.textContent=new Date(m.date).toLocaleString();date.dateTime=new Date(m.date).toISOString();heading.append(sender,date);body.className="sms-body";body.textContent=m.body;actions.className="sms-actions";
            if(!m.read){var read=document.createElement("button");read.textContent="标为已读";read.addEventListener("click",function(){request("sms/read",{id:m.id}).then(loadSMS).catch(function(e){notice(e.message,true);});});actions.appendChild(read);}
            var archive=document.createElement("button");archive.textContent=m.archived?(m.type===2?"恢复到已发送":"恢复到收件箱"):"归档";archive.addEventListener("click",function(){request("sms/archive",{id:m.id,archived:!m.archived}).then(loadSMS).catch(function(e){notice(e.message,true);});});actions.appendChild(archive);entry.append(heading,body,actions);el("sms-list").appendChild(entry);
        });if(!data.messages.length){var empty=document.createElement("p");empty.className="empty";empty.textContent=archived?"暂无已归档短信":"暂无短信";el("sms-list").appendChild(empty);}});}
    editor=new WFCForwardEditor(el("forward-editor"),function(path,body){return request(path,body);});
    document.querySelectorAll("[data-switch]").forEach(function(input){input.addEventListener("change",function(){var enabled=this.checked,kind=this.dataset.switch;if(kind==="flight"&&!enabled&&!confirm("关闭飞行模式将开启蜂窝射频，WFC 会暂停。继续？")){this.checked=true;return;}busy(function(){return request(kind==="flight"?"flight":"wfc",{enabled:enabled});}).then(function(){notice((kind==="flight"?"飞行模式":"Wi-Fi Calling")+(enabled?"已开启":"已关闭"));}).catch(function(e){notice(e.message,true);}).finally(function(){status(false).catch(function(){});});});});
    el("logout").addEventListener("click",function(){request("logout",{}).then(function(){sessionStorage.removeItem("wfc_session");location.replace("/login.html");}).catch(function(e){notice(e.message,true);});});
    el("refresh").addEventListener("click",function(){status(!proxyDirty).then(function(){if(currentView==="sms")return loadSMS();if(currentView==="euicc")return loadProfiles();if(currentView==="forward")return editor.load(!editor.dirty);}).catch(function(e){notice(e.message,true);});});
    el("proxy-form").addEventListener("input",function(){proxyDirty=true;});el("network-mode").addEventListener("change",nodeField);
    el("proxy-form").addEventListener("submit",function(e){e.preventDefault();busy(function(){return request("network",{mode:el("network-mode").value,uri:el("proxy-uri").value.trim(),epdg:el("epdg").value});}).then(function(){proxyDirty=false;el("proxy-uri").value="";notice("网络配置已应用");return status(true);}).catch(function(e){notice(e.message,true);});});
    el("wifi-security").addEventListener("change",function(){el("wifi-password-label").hidden=this.value==="open";});
    el("wifi-form").addEventListener("submit",function(e){e.preventDefault();connectWifi({ssid:el("wifi-ssid").value,security:el("wifi-security").value,password:el("wifi-security").value==="open"?"":el("wifi-password").value});});
    el("wifi-scan").addEventListener("click",function(){busy(function(){return request("wifi/scan");}).then(function(data){var seen={};el("wifi-networks").replaceChildren();(data.networks||[]).sort(function(a,b){return b.rssi-a.rssi;}).forEach(function(n){if(seen[n.ssid])return;seen[n.ssid]=true;var supported=!/WEP|EAP/.test(n.security)&&(!/SAE/.test(n.security)||/PSK/.test(n.security)),secure=/WPA|PSK|SAE/.test(n.security);el("wifi-networks").appendChild(item(n.ssid,(n.frequency>4900?"5 GHz":"2.4 GHz")+" · "+(secure?"加密":"开放")+(supported?"":" · 不支持此认证"),"选择",supported?function(){el("wifi-connect").open=true;el("wifi-ssid").value=n.ssid;el("wifi-security").value=secure?"wpa2":"open";el("wifi-password-label").hidden=!secure;el("wifi-password").value="";(secure?el("wifi-password"):el("wifi-ssid")).focus();}:null));});if(!Object.keys(seen).length){var empty=document.createElement("p");empty.className="empty";empty.textContent="未发现 Wi-Fi 网络";el("wifi-networks").appendChild(empty);}}).catch(function(e){notice(e.message,true);});});
    el("euicc-refresh").addEventListener("click",function(){loadProfiles().catch(function(e){notice(e.message,true);});});
    document.querySelectorAll("[data-sms-box]").forEach(function(button){button.addEventListener("click",function(){smsBox=this.dataset.smsBox;document.querySelectorAll("[data-sms-box]").forEach(function(x){x.classList.toggle("active",x.dataset.smsBox===smsBox);});loadSMS().catch(function(e){notice(e.message,true);});});});
    el("compose-open").addEventListener("click",function(){el("compose").hidden=false;el("recipient").focus();});el("compose-close").addEventListener("click",function(){el("compose").hidden=true;});
    el("send-form").addEventListener("submit",function(e){e.preventDefault();busy(function(){return request("send",{To:el("recipient").value.replace(/[\s().-]/g,""),Text:el("message").value},true);}).then(function(){el("message").value="";notice("短信已提交运营商");return loadSMS();}).catch(function(e){notice(e.message,true);});});
    window.addEventListener("hashchange",route);status(true).then(route).catch(function(e){text("app-loading",e.message);notice(e.message,true);});
    setInterval(function(){if(!busyState&&!document.hidden){status(false).catch(function(){});if(currentView==="forward"&&editor.loaded&&!editor.dirty)editor.load(false);}},20000);
}());
