#!/bin/sh

case $(uname -m) in
    i386|i686) architecture="386" ;;
    x86_64) architecture="amd64" ;;
    aarch64) architecture="arm64" ;;
    armv7|armv7l) architecture="arm7" ;;
    armv6|armv6l) architecture="arm5" ;;
    *) echo "Unsupported Arch. Can't continue."; exit 1 ;;
esac

binName="TorrServer-LT-linux-${architecture}"

mkdir -p /opt/torrserver
cd /opt/torrserver

rm -f ${binName}*

echo "Downloading latest ${binName} from 9000000/TorrServer-LT..."
if command -v curl >/dev/null 2>&1; then
    curl -sL -o "$binName" "https://github.com/9000000/TorrServer-LT/releases/latest/download/$binName"
else
    wget -O "$binName" "https://github.com/9000000/TorrServer-LT/releases/latest/download/$binName"
fi

chmod +x "$binName"

FLAGS=""

# sets start flags
RUN_PORT="${TS_PORT:-$PORT}"
RUN_PORT="${RUN_PORT:-8090}"
echo "Running on port: ${RUN_PORT}"
FLAGS="${FLAGS} --port ${RUN_PORT}"

CONF_PATH="${TS_CONF_PATH:-$TS_PATH}"
if [ -n "$CONF_PATH" ]; then
    echo "CONF_PATH: $CONF_PATH"
    FLAGS="${FLAGS} --path ${CONF_PATH}"
    [ ! -d "$CONF_PATH" ] && mkdir -p "$CONF_PATH"
fi

if [ -n "$TS_LOG_PATH" ]; then
    echo "TS_LOG_PATH: $TS_LOG_PATH"
    FLAGS="${FLAGS} --logpath ${TS_LOG_PATH}"
    LOG_DIR=$(dirname "$TS_LOG_PATH")
    [ ! -d "$LOG_DIR" ] && mkdir -p "$LOG_DIR"
elif [ -n "$TS_LOGFILE" ]; then
    echo "TS_LOGFILE: $TS_LOGPATHDIR/$TS_LOGFILE"
    FLAGS="${FLAGS} --logpath $TS_LOGPATHDIR/${TS_LOGFILE}"
    [ ! -d "$TS_LOGPATHDIR" ] && mkdir -p "$TS_LOGPATHDIR"
fi

[ -n "$TS_WEBLOGFILE" ] && echo "TS_WEBLOGFILE: $TS_LOGPATHDIR/$TS_WEBLOGFILE" && FLAGS="${FLAGS} --weblogpath $TS_LOGPATHDIR/${TS_WEBLOGFILE}"
{ [ "$TS_RDB" = "1" ] || [ "$TS_RDB" = "true" ]; } && echo "TS_RDB: $TS_RDB" && FLAGS="${FLAGS} --rdb"
{ [ "$TS_HTTPAUTH" = "1" ] || [ "$TS_HTTPAUTH" = "true" ]; } && echo "TS_HTTPAUTH: $TS_HTTPAUTH" && FLAGS="${FLAGS} --httpauth"
{ [ "$TS_DONTKILL" = "1" ] || [ "$TS_DONTKILL" = "true" ]; } && echo "TS_DONTKILL: $TS_DONTKILL" && FLAGS="${FLAGS} --dontkill"

TORR_DIR="${TS_TORR_DIR:-$TS_TORRENTSDIR}"
if [ -n "$TORR_DIR" ]; then
    echo "TORR_DIR: $TORR_DIR"
    FLAGS="${FLAGS} --torrentsdir ${TORR_DIR}"
    [ ! -d "$TORR_DIR" ] && mkdir -p "$TORR_DIR"
fi

[ -n "$TS_TORRENTADDR" ] && echo "TS_TORRENTADDR: $TS_TORRENTADDR" && FLAGS="${FLAGS} --torrentaddr ${TS_TORRENTADDR}"
[ -n "$TS_PUBIPV4" ] && echo "TS_PUBIPV4: $TS_PUBIPV4" && FLAGS="${FLAGS} --pubipv4 ${TS_PUBIPV4}"
[ -n "$TS_PUBIPV6" ] && echo "TS_PUBIPV6: $TS_PUBIPV6" && FLAGS="${FLAGS} --pubipv6 ${TS_PUBIPV6}"
{ [ "$TS_SEARCHWA" = "1" ] || [ "$TS_SEARCHWA" = "true" ]; } && echo "TS_SEARCHWA: $TS_SEARCHWA" && FLAGS="${FLAGS} --searchwa"
[ -n "$TS_PROXYURL" ] && echo "TS_PROXYURL: $TS_PROXYURL" && FLAGS="${FLAGS} --proxyurl ${TS_PROXYURL}"
[ -n "$TS_PROXYMODE" ] && echo "TS_PROXYMODE: $TS_PROXYMODE" && FLAGS="${FLAGS} --proxymode ${TS_PROXYMODE}"

echo "Running with: ${FLAGS}"
export GODEBUG=madvdontneed=1

exec /opt/torrserver/${binName} ${FLAGS}
