# syntax=docker/dockerfile:1.7
#
# TorrServer-LT builder WITH GStreamer (HLS transcoding) support:
#   - Stage 1: lt-build compiles libtorrent-rasterbar statically against boost/openssl/zlib
#   - Stage 2: go-build compiles the Go binary with cgo + '-tags gst'
#   - Stage 3: final image includes GStreamer runtime libraries & plugins
#

# Global build arguments (can be overridden via Docker Compose `build.args`)
ARG LT_TAG=v2.1.0
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.20
ARG TS_VERSION=MatriX.142.LT-116.1
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
# Stage 2: build TorrServer-LT with GStreamer tag (-tags gst)
############################
FROM golang:${GO_VERSION}-alpine AS go-build
ARG TS_VERSION

RUN apk add --no-cache \
        build-base musl-dev pkgconfig git \
        boost-dev boost-static \
        openssl-dev openssl-libs-static \
        zlib-dev zlib-static \
        gstreamer-dev gst-plugins-base-dev

# libtorrent artifacts from stage 1
COPY --from=lt-build /opt/lt /opt/lt
ENV PKG_CONFIG_PATH=/opt/lt/lib/pkgconfig

WORKDIR /src
COPY . .

WORKDIR /src/server

ENV CGO_ENABLED=1

# Gate the build on test suite execution (including ./gstreamer/)
RUN go test -tags gst -count=1 -timeout 180s ./lt/ ./torr/ ./torr/storage/torrstor/ ./dlna/ ./gstreamer/

# Build Go binary with '-tags gst' for GStreamer support
RUN go build \
      -tags 'osusergo netgo gst' \
      -ldflags "-s -w -X server/version.Version=${TS_VERSION} -linkmode external -extldflags '-static'" \
      -o /out/TorrServer-LT \
      ./cmd

############################
# Stage 3: final image with GStreamer runtime
############################
FROM alpine:${ALPINE_VERSION} AS final
ARG TS_PORT

LABEL maintainer="9000000"
LABEL description="TorrServer-LT with GStreamer HLS transcoding support"

ENV TS_CONF_PATH="/opt/ts/config" \
    TS_LOG_PATH="/opt/ts/log" \
    TS_TORR_DIR="/opt/ts/torrents" \
    TS_PORT=${TS_PORT} \
    GODEBUG=madvdontneed=1

# Install GStreamer runtime libraries and codec plugins
RUN apk add --no-cache \
        ca-certificates \
        libstdc++ \
        gstreamer \
        gst-plugins-base \
        gst-plugins-good \
        gst-plugins-bad \
        gst-plugins-ugly \
        gst-libav

COPY --link --from=go-build /out/TorrServer-LT /usr/local/bin/TorrServer-LT
COPY --link docker-entrypoint.sh /docker-entrypoint.sh

RUN ln -s /usr/local/bin/TorrServer-LT /usr/local/bin/torrserver \
 && sed -i 's/\r$//' /docker-entrypoint.sh \
 && chmod +x /docker-entrypoint.sh /usr/local/bin/TorrServer-LT

EXPOSE ${TS_PORT}
VOLUME ["/opt/ts/config", "/opt/ts/torrents"]

ENTRYPOINT ["/docker-entrypoint.sh"]
