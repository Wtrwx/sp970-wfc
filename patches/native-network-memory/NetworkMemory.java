package com.pingbao;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.net.ConnectivityManager;
import android.net.wifi.WifiConfiguration;
import android.net.wifi.WifiManager;
import android.os.Handler;
import android.os.Looper;
import android.util.Log;

/** Keeps only the three network switches; Tailscale has its own boot policy. */
public final class NetworkMemory extends BroadcastReceiver {
    private static final String TAG = "SP970 NetworkMemory";
    private static final String AP_ACTION = "android.net.wifi.WIFI_AP_STATE_CHANGED";
    private static final String USB_ACTION = "android.net.conn.TETHER_STATE_CHANGED";
    private static SharedPreferences prefs;
    private static Context app;
    private static boolean wifiAtBoot;
    private static boolean hotspotAtBoot;
    private static boolean usbAtBoot;
    private static boolean watching;
    private static boolean restoring = true;
    private static boolean shuttingDown;

    private static WifiManager wifi(Context context) {
        return (WifiManager) context.getSystemService(Context.WIFI_SERVICE);
    }

    private static int apState(Context context) throws Exception {
        return (Integer) WifiManager.class.getMethod("getWifiApState").invoke(wifi(context));
    }

    private static boolean usbEnabled(Context context) throws Exception {
        ConnectivityManager cm = (ConnectivityManager) context.getSystemService(Context.CONNECTIVITY_SERVICE);
        String[] active = (String[]) ConnectivityManager.class.getMethod("getTetheredIfaces").invoke(cm);
        if (active != null) {
            for (String name : active) {
                if (name != null && (name.startsWith("rndis") || name.startsWith("usb"))) return true;
            }
        }
        return false;
    }

    private static void savedHotspot(boolean enabled) throws Exception {
        Class.forName("websocket_server.RootCmd").getMethod("setWifiState", int.class)
            .invoke(null, enabled ? 1 : 0);
    }

    private static synchronized void init(Context context) {
        if (prefs != null) return;
        app = context.getApplicationContext();
        prefs = app.getSharedPreferences("sp970_network_memory", Context.MODE_PRIVATE);
        boolean oldHotspot = false;
        boolean oldUsb = false;
        try {
            oldHotspot = (Boolean) Class.forName("websocket_server.RootCmd")
                .getMethod("getWifiState").invoke(null);
            oldUsb = usbEnabled(app);
        } catch (Throwable error) {
            Log.w(TAG, "Could not read legacy network switches");
        }
        wifiAtBoot = prefs.getBoolean("wifi", wifi(app).isWifiEnabled());
        hotspotAtBoot = prefs.getBoolean("hotspot", oldHotspot);
        usbAtBoot = prefs.getBoolean("usb_tether", oldUsb);
        if (hotspotAtBoot) wifiAtBoot = false;
        prefs.edit().putBoolean("wifi", wifiAtBoot).putBoolean("hotspot", hotspotAtBoot)
            .putBoolean("usb_tether", usbAtBoot).commit();
        try { savedHotspot(hotspotAtBoot); } catch (Throwable ignored) {}
    }

    public static boolean hotspotAtBoot(Context context) {
        init(context);
        return hotspotAtBoot;
    }

    public static boolean usbAtBoot(Context context) {
        init(context);
        return usbAtBoot;
    }

    private static void restore() {
        try {
            WifiManager wm = wifi(app);
            int state = apState(app);
            if (hotspotAtBoot) {
                if (wm.isWifiEnabled()) wm.setWifiEnabled(false);
                if (state != 12 && state != 13) {
                    WifiManager.class.getMethod("setWifiApEnabled", WifiConfiguration.class, boolean.class)
                        .invoke(wm, null, true);
                }
            } else {
                if (state == 12 || state == 13) {
                    WifiManager.class.getMethod("setWifiApEnabled", WifiConfiguration.class, boolean.class)
                        .invoke(wm, null, false);
                }
                wm.setWifiEnabled(wifiAtBoot);
            }
            ConnectivityManager cm = (ConnectivityManager) app.getSystemService(Context.CONNECTIVITY_SERVICE);
            ConnectivityManager.class.getMethod("setUsbTethering", boolean.class).invoke(cm, usbAtBoot);
        } catch (Throwable error) {
            Log.w(TAG, "Could not restore all saved network switches");
        }
    }

    public static synchronized void finishBoot(Context context) {
        init(context);
        if (watching) return;
        watching = true;
        restore();
        IntentFilter filter = new IntentFilter();
        filter.addAction(WifiManager.WIFI_STATE_CHANGED_ACTION);
        filter.addAction(AP_ACTION);
        filter.addAction(USB_ACTION);
        filter.addAction(Intent.ACTION_SHUTDOWN);
        filter.setPriority(1000);
        app.registerReceiver(new NetworkMemory(), filter);
        new Handler(Looper.getMainLooper()).postDelayed(new Runnable() {
            @Override public void run() {
                restore();
                restoring = false;
                Log.i(TAG, "Restored saved Wi-Fi, hotspot and USB switches");
            }
        }, 3000);
    }

    @Override public void onReceive(Context context, Intent intent) {
        String action = intent.getAction();
        if (Intent.ACTION_SHUTDOWN.equals(action)) {
            shuttingDown = true;
            return;
        }
        if (restoring || shuttingDown || isInitialStickyBroadcast()) return;
        try {
            if (WifiManager.WIFI_STATE_CHANGED_ACTION.equals(action)) {
                int state = intent.getIntExtra(WifiManager.EXTRA_WIFI_STATE, -1);
                if (state == WifiManager.WIFI_STATE_ENABLED || state == WifiManager.WIFI_STATE_DISABLED) {
                    boolean enabled = state == WifiManager.WIFI_STATE_ENABLED;
                    prefs.edit().putBoolean("wifi", enabled).commit();
                    Log.i(TAG, "Saved Wi-Fi switch=" + enabled);
                }
            } else if (AP_ACTION.equals(action)) {
                int state = intent.getIntExtra("wifi_state", -1);
                if (state == 11 || state == 13) {
                    boolean enabled = state == 13;
                    prefs.edit().putBoolean("hotspot", enabled).commit();
                    savedHotspot(enabled);
                    Log.i(TAG, "Saved hotspot switch=" + enabled);
                }
            } else if (USB_ACTION.equals(action)) {
                boolean enabled = usbEnabled(app);
                prefs.edit().putBoolean("usb_tether", enabled).commit();
                Log.i(TAG, "Saved USB tether switch=" + enabled);
            }
        } catch (Throwable error) {
            Log.w(TAG, "Could not save a network switch");
        }
    }
}
