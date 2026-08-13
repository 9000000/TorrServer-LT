# syntax=docker/dockerfile:1.7
#
# TorrServer-LT builder: produces a fully-static linux binary with libtorrent
# (arvidn) linked into the Go binary via cgo.
#
# Stages:
#   lt-build  — compiles libtorrent-rasterbar 2.x as a static archive
#   go-build  — compiles the Go binary with cgo, statically linking libtorrent
#               and all C/C++ deps (boost, openssl, zlib, libstdc++, musl)
#   final     — minimal scratch image with just the binary
#
# Stage 1 milestone: only proves the toolchain works end-to-end (lt.Version()).
# Real wiring of session/torrent/storage lands in later milestones.

# Global build arguments (can be overridden via Docker Compose `build.args`)
ARG LT_TAG=v2.1.1
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.20
ARG TS_VERSION=MatriX.142.LT-1.1.8
ARG TS_PORT=8090

############################
# Stage 1: build libtorrent
############################
FROM alpine:${ALPINE_VERSION} AS lt-build
ARG LT_TAG

RUN apk add --no-cache \
        build-base cmake git linux-headers \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

WORKDIR /src
RUN git clone --branch ${LT_TAG} --depth 1 --recurse-submodules \
        https://github.com/arvidn/libtorrent.git

WORKDIR /src/libtorrent/build
# deprecated-functions=ON (ABI v2): the shim's torrent_info-from-buffer path
# uses ctors that libtorrent 2.1 marks deprecated under ABI < 4; OFF (= newest
# ABI) would remove them. webtorrent=OFF avoids unnecessary libdatachannel
# dependencies and fixes static linking undefined reference errors (rtc::*).
RUN cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_SHARED_LIBS=OFF \
        -Dstatic_runtime=ON \
        -Ddeprecated-functions=ON \
        -Dwebtorrent=ON \
        -Dbuild_examples=OFF \
        -Dbuild_tests=OFF \
        -Dpython-bindings=OFF \
        -DCMAKE_INSTALL_PREFIX=/opt/lt \
 && cmake --build . -j"$(nproc)" \
 && cmake --install . \
 && find . -name "*.a" -exec cp {} /opt/lt/lib/ \; \
 && for f in /opt/lt/lib/*-static.a; do [ -f "$f" ] && cp "$f" "${f%-static.a}.a" || true; done \
 && sed -i 's/-lssl -lcrypto/-ldatachannel -ljuice -lusrsctp -lssl -lcrypto/g' /opt/lt/lib/pkgconfig/libtorrent-rasterbar.pc

############################
# Stage 2: build TorrServer-LT
############################
FROM golang:${GO_VERSION}-alpine AS go-build
ARG TS_VERSION

RUN apk add --no-cache \
        build-base musl-dev pkgconfig git upx \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

# libtorrent artifacts from stage 1
COPY --from=lt-build /opt/lt /opt/lt
ENV PKG_CONFIG_PATH=/opt/lt/lib/pkgconfig

WORKDIR /src
COPY . .

WORKDIR /src/server

# CGO_ENABLED=1 + fully-static via -extldflags '-static'.
# pkg-config in lt.go resolves CXXFLAGS/LDFLAGS for libtorrent-rasterbar.
ENV CGO_ENABLED=1

# Gate the static binary build on the lt + torrstor test suites so a
# broken shim or piece-cache never ships. libtorrent is static
# (.a only under /opt/lt/lib) so the test binaries are fully
# self-contained.
RUN go test -count=1 -timeout 180s ./lt/ ./torr/ ./torr/storage/torrstor/ ./dlna/

RUN go build \
      -tags 'osusergo netgo' \
      -ldflags "-s -w -X server/version.Version=${TS_VERSION} -linkmode external -extldflags '-static'" \
      -o /out/TorrServer-LT \
      ./cmd \
 && upx --best --lzma /out/TorrServer-LT

############################
# Stage 3: final
############################
FROM alpine:${ALPINE_VERSION} AS final
ARG TS_PORT

LABEL maintainer="9000000"
LABEL description="TorrServer-LT fully-static lightweight image"

ENV TS_CONF_PATH="/opt/ts/config" \
    TS_LOG_PATH="" \
    TS_TORR_DIR="/opt/ts/torrents" \
    TS_PORT=${TS_PORT} \
    GODEBUG=madvdontneed=1

RUN apk add --no-cache ca-certificates libstdc++ \
 && rm -rf /var/cache/apk/* /usr/share/locale /usr/share/man /usr/share/doc /usr/share/gtk-doc

COPY --link --from=go-build /out/TorrServer-LT /usr/local/bin/TorrServer-LT
COPY --link docker-entrypoint.sh /docker-entrypoint.sh

RUN ln -s /usr/local/bin/TorrServer-LT /usr/local/bin/torrserver \
 && sed -i 's/\r$//' /docker-entrypoint.sh \
 && chmod +x /docker-entrypoint.sh /usr/local/bin/TorrServer-LT

EXPOSE ${TS_PORT}
VOLUME ["/opt/ts/config", "/opt/ts/torrents"]

ENTRYPOINT ["/docker-entrypoint.sh"]