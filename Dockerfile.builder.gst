# syntax=docker/dockerfile:1.7
#
# TorrServer-LT static builder WITH GStreamer support.
#
# Multi-stage build:
#   lt-build  — compiles libtorrent-rasterbar statically against boost/openssl/zlib
#   go-build  — links TorrServer with libtorrent and GStreamer (-tags 'osusergo netgo gst')
#   final     — minimal Alpine image with GStreamer runtime libraries
#

ARG LT_TAG=v2.0.13
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.24.1

############################
# Stage 1: build libtorrent
############################
FROM alpine:${ALPINE_VERSION} AS lt-build
ARG LT_TAG

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        build-base cmake git curl linux-headers ccache \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

# Download release tarball instead of git clone (faster, smaller, no .git)
WORKDIR /src
RUN LT_VER=$(echo ${LT_TAG} | sed 's/^v//') \
 && curl -sL https://github.com/arvidn/libtorrent/releases/download/${LT_TAG}/libtorrent-rasterbar-${LT_VER}.tar.gz | tar -xzf - \
 && mv libtorrent-rasterbar-${LT_VER} libtorrent

WORKDIR /src/libtorrent/build
RUN --mount=type=cache,target=/root/.cache/ccache \
    cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_CXX_COMPILER_LAUNCHER=ccache \
        -DBUILD_SHARED_LIBS=OFF \
        -Dstatic_runtime=ON \
        -Ddeprecated-functions=OFF \
        -Dlogging=OFF \
        -Dbuild_examples=OFF \
        -Dbuild_tests=OFF \
        -Dpython-bindings=OFF \
        -DCMAKE_INSTALL_PREFIX=/opt/lt \
 && cmake --build . -j"$(nproc)" \
 && cmake --install .

############################
# Stage 2: build TorrServer-LT with GStreamer tag
############################
FROM golang:${GO_VERSION}-alpine AS go-build

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        build-base musl-dev pkgconfig git \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

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

ARG TS_VERSION=MatriX.142.LT-114.1

ENV CGO_ENABLED=1

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test -count=1 -timeout 180s ./lt/ ./torr/ ./torr/storage/torrstor/ ./dlna/

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build \
      -tags 'osusergo netgo gst' \
      -ldflags "-s -w -X server/version.Version=${TS_VERSION}" \
      -o /out/TorrServer-LT \
      ./cmd

############################
# Stage 3: final image
############################
FROM alpine:${ALPINE_VERSION} AS final

LABEL maintainer="9000000"
LABEL description="TorrServer-LT ultra-lightweight image with GStreamer HLS transcoding"

ENV TS_CONF_PATH="/opt/ts/config" \
    TS_LOG_PATH="/opt/ts/log" \
    TS_TORR_DIR="/opt/ts/torrents" \
    TS_PORT=8090 \
    GODEBUG=madvdontneed=1

# Install minimal GStreamer runtime (no -tools, no -good unless needed)
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache \
        ca-certificates \
        libstdc++ \
        gstreamer \
        gst-plugins-base \
        gst-plugins-good \
 && rm -rf /tmp/* /var/tmp/*

COPY --link --from=go-build /out/TorrServer-LT /usr/local/bin/TorrServer-LT

COPY --link docker-entrypoint.sh /docker-entrypoint.sh
RUN ln -s /usr/local/bin/TorrServer-LT /usr/local/bin/torrserver \
 && sed -i 's/\r$//' /docker-entrypoint.sh \
 && chmod +x /docker-entrypoint.sh /usr/local/bin/TorrServer-LT

EXPOSE 8090
VOLUME ["/opt/ts/config", "/opt/ts/torrents"]

ENTRYPOINT ["/docker-entrypoint.sh"]
