#!/system/bin/sh
# Magisk late_start service. Preserve saved network switches; always resume Tailscale.
# Both workers run in the background so Android boot is never blocked.
umask 077
BASE=/data/adb/sp970-wfc
BB=/system/xbin/busybox
mkdir -p "$BASE/log" "$BASE/run"

note() {
    echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') $*"
}

start_services() {
    ATTEMPT=0
    while [ "$ATTEMPT" -lt 12 ]; do
        ACTION=start
        [ ! -e "$BASE/disabled" ] || ACTION=admin
        /system/bin/sh "$BASE/service-control.sh" "$ACTION" && return 0
        ATTEMPT=$((ATTEMPT + 1))
        sleep 5
    done
    note 'WFC boot startup failed after 12 attempts; inspect admin.log and supervisor.log'
    return 1
}

resume_tailscale() {
    BOOT_WAIT=0
    while [ "$(getprop sys.boot_completed)" != 1 ]; do
        if [ "$BOOT_WAIT" -ge 90 ]; then
            note 'Android boot did not complete within 180 seconds'
            return 1
        fi
        sleep 2
        BOOT_WAIT=$((BOOT_WAIT + 1))
    done
    if [ ! -x "$BASE/bin/tailscale" ] || [ ! -x "$BASE/bin/tailscaled" ]; then
        note 'Tailscale components are not installed; autostart preference retained'
        return 0
    fi
    TS_WAIT=0
    while [ ! -S "$BASE/run/tailscaled.sock" ]; do
        if [ "$TS_WAIT" -ge 30 ]; then
            note 'Tailscale daemon socket did not become ready within 60 seconds'
            return 1
        fi
        sleep 2
        TS_WAIT=$((TS_WAIT + 1))
    done
    # No up flags: resume the saved account and preferences without resetting them.
    # The outer timeout also bounds a missing login or unavailable control server.
    if SSL_CERT_FILE="$BASE/ca-roots.pem" HOME="$BASE/tailscale" \
        "$BB" timeout 20 "$BASE/bin/tailscale" --socket="$BASE/run/tailscaled.sock" up \
        >/dev/null 2>&1; then
        note 'Tailscale connection enabled using saved preferences'
        return 0
    fi
    note 'Tailscale resume did not finish; check login and tailscaled.log'
    return 1
}

# Re-enable at each boot; subsequent manual changes are respected until next boot.
printf '1\n' >"$BASE/tailscale-enabled"
(start_services) >>"$BASE/log/boot.log" 2>&1 </dev/null &
(resume_tailscale) >>"$BASE/log/boot-network.log" 2>&1 </dev/null &
