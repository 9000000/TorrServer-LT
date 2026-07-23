# syntax=docker/dockerfile:1
#
# TorrServer-LT static builder WITH GStreamer support (~60 MB final image size).
#
# Multi-stage build:
#   lt-build  — compiles libtorrent-rasterbar statically against boost/openssl/zlib
#   go-build  — links TorrServer statically with libtorrent and GStreamer (-tags 'osusergo netgo gst')
#   final     — minimal Alpine image with GStreamer runtime libraries
#

ARG LT_TAG=v2.0.13
ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.20

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
RUN cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_SHARED_LIBS=OFF \
        -Dstatic_runtime=ON \
        -Ddeprecated-functions=OFF \
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

RUN apk add --no-cache \
        build-base musl-dev pkgconfig git \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static

# libtorrent artifacts from stage 1
COPY --from=lt-build /opt/lt /opt/lt
ENV PKG_CONFIG_PATH=/opt/lt/lib/pkgconfig

WORKDIR /src
COPY . .

WORKDIR /src/server

ARG TS_VERSION=MatriX.142.LT-114.1

ENV CGO_ENABLED=1

RUN go test -count=1 -timeout 180s ./lt/ ./torr/ ./torr/storage/torrstor/ ./dlna/

RUN go build \
      -tags 'osusergo netgo gst' \
      -ldflags "-s -w -X server/version.Version=${TS_VERSION}" \
      -o /out/TorrServer-LT \
      ./cmd

############################
# Stage 3: final image (~60MB with GStreamer)
############################
FROM alpine:${ALPINE_VERSION} AS final

LABEL maintainer="9000000"
LABEL description="TorrServer-LT ultra-lightweight image with GStreamer HLS transcoding"

ENV TS_CONF_PATH="/opt/ts/config" \
    TS_LOG_PATH="/opt/ts/log" \
    TS_TORR_DIR="/opt/ts/torrents" \
    TS_PORT=8090 \
    GODEBUG=madvdontneed=1

# Install minimal GStreamer runtime libraries
RUN apk add --no-cache \
        ca-certificates \
        libstdc++ \
        gstreamer \
        gstreamer-tools \
        gst-plugins-base \
        gst-plugins-good

COPY --from=go-build /out/TorrServer-LT /usr/local/bin/TorrServer-LT
RUN ln -s /usr/local/bin/TorrServer-LT /usr/local/bin/torrserver

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh \
 && chmod +x /docker-entrypoint.sh /usr/local/bin/TorrServer-LT

EXPOSE 8090
VOLUME ["/opt/ts/config", "/opt/ts/torrents"]

ENTRYPOINT ["/docker-entrypoint.sh"]

