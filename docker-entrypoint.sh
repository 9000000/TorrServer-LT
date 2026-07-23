#!/bin/sh

TS_CONF_PATH="${TS_CONF_PATH:-/opt/ts/config}"
TS_LOG_PATH="${TS_LOG_PATH:-/opt/ts/log}"
TS_TORR_DIR="${TS_TORR_DIR:-/opt/ts/torrents}"
TS_PORT="${TS_PORT:-8090}"

FLAGS=""
[ -n "$TS_CONF_PATH" ] && FLAGS="${FLAGS} --path ${TS_CONF_PATH}"
[ -n "$TS_LOG_PATH" ] && FLAGS="${FLAGS} --logpath ${TS_LOG_PATH}"
[ -n "$TS_PORT" ] && FLAGS="${FLAGS} --port ${TS_PORT}"
[ -n "$TS_TORR_DIR" ] && FLAGS="${FLAGS} --torrentsdir ${TS_TORR_DIR}"
if [ -n "$TS_IP" ]; then FLAGS="${FLAGS} -i ${TS_IP}"; fi
if [ "$TS_HTTPAUTH" = "1" ] || [ "$TS_HTTPAUTH" = "true" ]; then FLAGS="${FLAGS} --httpauth"; fi
if [ "$TS_RDB" = "1" ] || [ "$TS_RDB" = "true" ]; then FLAGS="${FLAGS} --rdb"; fi
if [ "$TS_DONTKILL" = "1" ] || [ "$TS_DONTKILL" = "true" ]; then FLAGS="${FLAGS} --dontkill"; fi
if [ "$TS_EN_SSL" = "1" ] || [ "$TS_EN_SSL" = "true" ]; then FLAGS="${FLAGS} --ssl"; fi
if [ -n "$TS_SSL_PORT" ]; then FLAGS="${FLAGS} --sslport ${TS_SSL_PORT}"; fi
if [ -n "$TS_TORRENTADDR" ]; then FLAGS="${FLAGS} --torrentaddr ${TS_TORRENTADDR}"; fi
if [ -n "$TS_PUBIPV4" ]; then FLAGS="${FLAGS} --pubipv4 ${TS_PUBIPV4}"; fi
if [ -n "$TS_PUBIPV6" ]; then FLAGS="${FLAGS} --pubipv6 ${TS_PUBIPV6}"; fi
if [ "$TS_SEARCHWA" = "1" ] || [ "$TS_SEARCHWA" = "true" ]; then FLAGS="${FLAGS} --searchwa"; fi
if [ -n "$TS_PROXYURL" ]; then FLAGS="${FLAGS} --proxyurl ${TS_PROXYURL}"; fi
if [ -n "$TS_PROXYMODE" ]; then FLAGS="${FLAGS} --proxymode ${TS_PROXYMODE}"; fi

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

