#!/system/bin/sh
# Native RBMaster_srv entry point, overlaid by the SP970 SSH Magisk module.
# Preserve OEM initialization, then apply the saved SSH preference.
umask 077
if [ -f /cache/cache/device/fun/test/l0331 ]; then
    encrypt_dest=$(cat /cache/cache/device/fun/test/l0331)
    setprop persist.encrypt_dest_code "$encrypt_dest"
    setprop persist.oem_locked false
else
    setprop persist.oem_locked true
fi
/system/xbin/init.publish.sh
exec /system/bin/sh /data/adb/sp970-wfc/service-control.sh ssh
