#!/system/bin/sh
# Run before module mounting and Android's package scan. Keep the patched
# system APK readable even if this firmware restores unexpected metadata.
MODDIR=${0%/*}
BB=/system/xbin/busybox
APK=$MODDIR/system/app/pingbao.apk
[ -f "$APK" ] || exit 0
"$BB" chown 0:0 "$MODDIR/system" "$MODDIR/system/app" "$APK"
"$BB" chmod 755 "$MODDIR/system" "$MODDIR/system/app"
"$BB" chmod 644 "$APK"
# skip_mount prevents this old Magisk build from cloning corrupt file metadata.
"$BB" mount -o bind "$APK" /system/app/pingbao.apk
