#!/bin/sh
set -eu

case $(uname -m) in
    x86_64) architecture="amd64" ;;
    aarch64) architecture="arm64" ;;
    armv7|armv7l) architecture="armv7" ;;
    *) echo "Unsupported Arch. Can't continue."; exit 1 ;;
esac

USE_GST="${TS_GST:-${USE_GST:-}}"
if [ "$USE_GST" = "1" ] || [ "$USE_GST" = "true" ]; then
    echo "GStreamer variant enabled (TS_GST=1)."
    binName="TorrServer-LT-linux-${architecture}-gst"
else
    binName="TorrServer-LT-linux-${architecture}"
fi

mkdir -p /opt/torrserver
cd /opt/torrserver

rm -f "${binName}"*

echo "Downloading latest ${binName} from 9000000/TorrServer-LT..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${binName}.download" "https://github.com/9000000/TorrServer-LT/releases/latest/download/$binName"
else
    wget -q -O "${binName}.download" "https://github.com/9000000/TorrServer-LT/releases/latest/download/$binName"
fi

chmod +x "${binName}.download"
mv -f "${binName}.download" "$binName"

set --
LOG_FLAGS=""

RUN_PORT="${TS_PORT:-${PORT:-8090}}"
echo "Running on port: ${RUN_PORT}"
set -- "$@" --port "$RUN_PORT"
LOG_FLAGS="${LOG_FLAGS} --port ${RUN_PORT}"

CONF_PATH="${TS_CONF_PATH:-${TS_PATH:-}}"
if [ -n "$CONF_PATH" ]; then
    echo "CONF_PATH: $CONF_PATH"
    set -- "$@" --path "$CONF_PATH"
    LOG_FLAGS="${LOG_FLAGS} --path ${CONF_PATH}"
    [ ! -d "$CONF_PATH" ] && mkdir -p "$CONF_PATH"
fi

if [ -n "${TS_LOG_PATH:-}" ]; then
    echo "TS_LOG_PATH: $TS_LOG_PATH"
    set -- "$@" --logpath "$TS_LOG_PATH"
    LOG_FLAGS="${LOG_FLAGS} --logpath ${TS_LOG_PATH}"
    LOG_DIR=$(dirname "$TS_LOG_PATH")
    [ ! -d "$LOG_DIR" ] && mkdir -p "$LOG_DIR"
elif [ -n "${TS_LOGFILE:-}" ]; then
    LOG_TARGET="${TS_LOGPATHDIR:-/opt/torrserver}/${TS_LOGFILE}"
    echo "TS_LOGFILE: $LOG_TARGET"
    set -- "$@" --logpath "$LOG_TARGET"
    LOG_FLAGS="${LOG_FLAGS} --logpath ${LOG_TARGET}"
    mkdir -p "$(dirname "$LOG_TARGET")"
fi

if [ -n "${TS_WEBLOGFILE:-}" ]; then
    WEBLOG_TARGET="${TS_LOGPATHDIR:-/opt/torrserver}/${TS_WEBLOGFILE}"
    echo "TS_WEBLOGFILE: $WEBLOG_TARGET"
    set -- "$@" --weblogpath "$WEBLOG_TARGET"
    LOG_FLAGS="${LOG_FLAGS} --weblogpath ${WEBLOG_TARGET}"
    mkdir -p "$(dirname "$WEBLOG_TARGET")"
fi

{ [ "${TS_RDB:-}" = "1" ] || [ "${TS_RDB:-}" = "true" ]; } && { echo "TS_RDB: $TS_RDB"; set -- "$@" --rdb; LOG_FLAGS="${LOG_FLAGS} --rdb"; }
{ [ "${TS_HTTPAUTH:-}" = "1" ] || [ "${TS_HTTPAUTH:-}" = "true" ]; } && { echo "TS_HTTPAUTH: $TS_HTTPAUTH"; set -- "$@" --httpauth; LOG_FLAGS="${LOG_FLAGS} --httpauth"; }
{ [ "${TS_DONTKILL:-}" = "1" ] || [ "${TS_DONTKILL:-}" = "true" ]; } && { echo "TS_DONTKILL: $TS_DONTKILL"; set -- "$@" --dontkill; LOG_FLAGS="${LOG_FLAGS} --dontkill"; }

TORR_DIR="${TS_TORR_DIR:-${TS_TORRENTSDIR:-}}"
if [ -n "$TORR_DIR" ]; then
    echo "TORR_DIR: $TORR_DIR"
    set -- "$@" --torrentsdir "$TORR_DIR"
    LOG_FLAGS="${LOG_FLAGS} --torrentsdir ${TORR_DIR}"
    [ ! -d "$TORR_DIR" ] && mkdir -p "$TORR_DIR"
fi

[ -n "${TS_TORRENTADDR:-}" ] && { echo "TS_TORRENTADDR: $TS_TORRENTADDR"; set -- "$@" --torrentaddr "$TS_TORRENTADDR"; LOG_FLAGS="${LOG_FLAGS} --torrentaddr ${TS_TORRENTADDR}"; }
[ -n "${TS_PUBIPV4:-}" ] && { echo "TS_PUBIPV4: $TS_PUBIPV4"; set -- "$@" --pubipv4 "$TS_PUBIPV4"; LOG_FLAGS="${LOG_FLAGS} --pubipv4 ${TS_PUBIPV4}"; }
[ -n "${TS_PUBIPV6:-}" ] && { echo "TS_PUBIPV6: $TS_PUBIPV6"; set -- "$@" --pubipv6 "$TS_PUBIPV6"; LOG_FLAGS="${LOG_FLAGS} --pubipv6 ${TS_PUBIPV6}"; }
{ [ "${TS_SEARCHWA:-}" = "1" ] || [ "${TS_SEARCHWA:-}" = "true" ]; } && { echo "TS_SEARCHWA: $TS_SEARCHWA"; set -- "$@" --searchwa; LOG_FLAGS="${LOG_FLAGS} --searchwa"; }
[ -n "${TS_PROXYURL:-}" ] && { echo "TS_PROXYURL: [redacted]"; set -- "$@" --proxyurl "$TS_PROXYURL"; LOG_FLAGS="${LOG_FLAGS} --proxyurl [redacted]"; }
[ -n "${TS_PROXYMODE:-}" ] && { echo "TS_PROXYMODE: $TS_PROXYMODE"; set -- "$@" --proxymode "$TS_PROXYMODE"; LOG_FLAGS="${LOG_FLAGS} --proxymode ${TS_PROXYMODE}"; }

echo "Running with:${LOG_FLAGS}"
export GODEBUG=madvdontneed=1

exec "/opt/torrserver/${binName}" "$@"
