#!/system/bin/sh
# Use explicit binds on this firmware: Magisk 22.1's metadata cloning changes
# ownership on some existing system files. Requires this module's skip_mount.
MODDIR=${0%/*}
BB=/system/xbin/busybox
LAUNCHER=$MODDIR/system/bin/start-dropbear
KEYS=$MODDIR/system/sp970-ssh/authorized_keys
"$BB" chown 0:0 "$LAUNCHER" "$MODDIR/system/sp970-ssh" "$KEYS"
"$BB" chmod 755 "$LAUNCHER"
"$BB" chmod 700 "$MODDIR/system/sp970-ssh"
"$BB" chmod 600 "$KEYS"
"$BB" mount -o bind "$LAUNCHER" /system/bin/start-dropbear
"$BB" mount -o bind "$KEYS" /system/sp970-ssh/authorized_keys
