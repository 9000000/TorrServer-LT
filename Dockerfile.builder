# syntax=docker/dockerfile:1.7
#
# TorrServer-LT builder: produces a fully-static linux binary with libtorrent
# (arvidn) linked into the Go binary via cgo.
#
# Stages:
#   lt-build  — compiles libtorrent-rasterbar 2.x as a static archive
#   go-build  — compiles the Go binary with cgo, statically linking libtorrent
#               and all C/C++ deps (boost, openssl, zlib, libstdc++, musl)
#   final     — scratch image with just the static binary + entrypoint

ARG LT_TAG=v2.1.0
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.24.1

############################
# Stage 1: build libtorrent
############################
FROM alpine:${ALPINE_VERSION} AS lt-build
ARG LT_TAG

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        build-base cmake curl linux-headers ccache \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

WORKDIR /src
RUN LT_VER=$(echo ${LT_TAG} | sed 's/^v//') \
 && curl -sL https://github.com/arvidn/libtorrent/releases/download/${LT_TAG}/libtorrent-rasterbar-${LT_VER}.tar.gz | tar -xzf - \
 && mv libtorrent-rasterbar-${LT_VER} libtorrent

WORKDIR /src/libtorrent/build
RUN --mount=type=cache,target=/root/.cache/ccache \
    cmake .. \
        -DCMAKE_BUILD_TYPE=MinSizeRel \
        -DCMAKE_CXX_COMPILER_LAUNCHER=ccache \
        -DBUILD_SHARED_LIBS=OFF \
        -Dstatic_runtime=ON \
        -Ddeprecated-functions=ON \
        -Dwebtorrent=ON \
        -Dlogging=OFF \
        -Dbuild_examples=OFF \
        -Dbuild_tests=OFF \
        -Dpython-bindings=OFF \
        -DCMAKE_INSTALL_PREFIX=/opt/lt \
 && cmake --build . -j"$(nproc)" \
 && cmake --install .

############################
# Stage 2: build TorrServer-LT
############################
FROM golang:${GO_VERSION}-alpine AS go-build

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        build-base musl-dev pkgconfig git \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static \
        upx

# libtorrent artifacts from stage 1
COPY --from=lt-build /opt/lt /opt/lt
ENV PKG_CONFIG_PATH=/opt/lt/lib/pkgconfig

# Cache go module downloads: copy go.mod/go.sum first
WORKDIR /src
COPY server/go.mod server/go.sum ./server/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd server && go mod download

COPY . .

WORKDIR /src/server

ARG TS_VERSION=MatriX.142.LT-115.1

# CGO_ENABLED=1 + fully-static via -extldflags '-static'.
# pkg-config in lt.go resolves CXXFLAGS/LDFLAGS for libtorrent-rasterbar.
ENV CGO_ENABLED=1

# Gate the static binary build on the lt + torrstor test suites so a
# broken shim or piece-cache never ships.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test -count=1 -timeout 180s ./lt/ ./torr/ ./torr/storage/torrstor/ ./dlna/

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build \
      -tags 'osusergo netgo' \
      -ldflags "-s -w -X server/version.Version=${TS_VERSION} -linkmode external -extldflags '-static'" \
      -o /out/TorrServer-LT \
      ./cmd \
 && upx --best --lzma /out/TorrServer-LT

############################
# Stage 3: final (scratch + busybox for shell entrypoint)
############################
FROM busybox:1.37-musl AS final

LABEL maintainer="9000000"
LABEL description="TorrServer-LT fully static lightweight image"

# Grab CA certs from the builder (no apk needed in scratch-like image)
COPY --link --from=go-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV TS_CONF_PATH="/opt/ts/config" \
    TS_LOG_PATH="/opt/ts/log" \
    TS_TORR_DIR="/opt/ts/torrents" \
    TS_PORT=8090 \
    GODEBUG=madvdontneed=1

COPY --link --from=go-build /out/TorrServer-LT /usr/local/bin/TorrServer-LT
RUN ln -s /usr/local/bin/TorrServer-LT /usr/local/bin/torrserver

COPY --link docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh \
 && chmod +x /docker-entrypoint.sh /usr/local/bin/TorrServer-LT

EXPOSE 8090
VOLUME ["/opt/ts/config", "/opt/ts/torrents"]

ENTRYPOINT ["/docker-entrypoint.sh"]
