// SPDX-License-Identifier: AGPL-3.0-or-later
// app_process bridge for the Android 4.4 system connectivity services.
import android.content.Context;
import android.net.wifi.ScanResult;
import android.net.wifi.WifiConfiguration;
import android.net.wifi.WifiInfo;
import android.net.wifi.WifiManager;
import android.os.Looper;
import android.telephony.TelephonyManager;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.lang.reflect.Method;
import java.util.List;

public final class Sp970Android {
    static Context context;
    static WifiManager wifi;
    static boolean flight() throws Exception {
        Process p = new ProcessBuilder("/system/bin/sh","/system/bin/settings","get","global","airplane_mode_on").start();
        String value = new BufferedReader(new InputStreamReader(p.getInputStream(),"UTF-8")).readLine();
        return p.waitFor() == 0 && "1".equals(value);
    }
    static String unquote(String value) {
        if (value == null || (value.equals("<unknown ssid>") || value.equals("0x"))) return "";
        return value.startsWith("\"") && value.endsWith("\"") ? value.substring(1, value.length()-1) : value;
    }
    static String quoted(String value) { return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\""; }
    static JSONObject status(boolean scan) throws Exception {
        JSONObject out = new JSONObject();
        out.put("flight_mode", flight());
        out.put("model", android.os.Build.MODEL);
        out.put("android", android.os.Build.VERSION.RELEASE);
        out.put("sdk", android.os.Build.VERSION.SDK_INT);
        out.put("firmware", android.os.Build.DISPLAY);
        out.put("uptime_seconds", android.os.SystemClock.elapsedRealtime()/1000);
        out.put("device_time", System.currentTimeMillis());
        out.put("wifi_enabled", wifi.isWifiEnabled());
        WifiInfo info = wifi.getConnectionInfo();
        out.put("ssid", info == null ? "" : unquote(info.getSSID()));
        out.put("network_id", info == null ? -1 : info.getNetworkId());
        out.put("rssi", info == null ? -127 : info.getRssi());
        out.put("connection", info == null ? "" : info.getSupplicantState().toString());
        if (info != null) { int ip = info.getIpAddress(); out.put("ipv4", (ip&255)+"."+((ip>>8)&255)+"."+((ip>>16)&255)+"."+((ip>>24)&255)); }
        TelephonyManager phone = (TelephonyManager)context.getSystemService(Context.TELEPHONY_SERVICE);
        String imsi = phone.getSubscriberId();
        out.put("sim_prefix", imsi != null && imsi.length() >= 5 ? imsi.substring(0,5) : "");
        out.put("sim_state", phone.getSimState());
        JSONArray saved = new JSONArray();
        List<WifiConfiguration> configs = wifi.getConfiguredNetworks();
        if (configs != null) for (WifiConfiguration c : configs) saved.put(new JSONObject().put("id",c.networkId).put("ssid",unquote(c.SSID)));
        out.put("saved", saved);
        if (scan) {
            boolean started = wifi.startScan();
            if (started) Thread.sleep(2500);
            JSONArray networks = new JSONArray();
            List<ScanResult> results = wifi.getScanResults();
            if (results != null) for (ScanResult r : results) {
                if (r.SSID == null || r.SSID.isEmpty()) continue;
                networks.put(new JSONObject().put("ssid",r.SSID).put("rssi",r.level).put("frequency",r.frequency).put("security",r.capabilities));
            }
            out.put("networks", networks); out.put("scan_started", started);
        }
        return out;
    }
    public static void main(String[] args) {
        try {
            Looper.prepareMainLooper();
            Class<?> activityThread = Class.forName("android.app.ActivityThread");
            Method systemMain = activityThread.getDeclaredMethod("systemMain");
            Object thread = systemMain.invoke(null);
            context = (Context)activityThread.getDeclaredMethod("getSystemContext").invoke(thread);
            wifi = (WifiManager)context.getSystemService(Context.WIFI_SERVICE);
            BufferedReader reader = new BufferedReader(new InputStreamReader(System.in,"UTF-8"));
            String line = reader.readLine();
            if (line == null || line.length() > 8192) throw new Exception("请求无效");
            JSONObject req = new JSONObject(line);
            String action = req.getString("action");
            if (action.equals("status") || action.equals("scan")) {
                System.out.println(status(action.equals("scan"))); System.exit(0); return;
            }
            if (action.equals("flight")) {
                boolean enabled = req.getBoolean("enabled");
                if (flight() != enabled) {
                    boolean keepWifi = wifi.isWifiEnabled();
                    Process setting = new ProcessBuilder("/system/bin/sh","/system/bin/settings","put","global","airplane_mode_on",enabled ? "1" : "0").start();
                    if (setting.waitFor() != 0) throw new Exception("无法保存飞行模式");
                    Process broadcast = new ProcessBuilder("/system/bin/sh","/system/bin/am","broadcast","-a","android.intent.action.AIRPLANE_MODE","--ez","state",String.valueOf(enabled)).start();
                    if (broadcast.waitFor() != 0) throw new Exception("飞行模式广播失败");
                    if (keepWifi) wifi.setWifiEnabled(true);
                }
                System.out.println(new JSONObject().put("ok",true).put("flight_mode",flight())); System.exit(0); return;
            }
            if (!flight()) throw new Exception("飞行模式必须保持开启");
            if (!action.equals("connect")) throw new Exception("不支持的操作");
            if (!wifi.isWifiEnabled()) {
                // Client mode and the stock hotspot share one radio. This is
                // only reached by an explicit connect request, never a read.
                Method apState = WifiManager.class.getMethod("getWifiApState");
                int hotspot = (Integer)apState.invoke(wifi);
                if (hotspot == 12 || hotspot == 13) {
                    Method disableAP = WifiManager.class.getMethod("setWifiApEnabled",WifiConfiguration.class,boolean.class);
                    if (!Boolean.TRUE.equals(disableAP.invoke(wifi,null,false))) throw new Exception("无法关闭设备热点");
                    Thread.sleep(1500);
                }
                if (!wifi.setWifiEnabled(true)) throw new Exception("无法开启 Wi-Fi");
                Thread.sleep(2000);
            }
            int id = req.optInt("network_id",-1);
            if (id >= 0) {
                boolean found = false;
                for (WifiConfiguration c : wifi.getConfiguredNetworks()) if (c.networkId == id) found = true;
                if (!found) throw new Exception("已保存的网络不存在");
            } else {
                String ssid = req.getString("ssid"), password = req.optString("password",""), security = req.optString("security","wpa2");
                if (ssid.getBytes("UTF-8").length < 1 || ssid.getBytes("UTF-8").length > 32 || ssid.indexOf('\0') >= 0) throw new Exception("Wi-Fi 名称无效");
                WifiConfiguration c = new WifiConfiguration(); c.SSID = quoted(ssid);
                if (security.equals("open")) {
                    if (!password.isEmpty()) throw new Exception("开放网络不需要密码");
                    c.allowedKeyManagement.set(WifiConfiguration.KeyMgmt.NONE);
                } else if (security.equals("wpa2")) {
                    if (password.getBytes("UTF-8").length < 8 || password.getBytes("UTF-8").length > 63) throw new Exception("Wi-Fi 密码需为 8–63 字节");
                    c.allowedKeyManagement.set(WifiConfiguration.KeyMgmt.WPA_PSK); c.preSharedKey = quoted(password);
                } else throw new Exception("此 Android 版本仅支持开放或 WPA/WPA2 个人网络");
                c.status = WifiConfiguration.Status.ENABLED;
                List<WifiConfiguration> existing = wifi.getConfiguredNetworks();
                if (existing != null) for (WifiConfiguration old : existing) if (c.SSID.equals(old.SSID)) c.networkId = old.networkId;
                id = c.networkId >= 0 ? wifi.updateNetwork(c) : wifi.addNetwork(c);
                if (id < 0 || !wifi.saveConfiguration()) throw new Exception("无法保存 Wi-Fi 网络");
            }
            if (!flight()) throw new Exception("飞行模式必须保持开启");
            if (!wifi.enableNetwork(id,true) || !wifi.reconnect()) throw new Exception("无法开始连接");
            System.out.println(new JSONObject().put("ok",true).put("network_id",id).put("message","已开始连接，网络切换时页面可能短暂断开"));
            System.exit(0);
        } catch (Throwable e) {
            try { System.out.println(new JSONObject().put("error", e.getMessage() == null ? "Android Wi-Fi 服务不可用" : e.getMessage())); } catch(Exception ignored) {}
            System.exit(1);
        }
    }
}
