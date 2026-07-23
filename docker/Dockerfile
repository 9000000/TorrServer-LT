FROM debian:trixie-slim

LABEL maintainer="9000000"

# Install essential runtime & minimal GStreamer base (trimmed size)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libstdc++6 \
    libgstreamer1.0-0 \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-base-apps \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/torrserver

COPY start.sh /start.sh
RUN chmod +x /start.sh

EXPOSE 8090

ENTRYPOINT ["/bin/sh", "/start.sh"]
