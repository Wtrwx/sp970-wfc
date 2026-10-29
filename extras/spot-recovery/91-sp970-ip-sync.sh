#!/system/bin/sh
# Optional proxy subscription worker. Owns no radio, SIM or global routing settings.
BASE=/data/adb/sp970-wfc
BB=/system/xbin/busybox
export ANDROID_ROOT=/system ANDROID_DATA=/data
export LD_LIBRARY_PATH=/system/vendor/lib:/system/lib
export PATH=/system/bin:/system/xbin:/sbin
umask 077
mkdir -p "$BASE/run" "$BASE/log"
[ -f "$BASE/ip-sync-disabled" ] && exit 0
[ -x "$BASE/bin/proxy-ip-sync" ] || exit 0
[ -f "$BASE/ip-sync.json" ] || exit 0
"$BB" nohup "$BB" setsid "$BASE/bin/proxy-ip-sync" \
    3>&- 4>&- 5>&- 6>&- 7>&- 8>&- 9>&- \
    >>"$BASE/log/ip-sync.log" 2>&1 </dev/null &
