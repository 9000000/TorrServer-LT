#!/bin/sh

TS_CONF_PATH="${TS_CONF_PATH:-/opt/ts/config}"
TS_LOG_PATH="${TS_LOG_PATH:-}"
TS_TORR_DIR="${TS_TORR_DIR:-/opt/ts/torrents}"
TS_PORT="${TS_PORT:-8090}"

FLAGS=""
[ -n "$TS_CONF_PATH" ] && FLAGS="${FLAGS} --path ${TS_CONF_PATH}"
[ -n "$TS_LOG_PATH" ] && FLAGS="${FLAGS} --logpath ${TS_LOG_PATH}"
[ -n "$TS_PORT" ] && FLAGS="${FLAGS} --port ${TS_PORT}"
[ -n "$TS_TORR_DIR" ] && FLAGS="${FLAGS} --torrentsdir ${TS_TORR_DIR}"
if [ -n "$TS_IP" ]; then FLAGS="${FLAGS} --ip ${TS_IP}"; fi
if [ "$TS_HTTPAUTH" = "1" ] || [ "$TS_HTTPAUTH" = "true" ]; then FLAGS="${FLAGS} --httpauth"; fi
if [ "$TS_RDB" = "1" ] || [ "$TS_RDB" = "true" ]; then FLAGS="${FLAGS} --rdb"; fi
if [ "$TS_DONTKILL" = "1" ] || [ "$TS_DONTKILL" = "true" ]; then FLAGS="${FLAGS} --dontkill"; fi
# TS_EN_SSL is the old name of TS_SSL_ENABLE and keeps working.
if [ "$TS_SSL_ENABLE" = "1" ] || [ "$TS_SSL_ENABLE" = "true" ] || [ "$TS_EN_SSL" = "1" ] || [ "$TS_EN_SSL" = "true" ]; then FLAGS="${FLAGS} --ssl"; fi
if [ "$TS_SEARCH_WA_ENABLE" = "1" ] || [ "$TS_SEARCH_WA_ENABLE" = "true" ] || [ "$TS_SEARCHWA" = "1" ] || [ "$TS_SEARCHWA" = "true" ]; then FLAGS="${FLAGS} --searchwa"; fi
if [ "$TS_STREAM_WA_ENABLE" = "1" ] || [ "$TS_STREAM_WA_ENABLE" = "true" ]; then FLAGS="${FLAGS} --streamwa"; fi
if [ "$TS_WEBDAV_ENABLE" = "1" ] || [ "$TS_WEBDAV_ENABLE" = "true" ]; then FLAGS="${FLAGS} --webdav"; fi
if [ "$TS_FORCE_HTTPS_ENABLE" = "1" ] || [ "$TS_FORCE_HTTPS_ENABLE" = "true" ]; then FLAGS="${FLAGS} --force-https"; fi
if [ -n "$TS_SSL_PORT" ]; then FLAGS="${FLAGS} --sslport ${TS_SSL_PORT}"; fi
if [ -n "$TS_SSL_CERT_PATH" ]; then FLAGS="${FLAGS} --sslcert ${TS_SSL_CERT_PATH}"; fi
if [ -n "$TS_SSL_KEY_PATH" ]; then FLAGS="${FLAGS} --sslkey ${TS_SSL_KEY_PATH}"; fi
if [ -n "$TS_PROXYURL" ]; then FLAGS="${FLAGS} --proxyurl ${TS_PROXYURL}"; fi
if [ -n "$TS_PROXYMODE" ]; then FLAGS="${FLAGS} --proxymode ${TS_PROXYMODE}"; fi
if [ -n "$TS_WEB_LOG_PATH" ]; then FLAGS="${FLAGS} --weblogpath ${TS_WEB_LOG_PATH}"; fi
if [ -n "$TS_TORRENTADDR" ]; then FLAGS="${FLAGS} --torrentaddr ${TS_TORRENTADDR}"; fi
if [ -n "$TS_TORR_ADDR" ]; then FLAGS="${FLAGS} --torrentaddr ${TS_TORR_ADDR}"; fi
if [ -n "$TS_PUBIPV4" ]; then FLAGS="${FLAGS} --pubipv4 ${TS_PUBIPV4}"; fi
if [ -n "$TS_PUBLIC_IPV4_ADDR" ]; then FLAGS="${FLAGS} --pubipv4 ${TS_PUBLIC_IPV4_ADDR}"; fi
if [ -n "$TS_PUBIPV6" ]; then FLAGS="${FLAGS} --pubipv6 ${TS_PUBIPV6}"; fi
if [ -n "$TS_PUBLIC_IPV6_ADDR" ]; then FLAGS="${FLAGS} --pubipv6 ${TS_PUBLIC_IPV6_ADDR}"; fi
if [ -n "$TS_MAX_SIZE" ]; then FLAGS="${FLAGS} --maxsize ${TS_MAX_SIZE}"; fi
if [ -n "$TS_TELEGRAM_TOKEN" ]; then FLAGS="${FLAGS} --tgtoken ${TS_TELEGRAM_TOKEN}"; fi
if [ -n "$TS_FUSE_PATH" ]; then FLAGS="${FLAGS} --fusepath ${TS_FUSE_PATH}"; fi

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

echo "Running with: ${FLAGS}"

exec torrserver $FLAGS

