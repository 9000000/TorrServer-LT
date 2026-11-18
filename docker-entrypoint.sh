#!/bin/sh
set -eu

TS_CONF_PATH="${TS_CONF_PATH:-/opt/ts/config}"
TS_LOG_PATH="${TS_LOG_PATH:-}"
TS_TORR_DIR="${TS_TORR_DIR:-/opt/ts/torrents}"
TS_PORT="${TS_PORT:-8090}"

set --
LOG_FLAGS=""

add_flag() {
  set -- "$@" "$1"
  LOG_FLAGS="${LOG_FLAGS} $1"
}

[ -n "$TS_CONF_PATH" ] && { set -- "$@" --path "$TS_CONF_PATH"; LOG_FLAGS="${LOG_FLAGS} --path ${TS_CONF_PATH}"; }
[ -n "$TS_LOG_PATH" ] && { set -- "$@" --logpath "$TS_LOG_PATH"; LOG_FLAGS="${LOG_FLAGS} --logpath ${TS_LOG_PATH}"; }
[ -n "$TS_PORT" ] && { set -- "$@" --port "$TS_PORT"; LOG_FLAGS="${LOG_FLAGS} --port ${TS_PORT}"; }
[ -n "$TS_TORR_DIR" ] && { set -- "$@" --torrentsdir "$TS_TORR_DIR"; LOG_FLAGS="${LOG_FLAGS} --torrentsdir ${TS_TORR_DIR}"; }
[ -n "${TS_IP:-}" ] && { set -- "$@" --ip "$TS_IP"; LOG_FLAGS="${LOG_FLAGS} --ip ${TS_IP}"; }

if [ "${TS_HTTPAUTH:-}" = "1" ] || [ "${TS_HTTPAUTH:-}" = "true" ]; then set -- "$@" --httpauth; LOG_FLAGS="${LOG_FLAGS} --httpauth"; fi
if [ "${TS_RDB:-}" = "1" ] || [ "${TS_RDB:-}" = "true" ]; then set -- "$@" --rdb; LOG_FLAGS="${LOG_FLAGS} --rdb"; fi
if [ "${TS_DONTKILL:-}" = "1" ] || [ "${TS_DONTKILL:-}" = "true" ]; then set -- "$@" --dontkill; LOG_FLAGS="${LOG_FLAGS} --dontkill"; fi
# TS_EN_SSL is the old name of TS_SSL_ENABLE and keeps working.
if [ "${TS_SSL_ENABLE:-}" = "1" ] || [ "${TS_SSL_ENABLE:-}" = "true" ] || [ "${TS_EN_SSL:-}" = "1" ] || [ "${TS_EN_SSL:-}" = "true" ]; then set -- "$@" --ssl; LOG_FLAGS="${LOG_FLAGS} --ssl"; fi
if [ "${TS_SEARCH_WA_ENABLE:-}" = "1" ] || [ "${TS_SEARCH_WA_ENABLE:-}" = "true" ] || [ "${TS_SEARCHWA:-}" = "1" ] || [ "${TS_SEARCHWA:-}" = "true" ]; then set -- "$@" --searchwa; LOG_FLAGS="${LOG_FLAGS} --searchwa"; fi
if [ "${TS_STREAM_WA_ENABLE:-}" = "1" ] || [ "${TS_STREAM_WA_ENABLE:-}" = "true" ]; then set -- "$@" --streamwa; LOG_FLAGS="${LOG_FLAGS} --streamwa"; fi
if [ "${TS_WEBDAV_ENABLE:-}" = "1" ] || [ "${TS_WEBDAV_ENABLE:-}" = "true" ]; then set -- "$@" --webdav; LOG_FLAGS="${LOG_FLAGS} --webdav"; fi
if [ "${TS_FORCE_HTTPS_ENABLE:-}" = "1" ] || [ "${TS_FORCE_HTTPS_ENABLE:-}" = "true" ]; then set -- "$@" --force-https; LOG_FLAGS="${LOG_FLAGS} --force-https"; fi
if [ "${TS_HTTP_MEDIA_ENABLE:-}" = "1" ] || [ "${TS_HTTP_MEDIA_ENABLE:-}" = "true" ]; then set -- "$@" --http-media; LOG_FLAGS="${LOG_FLAGS} --http-media"; fi
if [ "${TS_HTTPS_ONLY_ENABLE:-}" = "1" ] || [ "${TS_HTTPS_ONLY_ENABLE:-}" = "true" ]; then set -- "$@" --https-only; LOG_FLAGS="${LOG_FLAGS} --https-only"; fi

[ -n "${TS_SSL_PORT:-}" ] && { set -- "$@" --sslport "$TS_SSL_PORT"; LOG_FLAGS="${LOG_FLAGS} --sslport ${TS_SSL_PORT}"; }
[ -n "${TS_SSL_CERT_PATH:-}" ] && { set -- "$@" --sslcert "$TS_SSL_CERT_PATH"; LOG_FLAGS="${LOG_FLAGS} --sslcert ${TS_SSL_CERT_PATH}"; }
[ -n "${TS_SSL_KEY_PATH:-}" ] && { set -- "$@" --sslkey "$TS_SSL_KEY_PATH"; LOG_FLAGS="${LOG_FLAGS} --sslkey ${TS_SSL_KEY_PATH}"; }
[ -n "${TS_PROXYURL:-}" ] && { set -- "$@" --proxyurl "$TS_PROXYURL"; LOG_FLAGS="${LOG_FLAGS} --proxyurl [redacted]"; }
[ -n "${TS_PROXYMODE:-}" ] && { set -- "$@" --proxymode "$TS_PROXYMODE"; LOG_FLAGS="${LOG_FLAGS} --proxymode ${TS_PROXYMODE}"; }
[ -n "${TS_WEB_LOG_PATH:-}" ] && { set -- "$@" --weblogpath "$TS_WEB_LOG_PATH"; LOG_FLAGS="${LOG_FLAGS} --weblogpath ${TS_WEB_LOG_PATH}"; }
[ -n "${TS_TORRENTADDR:-}" ] && { set -- "$@" --torrentaddr "$TS_TORRENTADDR"; LOG_FLAGS="${LOG_FLAGS} --torrentaddr ${TS_TORRENTADDR}"; }
[ -n "${TS_TORR_ADDR:-}" ] && { set -- "$@" --torrentaddr "$TS_TORR_ADDR"; LOG_FLAGS="${LOG_FLAGS} --torrentaddr ${TS_TORR_ADDR}"; }
[ -n "${TS_PUBIPV4:-}" ] && { set -- "$@" --pubipv4 "$TS_PUBIPV4"; LOG_FLAGS="${LOG_FLAGS} --pubipv4 ${TS_PUBIPV4}"; }
[ -n "${TS_PUBLIC_IPV4_ADDR:-}" ] && { set -- "$@" --pubipv4 "$TS_PUBLIC_IPV4_ADDR"; LOG_FLAGS="${LOG_FLAGS} --pubipv4 ${TS_PUBLIC_IPV4_ADDR}"; }
[ -n "${TS_PUBIPV6:-}" ] && { set -- "$@" --pubipv6 "$TS_PUBIPV6"; LOG_FLAGS="${LOG_FLAGS} --pubipv6 ${TS_PUBIPV6}"; }
[ -n "${TS_PUBLIC_IPV6_ADDR:-}" ] && { set -- "$@" --pubipv6 "$TS_PUBLIC_IPV6_ADDR"; LOG_FLAGS="${LOG_FLAGS} --pubipv6 ${TS_PUBLIC_IPV6_ADDR}"; }
[ -n "${TS_MAX_SIZE:-}" ] && { set -- "$@" --maxsize "$TS_MAX_SIZE"; LOG_FLAGS="${LOG_FLAGS} --maxsize ${TS_MAX_SIZE}"; }
[ -n "${TS_TELEGRAM_TOKEN:-}" ] && { set -- "$@" --tgtoken "$TS_TELEGRAM_TOKEN"; LOG_FLAGS="${LOG_FLAGS} --tgtoken [redacted]"; }
[ -n "${TS_FUSE_PATH:-}" ] && { set -- "$@" --fusepath "$TS_FUSE_PATH"; LOG_FLAGS="${LOG_FLAGS} --fusepath ${TS_FUSE_PATH}"; }

if [ -n "$TS_CONF_PATH" ] && [ ! -d "$TS_CONF_PATH" ]; then
  mkdir -p "$TS_CONF_PATH"
fi

if [ -n "$TS_TORR_DIR" ] && [ ! -d "$TS_TORR_DIR" ]; then
  mkdir -p "$TS_TORR_DIR"
fi

if [ -n "$TS_LOG_PATH" ]; then
  LOG_DIR=$(dirname "$TS_LOG_PATH")
  [ ! -d "$LOG_DIR" ] && mkdir -p "$LOG_DIR"
fi

echo "Running with:${LOG_FLAGS}"

exec torrserver "$@"
