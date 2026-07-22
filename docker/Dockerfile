FROM debian:trixie-slim

LABEL maintainer="9000000"

# Minimize image size: omit ffmpeg (saves ~200MB), install only essential runtime packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/torrserver

COPY start.sh /start.sh
RUN chmod +x /start.sh

EXPOSE 8090

ENTRYPOINT ["/bin/sh", "/start.sh"]
