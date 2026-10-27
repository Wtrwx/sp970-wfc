#!/system/bin/sh
# SP970-B WFC service. Owns only its processes and metric-27027 routes.
# The native firmware still manages Wi-Fi, flight mode and USB functions.
umask 077
BASE=/data/adb/sp970-wfc
SELF=$BASE/service-control.sh
BB=/system/xbin/busybox
IP=/system/bin/ip
NETWORK_MODE=$(cat "$BASE/network-mode" 2>/dev/null)
NODE_IP=$(cat "$BASE/proxy-ipv4" 2>/dev/null)
EPDG=$(cat "$BASE/epdg-ipv4" 2>/dev/null)
[ -n "$EPDG" ] || EPDG=84.246.115.151
case "$NETWORK_MODE" in hysteria2|direct) ;; *) echo "Invalid network mode"; exit 1;; esac
if [ "$NETWORK_MODE" = hysteria2 ]; then
    case "$NODE_IP" in ''|*[!0-9.]*) echo "Invalid proxy IPv4"; exit 1;; esac
fi
METRIC=27027
RUN=$BASE/run
LOG=$BASE/log
CONFIG=$BASE/psm-direct-tun.json
PROXY=$BASE/bin/hysteria
WFC=$BASE/bin/sp970-wfc
export PATH=/system/bin:/system/xbin:/sbin
export LD_LIBRARY_PATH=/system/vendor/lib:/system/lib

mkdir -p "$RUN" "$LOG"
chmod 700 "$BASE" "$RUN" "$LOG"

note() {
    echo "$(date -u '+%Y-%m-%dT%H:%M:%SZ') $*"
}

matches() {
    OPID=$2
    case "$OPID" in ''|*[!0-9]*|0|1) return 1;; esac
    [ -r "/proc/$OPID/cmdline" ] || return 1
    OEXE=$($BB readlink "/proc/$OPID/exe")
    OCMD=$($BB tr '\000' ' ' <"/proc/$OPID/cmdline")
    case "$1:$OEXE" in
        proxy:$PROXY|proxy:/data/local/tmp/sp970_hysteria_stable_tun)
            case " $OCMD " in *" client "*" -c $CONFIG "*) return 0;; esac;;
        wfc:$WFC|wfc:/data/local/tmp/sp970_wfc_preflight) return 0;;
        supervisor:/system/bin/mksh|supervisor:/system/bin/sh)
            case " $OCMD " in *" $SELF supervise "*) return 0;; esac;;
    esac
    return 1
}

owned() {
    [ -r "$RUN/$1.pid" ] || return 1
    matches "$1" "$(cat "$RUN/$1.pid")"
}

adopt() {
    owned "$1" && return 0
    [ -r "$2" ] || return 1
    if matches "$1" "$(cat "$2")"; then
        echo "$OPID" >"$RUN/$1.pid"
        note "adopted $1 pid=$OPID"
        return 0
    fi
    return 1
}

stop_role() {
    STOP_ROLE=$1
    if owned "$STOP_ROLE"; then
        note "stopping $STOP_ROLE pid=$OPID"
        kill -TERM "$OPID"
        STOP_TRIES=0
        while owned "$STOP_ROLE" && [ "$STOP_TRIES" -lt 15 ]; do
            sleep 1
            STOP_TRIES=$((STOP_TRIES + 1))
        done
        if owned "$STOP_ROLE"; then
            note "$STOP_ROLE did not stop gracefully; terminating owned pid=$OPID"
            kill -KILL "$OPID"
            sleep 1
        fi
    fi
    rm -f "$RUN/$STOP_ROLE.pid"
}

remove_routes() {
    "$IP" -4 route del 0.0.0.0/1 dev hypsm0 metric "$METRIC" 2>/dev/null
    "$IP" -4 route del 128.0.0.0/1 dev hypsm0 metric "$METRIC" 2>/dev/null
    "$IP" -6 route del 2000::/3 dev hypsm0 metric "$METRIC" 2>/dev/null
    [ -z "$NODE_IP" ] || "$IP" -4 route del "$NODE_IP/32" dev wlan0 metric "$METRIC" 2>/dev/null
    return 0
}

set_state() {
    if [ "$SERVICE_STATE" != "$1" ]; then
        SERVICE_STATE=$1
        echo "$1" >"$RUN/state"
        note "$1"
    fi
}

rotate() {
    [ -f "$1" ] || return 0
    "$BB" mv -f "$1" "$1.1"
}

prepare_binaries() {
    for ASSET in sp970-wfc sp970_uim_bridge sp970_ims_xfrm; do
        [ -x "$BASE/bin/$ASSET" ] || return 1
    done
    if [ "$NETWORK_MODE" = hysteria2 ]; then
        [ -x "$PROXY" ] || return 1
    fi
    [ -c /dev/tun ] || "$BB" mknod -m 600 /dev/tun c 10 200 || return 1
    mkdir -p /dev/net
    [ -e /dev/net/tun ] || ln -s ../tun /dev/net/tun || return 1
}

proxy_routes() {
    # Pin the outer server to the physical Wi-Fi gateway to avoid a loop.
    "$IP" -4 route replace "$NODE_IP/32" via "$GATEWAY" dev wlan0 metric "$METRIC" || return 1
    [ -e /sys/class/net/hypsm0 ] || return 1
    "$IP" -4 route replace 0.0.0.0/1 dev hypsm0 metric "$METRIC" || return 1
    "$IP" -4 route replace 128.0.0.0/1 dev hypsm0 metric "$METRIC" || return 1
    "$IP" -6 route replace 2000::/3 dev hypsm0 metric "$METRIC" || return 1
}

start_proxy() {
    if [ -e /sys/class/net/hypsm0 ]; then
        set_state 'waiting: proxy interface has another owner'
        return 1
    fi
    remove_routes
    "$IP" -4 route replace "$NODE_IP/32" via "$GATEWAY" dev wlan0 metric "$METRIC" || return 1
    rotate "$LOG/proxy.log"
    "$PROXY" client --disable-update-check --log-level info -c "$CONFIG" \
        9>&- >"$LOG/proxy.log" 2>&1 </dev/null &
    echo $! >"$RUN/proxy.pid"
    "$BB" cp "$RUN/proxy.pid" /data/local/tmp/psm.pid
    note "started proxy pid=$(cat "$RUN/proxy.pid")"
}

start_wfc() {
    [ ! -e /sys/class/net/spwfc0 ] || {
        set_state 'waiting: IMS interface has another owner'
        return 1
    }
    rotate "$LOG/wfc.log"
    # One process per bounded session: process exit also releases a TUN read
    # still blocked in this old kernel. The supervisor starts the next session.
    # Deliberately omit --self-test. Never sends test messages.
    "$WFC" --dh-group 14 --port 4500 --epdg-ip "$EPDG" \
        --dns 1.1.1.1 --ims-probe --hold 25m \
        9>&- >"$LOG/wfc.log" 2>&1 </dev/null &
    echo $! >"$RUN/wfc.pid"
    "$BB" cp "$RUN/wfc.pid" /data/local/tmp/sp970-wfc-active.pid
    note "started WFC pid=$(cat "$RUN/wfc.pid")"
}

ensure_webui() {
    # Respect any existing native listener. Start only if port 80 is unused.
    if "$BB" awk '$4 == "0A" && $2 ~ /:0050$/ {found=1} END {exit !found}' \
        /proc/net/tcp /proc/net/tcp6; then
        return 0
    fi
    /system/bin/nginx -t -p /sdcard/nginx -c /system/etc/nginx/conf/nginx.conf \
        >/dev/null 2>&1 || return 1
    rotate "$LOG/nginx.log"
    /system/bin/nginx -p /sdcard/nginx -c /system/etc/nginx/conf/nginx.conf \
        9>&- >"$LOG/nginx.log" 2>&1 </dev/null &
    echo $! >"$RUN/nginx.pid"
    note 'started native WebUI with WFC API route'
}

finish() {
    [ "$FINISHING" = 1 ] && return
    FINISHING=1
    trap '' TERM INT
    stop_role wfc
    stop_role proxy
    remove_routes
    rm -f "$RUN/supervisor.pid"
    echo stopped >"$RUN/state"
    note 'service stopped'
}

supervise() {
    # BusyBox owns the lock descriptor: this firmware's mksh closes shell-
    # created descriptors before running external commands such as flock.
    echo $$ >"$RUN/supervisor.pid"
    trap 'exit 0' TERM INT
    trap finish EXIT
    adopt proxy /data/local/tmp/psm.pid
    adopt wfc /data/local/tmp/sp970-wfc-active.pid
    if owned wfc; then
        case " $OCMD " in *" --serve "*) stop_role wfc;; esac
    fi
    if ! prepare_binaries; then
        set_state 'error: persistent runtime assets unavailable'
        exit 1
    fi
    NEXT_WFC=0
    PREVIOUS_GATEWAY=
    while [ ! -e "$BASE/disabled" ]; do
        if [ "$(getprop sys.boot_completed)" != 1 ]; then
            set_state 'waiting: Android boot'
        elif [ "$(settings get global airplane_mode_on)" != 1 ]; then
            stop_role wfc
            stop_role proxy
            remove_routes
            set_state 'paused: flight mode is off'
        else
            GATEWAY=$("$IP" -4 route show default dev wlan0 | "$BB" awk '/^default via / {print $3;exit}')
            if [ -z "$GATEWAY" ]; then
                stop_role wfc
                stop_role proxy
                remove_routes
                set_state 'waiting: Wi-Fi gateway'
            else
                if [ -n "$PREVIOUS_GATEWAY" ] && [ "$GATEWAY" != "$PREVIOUS_GATEWAY" ]; then
                    stop_role wfc
                    stop_role proxy
                    remove_routes
                fi
                PREVIOUS_GATEWAY=$GATEWAY
                NETWORK_READY=0
                if [ "$NETWORK_MODE" = direct ]; then
                    NETWORK_READY=1
                else
                    if ! owned proxy; then
                        stop_role wfc
                        start_proxy
                    fi
                    if owned proxy && proxy_routes; then NETWORK_READY=1; fi
                fi
                if [ "$NETWORK_READY" = 1 ]; then
                    ensure_webui
                    NOW=$($BB awk '{print int($1)}' /proc/uptime)
                    if ! owned wfc && [ "$NOW" -ge "$NEXT_WFC" ]; then
                        start_wfc
                        NEXT_WFC=$((NOW + 30))
                    fi
                    if owned wfc; then
                        set_state 'running: proxy and WFC supervised'
                    else
                        set_state 'waiting: WFC process'
                    fi
                else
                    set_state 'waiting: UK proxy tunnel'
                fi
            fi
        fi
        sleep 10
    done
}

status() {
    if [ -e "$BASE/disabled" ]; then echo 'autostart disabled'; else echo 'autostart enabled'; fi
    for ROLE in supervisor proxy wfc; do
        if owned "$ROLE"; then echo "$ROLE running pid=$OPID"; else echo "$ROLE stopped"; fi
    done
    [ ! -r "$RUN/state" ] || cat "$RUN/state"
}

start() {
    [ ! -e "$BASE/disabled" ] || { echo 'autostart disabled'; return 1; }
    if ! owned supervisor; then
        "$BB" nohup "$BB" setsid "$BB" flock -n "$RUN/service.lock" \
            /system/bin/sh "$SELF" supervise \
            >>"$LOG/supervisor.log" 2>&1 </dev/null &
        # The caller may be an ADB shell. Let nohup/setsid finish before exit.
        sleep 3
    fi
    status
    owned supervisor
}

case "$1" in
    start) start;;
    supervise) supervise;;
    status) status;;
    enable) rm -f "$BASE/disabled"; start;;
    disable)
        touch "$BASE/disabled"
        if owned supervisor; then kill -TERM "$OPID"; fi
        echo 'autostart disabled; owned runtime is stopping';;
    *) echo 'Usage: service-control.sh start|status|enable|disable'; exit 2;;
esac
