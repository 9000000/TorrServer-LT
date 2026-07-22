## TorrServer-LT Docker

Automatic launcher for [TorrServer-LT](https://github.com/9000000/TorrServer-LT).
When the container starts, it downloads the latest release binary matching the architecture from GitHub Releases.

### Supported architectures
* TorrServer-LT-linux-386
* TorrServer-LT-linux-amd64
* TorrServer-LT-linux-arm5
* TorrServer-LT-linux-arm64
* TorrServer-LT-linux-arm7

### Docker Run Example
```bash
docker run -d \
  --name torrserver-lt \
  -p 8090:8090 \
  -p 32000:32000 \
  -e TS_PORT=8090 \
  -e TS_PATH="/opt/torrserver/config" \
  -e TS_TORRENTSDIR="/opt/torrserver/torrents" \
  -v ./config:/opt/torrserver/config \
  -v ./torrents:/opt/torrserver/torrents \
  torrserver-lt:latest
```

### Docker Compose Example
```yaml
version: '3.8'
services:
  torrserver-lt:
    container_name: torrserver-lt
    build:
      context: ./docker
      dockerfile: Dockerfile
    restart: unless-stopped
    ports:
      - "8090:8090"
      - "32000:32000"
    environment:
      - TS_PORT=8090
      - TS_PATH=/opt/torrserver/config
      - TS_TORRENTSDIR=/opt/torrserver/torrents
    volumes:
      - ./config:/opt/torrserver/config
      - ./torrents:/opt/torrserver/torrents
```
