#!/system/bin/sh
# Magisk late_start service. Installed only by an explicit toolkit install.
# service-control backgrounds itself so Android boot is never blocked.
/system/bin/sh /data/adb/sp970-wfc/service-control.sh start \
    >>/data/adb/sp970-wfc/log/boot.log 2>&1
