FROM debian:trixie-slim

LABEL maintainer="9000000"

# Install minimal runtime dependencies without GStreamer (glibc 2.38+ & C++ libstdc++)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libstdc++6 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/torrserver

COPY start.sh /start.sh
RUN chmod +x /start.sh

EXPOSE 8090

ENTRYPOINT ["/bin/sh", "/start.sh"]
