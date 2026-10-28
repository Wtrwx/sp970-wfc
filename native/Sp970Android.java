// SPDX-License-Identifier: AGPL-3.0-or-later
// app_process bridge for the Android 4.4 system connectivity services.
import android.content.Context;
import android.net.ConnectivityManager;
import android.net.wifi.ScanResult;
import android.net.wifi.WifiConfiguration;
import android.net.wifi.WifiInfo;
import android.net.wifi.WifiManager;
import android.os.Looper;
import android.provider.Settings;
import android.telephony.TelephonyManager;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.lang.reflect.Method;
import java.net.Inet4Address;
import java.net.InetAddress;
import java.net.NetworkInterface;
import java.util.ArrayList;
import java.util.Enumeration;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

public final class Sp970Android {
    static Context context;
    static WifiManager wifi;
    static ConnectivityManager cm;

    static int globalInt(String key, int fallback) {
        try {
            return Settings.Global.getInt(context.getContentResolver(), key);
        } catch (Throwable ignored) {}
        try {
            Process p = new ProcessBuilder("/system/bin/sh", "/system/bin/settings", "get", "global", key).start();
            String value = new BufferedReader(new InputStreamReader(p.getInputStream(), "UTF-8")).readLine();
            if (p.waitFor() == 0 && value != null) {
                String trimmed = value.trim();
                if ("1".equals(trimmed)) return 1;
                if ("0".equals(trimmed)) return 0;
            }
        } catch (Throwable ignored) {}
        return fallback;
    }

    static boolean putGlobalInt(String key, int val) throws Exception {
        Process p = new ProcessBuilder("/system/bin/sh", "/system/bin/settings", "put", "global", key, String.valueOf(val)).start();
        return p.waitFor() == 0;
    }

    static boolean flight() {
        return globalInt("airplane_mode_on", 0) == 1;
    }

    static String unquote(String value) {
        if (value == null || value.equals("<unknown ssid>") || value.equals("0x")) return "";
        return value.startsWith("\"") && value.endsWith("\"") && value.length() >= 2
            ? value.substring(1, value.length() - 1)
            : value;
    }

    static String quoted(String value) {
        return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\"";
    }

    static String ifaceIpv4(String ifaceName) {
        if (ifaceName == null || ifaceName.isEmpty()) return "";
        try {
            NetworkInterface ni = NetworkInterface.getByName(ifaceName);
            if (ni == null) return "";
            Enumeration<InetAddress> addrs = ni.getInetAddresses();
            while (addrs.hasMoreElements()) {
                InetAddress addr = addrs.nextElement();
                if (addr instanceof Inet4Address && !addr.isLoopbackAddress()) {
                    return addr.getHostAddress();
                }
            }
        } catch (Throwable ignored) {}
        return "";
    }

    static int countArpClients(String dev) {
        int count = 0;
        BufferedReader br = null;
        try {
            br = new BufferedReader(new InputStreamReader(new FileInputStream("/proc/net/arp"), "UTF-8"));
            String line = br.readLine(); // skip header
            while ((line = br.readLine()) != null) {
                String[] parts = line.trim().split("\\s+");
                if (parts.length >= 6 && dev.equals(parts[5]) && !"0x0".equals(parts[2]) && !"00:00:00:00:00:00".equals(parts[3])) {
                    count++;
                }
            }
        } catch (Throwable ignored) {
        } finally {
            if (br != null) try { br.close(); } catch (Throwable ignored) {}
        }
        return count;
    }

    static int wpa2KeyMgmt() {
        try {
            return WifiConfiguration.KeyMgmt.class.getField("WPA2_PSK").getInt(null);
        } catch (Throwable ignored) {
            return 4;
        }
    }

    static boolean isOpenWifi(WifiConfiguration c) {
        if (c == null) return true;
        if (c.allowedKeyManagement.get(WifiConfiguration.KeyMgmt.WPA_PSK)
                || c.allowedKeyManagement.get(WifiConfiguration.KeyMgmt.WPA_EAP)
                || c.allowedKeyManagement.get(WifiConfiguration.KeyMgmt.IEEE8021X)
                || c.allowedKeyManagement.get(wpa2KeyMgmt())) {
            return false;
        }
        return c.wepKeys == null || c.wepKeys[0] == null || c.wepKeys[0].isEmpty();
    }

    static boolean isApWpa2(WifiConfiguration c) {
        if (c == null) return false;
        return c.allowedKeyManagement.get(wpa2KeyMgmt())
            || c.allowedKeyManagement.get(WifiConfiguration.KeyMgmt.WPA_PSK);
    }

    static int getApState() {
        try {
            return (Integer) WifiManager.class.getMethod("getWifiApState").invoke(wifi);
        } catch (Throwable ignored) {
            return 11;
        }
    }

    static WifiConfiguration getApConfig() {
        try {
            return (WifiConfiguration) WifiManager.class.getMethod("getWifiApConfiguration").invoke(wifi);
        } catch (Throwable ignored) {
            return null;
        }
    }

    static boolean setApEnabled(WifiConfiguration cfg, boolean enabled) throws Exception {
        Method m = WifiManager.class.getMethod("setWifiApEnabled", WifiConfiguration.class, boolean.class);
        return Boolean.TRUE.equals(m.invoke(wifi, cfg, enabled));
    }

    static boolean matchesUsb(String iface, String[] regexs) {
        if (iface == null) return false;
        if (regexs != null) {
            for (String rx : regexs) {
                if (rx != null && iface.matches(rx)) return true;
            }
        }
        return iface.startsWith("rndis") || iface.startsWith("usb");
    }

    static JSONObject status(boolean scan) throws Exception {
        JSONObject out = new JSONObject();
        out.put("flight_mode", flight());
        out.put("model", android.os.Build.MODEL);
        out.put("android", android.os.Build.VERSION.RELEASE);
        out.put("sdk", android.os.Build.VERSION.SDK_INT);
        out.put("firmware", android.os.Build.DISPLAY);
        out.put("uptime_seconds", android.os.SystemClock.elapsedRealtime() / 1000);
        out.put("device_time", System.currentTimeMillis());

        boolean wifiOn = wifi.isWifiEnabled();
        out.put("wifi_enabled", wifiOn);
        WifiInfo info = wifi.getConnectionInfo();
        out.put("ssid", info == null ? "" : unquote(info.getSSID()));
        out.put("network_id", info == null ? -1 : info.getNetworkId());
        out.put("rssi", info == null ? -127 : info.getRssi());
        out.put("connection", info == null ? "" : info.getSupplicantState().toString());
        String wifiIp = "0.0.0.0";
        if (info != null) {
            int ip = info.getIpAddress();
            if (ip != 0) {
                wifiIp = (ip & 255) + "." + ((ip >> 8) & 255) + "." + ((ip >> 16) & 255) + "." + ((ip >> 24) & 255);
            } else if (wifiOn) {
                String fallbackIp = ifaceIpv4("wlan0");
                if (!fallbackIp.isEmpty()) wifiIp = fallbackIp;
            }
        }
        out.put("ipv4", wifiIp);

        /* Stock SoftAP / Wireless Hotspot */
        int apState = getApState();
        boolean hotspotOn = (apState == 12 || apState == 13);
        WifiConfiguration apCfg = getApConfig();
        out.put("hotspot_state", apState);
        out.put("hotspot_enabled", hotspotOn);
        out.put("hotspot_ssid", apCfg != null && apCfg.SSID != null ? unquote(apCfg.SSID) : "");
        out.put("hotspot_broadcast", apCfg == null || !apCfg.hiddenSSID);
        out.put("hotspot_security", isApWpa2(apCfg) ? "wpa2" : "open");
        out.put("hotspot_password", apCfg != null && apCfg.preSharedKey != null ? apCfg.preSharedKey : "");
        out.put("hotspot_has_password", apCfg != null && apCfg.preSharedKey != null && apCfg.preSharedKey.length() >= 8);
        out.put("hotspot_ip", hotspotOn ? ifaceIpv4("wlan0") : "");
        out.put("hotspot_clients", hotspotOn ? countArpClients("wlan0") : 0);

        /* USB Tethering */
        boolean usbTethered = false;
        boolean usbAvailable = new File("/sys/class/net/rndis0").exists();
        String usbIface = "";
        try {
            String[] usbRegexs = (String[]) ConnectivityManager.class.getMethod("getTetherableUsbRegexs").invoke(cm);
            String[] tethered = (String[]) ConnectivityManager.class.getMethod("getTetheredIfaces").invoke(cm);
            String[] tetherable = (String[]) ConnectivityManager.class.getMethod("getTetherableIfaces").invoke(cm);
            if (tethered != null) {
                for (String iface : tethered) {
                    if (matchesUsb(iface, usbRegexs)) {
                        usbTethered = true;
                        usbAvailable = true;
                        usbIface = iface;
                        break;
                    }
                }
            }
            if (!usbAvailable && tetherable != null) {
                for (String iface : tetherable) {
                    if (matchesUsb(iface, usbRegexs)) {
                        usbAvailable = true;
                        if (usbIface.isEmpty()) usbIface = iface;
                        break;
                    }
                }
            }
        } catch (Throwable ignored) {}
        if (usbIface.isEmpty() && new File("/sys/class/net/rndis0").exists()) {
            usbIface = "rndis0";
        }
        out.put("usb_tether_enabled", usbTethered);
        out.put("usb_available", usbAvailable);
        out.put("usb_ip", usbTethered ? ifaceIpv4(usbIface) : "");

        /* Mobile Data & Data Roaming */
        boolean mobileData = globalInt("mobile_data", 0) == 1;
        try {
            mobileData = Boolean.TRUE.equals(ConnectivityManager.class.getMethod("getMobileDataEnabled").invoke(cm));
        } catch (Throwable ignored) {}
        out.put("mobile_data_enabled", mobileData);
        out.put("data_roaming_enabled", globalInt("data_roaming", 0) == 1);

        TelephonyManager phone = (TelephonyManager) context.getSystemService(Context.TELEPHONY_SERVICE);
        String imsi = phone.getSubscriberId();
        out.put("sim_prefix", imsi != null && imsi.length() >= 5 ? imsi.substring(0, 5) : "");
        out.put("sim_state", phone.getSimState());
        out.put("data_state", phone.getDataState());
        String op = phone.getNetworkOperatorName();
        out.put("operator", op == null ? "" : op.trim());
        String devId = null;
        try { devId = phone.getDeviceId(); } catch (Throwable ignored) {}
        out.put("imei", devId == null ? "" : devId.trim());

        JSONArray saved = new JSONArray();
        List<WifiConfiguration> configs = wifi.getConfiguredNetworks();
        if (configs != null && !configs.isEmpty()) {
            for (WifiConfiguration c : configs) {
                String s = unquote(c.SSID);
                if (s.isEmpty()) continue;
                saved.put(new JSONObject()
                    .put("id", c.networkId)
                    .put("ssid", s)
                    .put("security", isOpenWifi(c) ? "open" : "wpa2"));
            }
        }
        if (saved.length() == 0) {
            saved = readSupplicantSaved();
        }
        out.put("saved", saved);

        if (scan) {
            boolean started = wifiOn && wifi.startScan();
            if (started) Thread.sleep(2500);
            JSONArray networks = new JSONArray();
            List<ScanResult> results = wifiOn ? wifi.getScanResults() : null;
            if (results != null) {
                for (ScanResult r : results) {
                    if (r.SSID == null || r.SSID.isEmpty()) continue;
                    networks.put(new JSONObject()
                        .put("ssid", r.SSID)
                        .put("rssi", r.level)
                        .put("frequency", r.frequency)
                        .put("security", r.capabilities));
                }
            }
            out.put("networks", networks);
            out.put("scan_started", started);
        }
        return out;
    }

    static String decodeSupplicantSsid(String raw) {
        if (raw == null) return "";
        String v = raw.trim();
        if (v.length() >= 2 && v.startsWith("\"") && v.endsWith("\"")) {
            return v.substring(1, v.length() - 1).replace("\\\"", "\"").replace("\\\\", "\\");
        }
        if (v.length() >= 2 && (v.length() % 2) == 0) {
            byte[] buf = new byte[v.length() / 2];
            for (int i = 0; i < v.length(); i += 2) {
                int hi = Character.digit(v.charAt(i), 16);
                int lo = Character.digit(v.charAt(i + 1), 16);
                if (hi < 0 || lo < 0) return "";
                buf[i / 2] = (byte) ((hi << 4) | lo);
            }
            try {
                return new String(buf, "UTF-8");
            } catch (Throwable ignored) {}
        }
        return "";
    }

    static JSONArray readSupplicantSaved() {
        JSONArray out = new JSONArray();
        File conf = new File("/data/misc/wifi/wpa_supplicant.conf");
        if (!conf.isFile()) return out;
        BufferedReader br = null;
        try {
            br = new BufferedReader(new InputStreamReader(new FileInputStream(conf), "UTF-8"));
            Set<String> seen = new LinkedHashSet<String>();
            boolean inBlock = false;
            String ssid = "";
            String security = "wpa2";
            boolean hasKeyMgmt = false;
            String line;
            while ((line = br.readLine()) != null) {
                String trimmed = line.trim();
                if (!inBlock) {
                    if (trimmed.startsWith("network=") && trimmed.endsWith("{")) {
                        inBlock = true;
                        ssid = "";
                        security = "wpa2";
                        hasKeyMgmt = false;
                    }
                    continue;
                }
                if ("}".equals(trimmed)) {
                    if (!ssid.isEmpty() && seen.add(ssid)) {
                        out.put(new JSONObject()
                            .put("id", -1)
                            .put("ssid", ssid)
                            .put("security", security));
                    }
                    inBlock = false;
                    continue;
                }
                if (trimmed.startsWith("ssid=")) {
                    ssid = decodeSupplicantSsid(trimmed.substring(5));
                } else if (trimmed.startsWith("key_mgmt=")) {
                    hasKeyMgmt = true;
                    String km = trimmed.substring(9).trim().toUpperCase();
                    security = "NONE".equals(km) ? "open" : "wpa2";
                } else if (trimmed.startsWith("wep_key0=") && "open".equals(security)) {
                    security = "wpa2";
                } else if (!hasKeyMgmt && trimmed.startsWith("psk=")) {
                    security = "wpa2";
                }
            }
        } catch (Throwable ignored) {
        } finally {
            if (br != null) try { br.close(); } catch (Throwable ignored) {}
        }
        return out;
    }

    static boolean removeSupplicantSsid(String targetSsid) {
        if (targetSsid == null || targetSsid.isEmpty()) return false;
        File conf = new File("/data/misc/wifi/wpa_supplicant.conf");
        if (!conf.isFile()) return false;
        List<String> kept = new ArrayList<String>();
        List<String> block = new ArrayList<String>();
        boolean inBlock = false;
        boolean removed = false;
        String blockSsid = "";
        BufferedReader br = null;
        try {
            br = new BufferedReader(new InputStreamReader(new FileInputStream(conf), "UTF-8"));
            String line;
            while ((line = br.readLine()) != null) {
                String trimmed = line.trim();
                if (!inBlock) {
                    if (trimmed.startsWith("network=") && trimmed.endsWith("{")) {
                        inBlock = true;
                        block.clear();
                        block.add(line);
                        blockSsid = "";
                    } else {
                        kept.add(line);
                    }
                    continue;
                }
                block.add(line);
                if (trimmed.startsWith("ssid=")) {
                    blockSsid = decodeSupplicantSsid(trimmed.substring(5));
                } else if ("}".equals(trimmed)) {
                    if (targetSsid.equals(blockSsid)) {
                        removed = true;
                    } else {
                        kept.addAll(block);
                    }
                    inBlock = false;
                }
            }
            if (inBlock) kept.addAll(block);
        } catch (Throwable e) {
            return false;
        } finally {
            if (br != null) try { br.close(); } catch (Throwable ignored) {}
        }
        if (!removed) return false;
        FileOutputStream fos = null;
        try {
            StringBuilder sb = new StringBuilder();
            for (String l : kept) sb.append(l).append('\n');
            fos = new FileOutputStream(conf, false);
            fos.write(sb.toString().getBytes("UTF-8"));
            fos.flush();
            return true;
        } catch (Throwable e) {
            return false;
        } finally {
            if (fos != null) try { fos.close(); } catch (Throwable ignored) {}
        }
    }

    static void ensureClientWifiOn() throws Exception {
        if (!wifi.isWifiEnabled()) {
            int hotspot = getApState();
            if (hotspot == 12 || hotspot == 13) {
                if (!setApEnabled(null, false)) throw new Exception("无法关闭设备热点");
                Thread.sleep(1500);
            }
            if (!wifi.setWifiEnabled(true)) throw new Exception("无法开启 Wi-Fi");
            for (int i = 0; i < 12; i++) {
                Thread.sleep(300);
                if (wifi.isWifiEnabled() && wifi.getConfiguredNetworks() != null) break;
            }
        }
    }

    public static void main(String[] args) {
        try {
            Looper.prepareMainLooper();
            Class<?> activityThread = Class.forName("android.app.ActivityThread");
            Method systemMain = activityThread.getDeclaredMethod("systemMain");
            Object thread = systemMain.invoke(null);
            context = (Context) activityThread.getDeclaredMethod("getSystemContext").invoke(thread);
            wifi = (WifiManager) context.getSystemService(Context.WIFI_SERVICE);
            cm = (ConnectivityManager) context.getSystemService(Context.CONNECTIVITY_SERVICE);

            BufferedReader reader = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
            String line = reader.readLine();
            if (line == null || line.length() > 8192) throw new Exception("请求无效");
            JSONObject req = new JSONObject(line);
            String action = req.getString("action");

            if (action.equals("status") || action.equals("scan")) {
                System.out.println(status(action.equals("scan")));
                System.exit(0);
                return;
            }

            if (action.equals("flight")) {
                boolean enabled = req.getBoolean("enabled");
                if (flight() != enabled) {
                    boolean keepWifi = wifi.isWifiEnabled();
                    if (!putGlobalInt("airplane_mode_on", enabled ? 1 : 0)) throw new Exception("无法保存飞行模式");
                    Process broadcast = new ProcessBuilder("/system/bin/sh", "/system/bin/am", "broadcast", "-a",
                        "android.intent.action.AIRPLANE_MODE", "--ez", "state", String.valueOf(enabled)).start();
                    if (broadcast.waitFor() != 0) throw new Exception("飞行模式广播失败");
                    if (keepWifi) wifi.setWifiEnabled(true);
                }
                System.out.println(new JSONObject().put("ok", true).put("flight_mode", flight()));
                System.exit(0);
                return;
            }

            if (action.equals("wifi")) {
                boolean enabled = req.getBoolean("enabled");
                if (enabled) {
                    ensureClientWifiOn();
                } else if (wifi.isWifiEnabled()) {
                    if (!wifi.setWifiEnabled(false)) throw new Exception("无法关闭 Wi-Fi");
                    Thread.sleep(600);
                }
                System.out.println(new JSONObject().put("ok", true).put("wifi_enabled", wifi.isWifiEnabled()));
                System.exit(0);
                return;
            }

            if (action.equals("wifi_disconnect")) {
                if (!wifi.disconnect()) throw new Exception("无法断开当前 Wi-Fi");
                System.out.println(new JSONObject().put("ok", true));
                System.exit(0);
                return;
            }

            if (action.equals("wifi_forget")) {
                int id = req.optInt("network_id", -1);
                String ssid = req.optString("ssid", "");
                if (wifi.isWifiEnabled()) {
                    if (id < 0 && !ssid.isEmpty()) {
                        List<WifiConfiguration> list = wifi.getConfiguredNetworks();
                        if (list != null) {
                            for (WifiConfiguration c : list) {
                                if (ssid.equals(unquote(c.SSID))) {
                                    id = c.networkId;
                                    break;
                                }
                            }
                        }
                    }
                    if (id < 0 || !wifi.removeNetwork(id) || !wifi.saveConfiguration()) {
                        throw new Exception("无法移除已保存的 Wi-Fi");
                    }
                } else {
                    if (ssid.isEmpty() || !removeSupplicantSsid(ssid)) {
                        throw new Exception("无法移除已保存的 Wi-Fi");
                    }
                }
                System.out.println(new JSONObject().put("ok", true));
                System.exit(0);
                return;
            }

            if (action.equals("hotspot")) {
                boolean enabled = req.getBoolean("enabled");
                if (enabled) {
                    if (wifi.isWifiEnabled()) {
                        wifi.setWifiEnabled(false);
                        Thread.sleep(1000);
                    }
                    int st = getApState();
                    if (st != 12 && st != 13) {
                        if (!setApEnabled(null, true)) throw new Exception("无法开启无线热点");
                        Thread.sleep(1200);
                    }
                } else {
                    int st = getApState();
                    if (st == 12 || st == 13) {
                        if (!setApEnabled(null, false)) throw new Exception("无法关闭无线热点");
                        Thread.sleep(1000);
                    }
                }
                int after = getApState();
                System.out.println(new JSONObject().put("ok", true).put("hotspot_enabled", after == 12 || after == 13));
                System.exit(0);
                return;
            }

            if (action.equals("hotspot_config")) {
                WifiConfiguration cfg = getApConfig();
                if (cfg == null) cfg = new WifiConfiguration();
                if (req.has("ssid")) {
                    String ssid = req.getString("ssid");
                    byte[] rawSsid = ssid.getBytes("UTF-8");
                    if (rawSsid.length < 1 || rawSsid.length > 32 || ssid.indexOf('\0') >= 0 || ssid.indexOf('\n') >= 0 || ssid.indexOf('\r') >= 0) {
                        throw new Exception("热点名称无效");
                    }
                    cfg.SSID = ssid;
                }
                if (req.has("broadcast")) {
                    cfg.hiddenSSID = !req.getBoolean("broadcast");
                }
                if (req.has("security")) {
                    String sec = req.getString("security");
                    String pwd = req.optString("password", "");
                    cfg.allowedKeyManagement.clear();
                    cfg.allowedAuthAlgorithms.clear();
                    cfg.allowedAuthAlgorithms.set(WifiConfiguration.AuthAlgorithm.OPEN);
                    if ("open".equals(sec)) {
                        cfg.allowedKeyManagement.set(WifiConfiguration.KeyMgmt.NONE);
                        cfg.preSharedKey = "";
                    } else if ("wpa2".equals(sec)) {
                        cfg.allowedKeyManagement.set(wpa2KeyMgmt());
                        if (!pwd.isEmpty()) {
                            int len = pwd.getBytes("UTF-8").length;
                            if (len < 8 || len > 63) throw new Exception("热点密码需为 8–63 字节");
                            cfg.preSharedKey = pwd;
                        } else if (cfg.preSharedKey == null || cfg.preSharedKey.getBytes("UTF-8").length < 8) {
                            throw new Exception("热点密码需为 8–63 字节");
                        }
                    } else {
                        throw new Exception("热点参数无效");
                    }
                }
                Method setCfg = WifiManager.class.getMethod("setWifiApConfiguration", WifiConfiguration.class);
                if (!Boolean.TRUE.equals(setCfg.invoke(wifi, cfg))) {
                    throw new Exception("无法保存热点配置");
                }
                int st = getApState();
                if (st == 12 || st == 13) {
                    setApEnabled(null, false);
                    Thread.sleep(1200);
                    if (!setApEnabled(cfg, true)) throw new Exception("无法开启无线热点");
                    Thread.sleep(1000);
                }
                System.out.println(new JSONObject().put("ok", true));
                System.exit(0);
                return;
            }

            if (action.equals("usb_tether")) {
                boolean enabled = req.getBoolean("enabled");
                Method m = ConnectivityManager.class.getMethod("setUsbTethering", boolean.class);
                int rc = (Integer) m.invoke(cm, enabled);
                if (rc != 0) throw new Exception("USB 网络共享切换失败");
                System.out.println(new JSONObject().put("ok", true).put("usb_tether_enabled", enabled));
                System.exit(0);
                return;
            }

            if (action.equals("mobile_data")) {
                boolean enabled = req.getBoolean("enabled");
                Method m = ConnectivityManager.class.getMethod("setMobileDataEnabled", boolean.class);
                m.invoke(cm, enabled);
                putGlobalInt("mobile_data", enabled ? 1 : 0);
                System.out.println(new JSONObject().put("ok", true).put("mobile_data_enabled", enabled));
                System.exit(0);
                return;
            }

            if (action.equals("data_roaming")) {
                boolean enabled = req.getBoolean("enabled");
                if (!putGlobalInt("data_roaming", enabled ? 1 : 0)) {
                    throw new Exception("数据漫游设置失败");
                }
                System.out.println(new JSONObject().put("ok", true).put("data_roaming_enabled", enabled));
                System.exit(0);
                return;
            }

            if (!action.equals("connect")) throw new Exception("不支持的操作");
            ensureClientWifiOn();
            int id = req.optInt("network_id", -1);
            String reqSsid = req.optString("ssid", "");
            String password = req.optString("password", "");
            boolean useSaved = req.optBoolean("saved", false);
            List<WifiConfiguration> configured = wifi.getConfiguredNetworks();
            if (id >= 0) {
                boolean found = false;
                if (configured != null) {
                    for (WifiConfiguration c : configured) {
                        if (c.networkId == id) {
                            found = true;
                            break;
                        }
                    }
                }
                if (!found && !reqSsid.isEmpty() && configured != null) {
                    for (WifiConfiguration c : configured) {
                        if (reqSsid.equals(unquote(c.SSID))) {
                            id = c.networkId;
                            found = true;
                            break;
                        }
                    }
                }
                if (!found) throw new Exception("已保存的网络不存在");
            } else if ((useSaved || password.isEmpty()) && !reqSsid.isEmpty() && configured != null) {
                for (WifiConfiguration c : configured) {
                    if (reqSsid.equals(unquote(c.SSID)) && (useSaved || isOpenWifi(c))) {
                        id = c.networkId;
                        break;
                    }
                }
                if (useSaved && id < 0) throw new Exception("已保存的网络不存在");
            }
            if (id < 0) {
                String ssid = req.getString("ssid"), security = req.optString("security", "wpa2");
                if (ssid.getBytes("UTF-8").length < 1 || ssid.getBytes("UTF-8").length > 32 || ssid.indexOf('\0') >= 0) throw new Exception("Wi-Fi 名称无效");
                WifiConfiguration c = new WifiConfiguration();
                c.SSID = quoted(ssid);
                if (security.equals("open")) {
                    if (!password.isEmpty()) throw new Exception("开放网络不需要密码");
                    c.allowedKeyManagement.set(WifiConfiguration.KeyMgmt.NONE);
                } else if (security.equals("wpa2")) {
                    if (password.getBytes("UTF-8").length < 8 || password.getBytes("UTF-8").length > 63) throw new Exception("Wi-Fi 密码需为 8–63 字节");
                    c.allowedKeyManagement.set(WifiConfiguration.KeyMgmt.WPA_PSK);
                    c.preSharedKey = quoted(password);
                } else {
                    throw new Exception("此 Android 版本仅支持开放或 WPA/WPA2 个人网络");
                }
                c.status = WifiConfiguration.Status.ENABLED;
                if (configured != null) {
                    for (WifiConfiguration old : configured) if (c.SSID.equals(old.SSID)) c.networkId = old.networkId;
                }
                id = c.networkId >= 0 ? wifi.updateNetwork(c) : wifi.addNetwork(c);
                if (id < 0 || !wifi.saveConfiguration()) throw new Exception("无法保存 Wi-Fi 网络");
            }
            if (!wifi.enableNetwork(id, true) || !wifi.reconnect()) throw new Exception("无法开始连接");
            System.out.println(new JSONObject().put("ok", true).put("network_id", id).put("message", "已开始连接，网络切换时页面可能短暂断开"));
            System.exit(0);
        } catch (Throwable e) {
            try {
                System.out.println(new JSONObject().put("error", e.getMessage() == null ? "Android Wi-Fi 服务不可用" : e.getMessage()));
            } catch (Exception ignored) {}
            System.exit(1);
        }
    }
}
