# TorrServer-LT

> Bản fork từ [YouROK/TorrServer](https://github.com/YouROK/TorrServer) với lõi BitTorrent được thay thế bằng [libtorrent (arvidn)](https://www.libtorrent.org/).
>
> **Trạng thái:** Tương đương tính năng (feature parity) với bản upstream **MatriX.142.2** (bao gồm cả chuyển mã GStreamer HLS) trên engine libtorrent — khả năng phát trực tuyến (streaming), tải trước (preload), và tua (seeking - kể cả tua lại các vùng đã bị xoá khỏi bộ nhớ đệm) đã được kiểm chứng trên torrent thực tế. HTTP API, cơ sở dữ liệu trên đĩa (`config.db`, JSON, `accs.db`, viewed) và định dạng token `torrs://` hoàn toàn tương thích với upstream. Cấu trúc bộ nhớ đệm tại `TorrentsSavePath/<hash>/<pieceID>` được giữ nguyên.
>
> **Nền tảng hỗ trợ:**
> - Linux: `amd64`, `arm64`, `armv7`
> - Windows: `amd64` (môi trường runtime MSYS2 / MinGW64)
> - macOS: `amd64` (Intel), `arm64` (Apple Silicon)
> - Android: `arm64-v8a`, `armeabi-v7a` (Termux hoặc shell tương tự)
>
> Các tệp thực thi (binary) trên mỗi nền tảng được liên kết tĩnh (hoặc gần như tĩnh) với libtorrent + Boost; xem quy trình CI theo từng nền tảng trong `.github/workflows/` để biết chính xác bộ công cụ (toolchain) biên dịch.

## Giới thiệu

TorrServer-LT là ứng dụng cho phép người dùng xem nội dung torrent trực tuyến mà không cần tải tệp về máy trước.
Các chức năng cốt lõi bao gồm lưu bộ nhớ đệm (caching) torrent và truyền dữ liệu qua giao thức HTTP,
cho phép điều chỉnh dung lượng bộ nhớ đệm theo thông số hệ thống và tốc độ kết nối internet của người dùng.

Điểm khác biệt so với bản upstream nằm ở engine torrent bên dưới: `arvidn/libtorrent` (C++) được kết nối qua một lớp đệm (shim) CGo mỏng, thay thế cho `anacrolix/torrent` (Go). Điều này giúp cải thiện hành vi giao thức peer, hiệu năng băng thông trong điều kiện thực tế, cũng như mang lại các tính năng mà engine thuần Go chưa có.

## Tài liệu AI

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/YouROK/TorrServer)

## Tính năng

- Lưu bộ nhớ đệm (Caching)
- Phát trực tuyến (Streaming)
- Máy chủ Cục bộ và Từ xa (Local và Remote Server)
- Xem torrent trên nhiều thiết bị khác nhau
- Tích hợp với các ứng dụng khác thông qua API
- Tìm kiếm Torznab (Jackett, Prowlarr, và các trình quản lý indexer tương tự)
- Giao diện web hiện đại, tương thích đa trình duyệt
- Máy chủ DLNA tùy chọn (hỗ trợ phân loại thư mục)
- Chuyển mã GStreamer HLS tùy chọn (dành cho bản build `-gst`; chuyển đổi âm thanh AC3/EAC3/DTS → AAC cho các trình phát không hỗ trợ các bộ giải mã này)

## Bắt đầu

### Cài đặt

Tải xuống ứng dụng cho nền tảng tương ứng tại trang [releases](https://github.com/9000000/TorrServer-LT/releases). Sau khi cài đặt, mở liên kết <http://127.0.0.1:8090> trên trình duyệt.

Mỗi phiên bản phát hành cung cấp 2 biến thể cho mỗi nền tảng máy tính:

- `TorrServer-LT-<platform>` — bản build cơ bản;
- `TorrServer-LT-<platform>-gst` — bản build tích hợp sẵn **chuyển mã GStreamer HLS** (linux amd64/arm64, windows amd64, macOS amd64/arm64). Phiên bản này sẽ nạp động các thư viện GStreamer của hệ thống khi chạy — hãy cài đặt [GStreamer](https://gstreamer.freedesktop.org/download/) **1.22+** (bao gồm các bộ plugin base/good/bad) để sử dụng; nếu không có GStreamer, tệp thực thi vẫn hoạt động bình thường, thẻ chuyển mã chỉ báo không khả dụng. Bản build cơ bản sẽ bỏ hoàn toàn tính năng này.

#### Windows

Chạy tệp `TorrServer-LT-windows-amd64.exe` (hoặc `TorrServer-LT-windows-amd64-gst.exe` nếu muốn dùng tính năng chuyển mã).

#### Linux

Chạy lệnh trong terminal:

```bash
curl -s https://raw.githubusercontent.com/9000000/TorrServer-LT/master/installTorrServerLinux.sh | sudo bash
```

Kịch bản (script) hỗ trợ cài đặt tương tác và tự động (non-interactive), cấu hình, cập nhật và gỡ bỏ. Khi chạy tương tác, bạn có thể:

- **Install/Update**: Chọn cài đặt hoặc cập nhật TorrServer
- **Reconfigure**: Nếu TorrServer đã được cài đặt, bạn sẽ được hỏi để cấu hình lại các thiết lập (port, auth, chế độ read-only, logging, BBR)
- **Uninstall**: Gõ `Delete` (hoặc `Удалить` bằng tiếng Nga) để gỡ cài đặt TorrServer

**Tải xuống trước và cấp quyền thực thi:**

```bash
curl -s https://raw.githubusercontent.com/9000000/TorrServer-LT/master/installTorrServerLinux.sh -o installTorrServerLinux.sh && chmod 755 installTorrServerLinux.sh
```

**Ví dụ lệnh trong terminal:**

- Cài đặt một phiên bản cụ thể:

  ```bash
  sudo bash ./installTorrServerLinux.sh --install MatriX.142.LT-1.1.8 --silent
  ```

- Cập nhật lên phiên bản mới nhất:

  ```bash
  sudo bash ./installTorrServerLinux.sh --update --silent
  ```

- Thay đổi cấu hình chế độ tương tác:

  ```bash
  sudo bash ./installTorrServerLinux.sh --reconfigure
  ```

- Kiểm tra bản cập nhật:

  ```bash
  sudo bash ./installTorrServerLinux.sh --check
  ```

- Hạ cấp xuống phiên bản cụ thể:

  ```bash
  sudo bash ./installTorrServerLinux.sh --down MatriX.142.LT-1.1.8
  ```

- Gỡ bỏ / Gỡ cài đặt:

  ```bash
  sudo bash ./installTorrServerLinux.sh --remove --silent
  ```

- Thay đổi người dùng chạy dịch vụ systemd:

  ```bash
  sudo bash ./installTorrServerLinux.sh --change-user root --silent
  ```

- Cài đặt biến thể chuyển mã GStreamer (`-gst`):

  ```bash
  sudo bash ./installTorrServerLinux.sh --install --gst --silent
  ```

**Tất cả các lệnh hỗ trợ:**

- `--install [VERSION]` - Cài đặt phiên bản mới nhất hoặc phiên bản chỉ định
- `--update` - Cập nhật lên phiên bản mới nhất
- `--reconfigure` - Cấu hình lại TorrServer (cổng, xác thực, chế độ chỉ đọc, nhật ký log, BBR)
- `--check` - Kiểm tra bản cập nhật (chỉ hiển thị thông tin phiên bản)
- `--down VERSION` - Hạ cấp xuống phiên bản cụ thể
- `--remove` - Gỡ cài đặt TorrServer
- `--change-user USER` - Thay đổi người dùng dịch vụ (root|torrserver)
- `--root` - Chạy dịch vụ dưới quyền người dùng root
- `--silent` - Chế độ tự động với các thiết lập mặc định
- `--gst` - Cài đặt biến thể phát hành `-gst` (chuyển mã GStreamer HLS; chỉ hỗ trợ amd64/arm64, yêu cầu GStreamer ≥ 1.22 trên hệ thống khi chạy). Các lần cập nhật sau sẽ tự động giữ nguyên biến thể này.
- `--no-gst` - Chuyển bản cài đặt `-gst` hiện có về bản cơ bản
- `--help` - Hiển thị trợ giúp

#### macOS

Chạy trong ứng dụng Terminal.app:

```bash
curl -s https://raw.githubusercontent.com/9000000/TorrServer-LT/master/installTorrServerMac.sh -o installTorrServerMac.sh && chmod 755 installTorrServerMac.sh && bash ./installTorrServerMac.sh
```

Script cài đặt thay thế cho máy Mac chip Intel: <https://github.com/dancheskus/TorrServerMacInstaller>

#### Plugin IOCage (Không chính thức)

Trên FreeBSD (TrueNAS/FreeNAS), bạn có thể sử dụng plugin này: <https://github.com/filka96/iocage-plugin-TorrServer>

#### Hệ thống NAS (Không chính thức)

- Nhiều bản build được cung cấp tại liên kết: <https://github.com/vladlenas>
- Kho lưu trữ gói ứng dụng cho **Synology NAS**: <https://grigi.lt>

### Tham số dòng lệnh máy chủ (Server args)

- `--port PORT`, `-p PORT` - Cổng máy chủ web (mặc định 8090)
- `--ip IP`, `-i IP` - Địa chỉ IP lắng nghe của web server (có thể lặp lại; mặc định để trống sẽ lắng nghe trên tất cả giao diện mạng)
- `--ssl` - Bật HTTPS cho web server
- `--sslport PORT` - Cổng HTTPS của web server (mặc định 8091). Nếu không thiết lập, cổng sẽ lấy từ DB (nếu đã lưu trước đó) hoặc dùng mặc định.
- `--sslcert PATH` - Đường dẫn tới tệp chứng chỉ SSL. Nếu không thiết lập, sẽ lấy từ DB hoặc tự động tạo chứng chỉ/khóa tự ký mặc định.
- `--sslkey PATH` - Đường dẫn tới tệp khóa SSL. Nếu không thiết lập, sẽ lấy từ DB hoặc tự động tạo chứng chỉ/khóa tự ký mặc định.
- `--force-https` - Khi kết hợp với `--ssl`, cổng HTTP (`--port`) chỉ trả về **307 Temporary Redirect** chuyển hướng sang HTTPS (`--sslport`). Giao diện web và API chỉ phục vụ qua HTTPS. Yêu cầu phải có `--ssl` (khởi động sẽ thất bại nếu dùng `--force-https` mà không có `--ssl`). Mặc định tắt để HTTP thường vẫn hoạt động khi không bật SSL.
- `--path PATH`, `-d PATH` - Đường dẫn thư mục chứa cơ sở dữ liệu và tệp cấu hình
- `--logpath LOGPATH`, `-l LOGPATH` - Đường dẫn tệp log của máy chủ
- `--weblogpath WEBLOGPATH`, `-w WEBLOGPATH` - Đường dẫn tệp log truy cập web
- `--rdb`, `-r` - Khởi động cơ sở dữ liệu ở chế độ chỉ đọc (read-only)
- `--httpauth`, `-a` - Bật xác thực HTTP Auth cho tất cả yêu cầu
- `--dontkill`, `-k` - Không tắt máy chủ khi nhận tín hiệu kết thúc (signal)
- `--ui`, `-u` - Tự động mở trang torrserver trên trình duyệt
- `--torrentsdir TORRENTSDIR`, `-t TORRENTSDIR` - Tự động nạp các tệp torrent từ thư mục chỉ định
- `--torrentaddr TORRENTADDR` - Địa chỉ torrent client (định dạng [IP]:PORT, ví dụ: `:32000`, `127.0.0.1:32768`, v.v.)
- `--pubipv4 PUBIPV4`, `-4 PUBIPV4` - Thiết lập địa chỉ IPv4 công cộng
- `--pubipv6 PUBIPV6`, `-6 PUBIPV6` - Thiết lập địa chỉ IPv6 công cộng
- `--searchwa`, `-s` - Cho phép tìm kiếm không cần xác thực
- `--maxsize MAXSIZE`, `-m MAXSIZE` - Dung lượng luồng tối đa cho phép (tính bằng Byte)
- `--tg TGTOKEN`, `-T TGTOKEN` - Token [Telegram bot](server/tgbot/README.md)
- `--fuse FUSEPATH`, `-f FUSEPATH` - Đường dẫn mount FUSE
- `--webdav` - Bật dịch vụ WebDAV
- `--proxyurl PROXYURL` - Đặt URL proxy cho lưu lượng BitTorrent (http, socks4, socks5, socks5h), ví dụ: `socks5h://user:password@example.com:2080`
- `--proxymode PROXYMODE` - Đặt chế độ proxy: "tracker" (chỉ HTTP tracker, mặc định), "peers" (chỉ kết nối peer), hoặc "full" (toàn bộ lưu lượng)
- `--help`, `-h` - Hiển thị trợ giúp và thoát
- `--version` - Hiển thị phiên bản và thoát

Ví dụ:

```bash
TorrServer-darwin-arm64 [--port PORT] [--ip IP ...] [--path PATH] [--logpath LOGPATH] [--weblogpath WEBLOGPATH] [--rdb] [--httpauth] [--dontkill] [--ui] [--torrentsdir TORRENTSDIR] [--torrentaddr TORRENTADDR] [--pubipv4 PUBIPV4] [--pubipv6 PUBIPV6] [--searchwa] [--maxsize MAXSIZE] [--tg TGTOKEN] [--fuse FUSEPATH] [--webdav] [--ssl] [--sslport PORT] [--sslcert PATH] [--sslkey PATH] [--force-https]
```

### Chạy trong Docker & Docker Compose

Chạy trong terminal:

```bash
docker run --rm -d --name torrserver -p 8090:8090 ghcr.io/9000000/torrserver-lt:latest
```

Để chạy ở chế độ lưu trữ dữ liệu lâu dài (persistence), hãy gắn volume vào container bằng cách thêm `-v ~/ts:/opt/ts` (thư mục `~/ts` chỉ là ví dụ):

```bash
docker run --rm -d --name torrserver -v ~/ts:/opt/ts -p 8090:8090 ghcr.io/9000000/torrserver-lt:latest
```

#### Biến môi trường (Environments)

- `TS_HTTPAUTH` - Đặt là 1 và đặt tệp xác thực vào thư mục `~/ts/config` để bật xác thực cơ bản (basic auth)
- `TS_RDB` - Nếu là 1, bật cờ `--rdb`
- `TS_DONTKILL` - Nếu là 1, bật cờ `--dontkill`
- `TS_PORT` - Để đổi cổng mặc định thành **5555** (ví dụ), bạn cũng cần đổi `-p 8090:8090` thành `-p 5555:5555`
- `TS_CONF_PATH` - Ghi đè đường dẫn cấu hình torrserver bên trong container. Ví dụ `/opt/tsss`
- `TS_TORR_DIR` - Ghi đè thư mục torrent. Ví dụ `/opt/torr_files`
- `TS_LOG_PATH` - Ghi đè đường dẫn tệp log. Ví dụ `/opt/torrserver.log`
- `TS_PROXYURL` - Đặt URL proxy cho lưu lượng BitTorrent (http, socks4, socks5, socks5h), ví dụ: `socks5h://user:password@example.com:2080`
- `TS_PROXYMODE` - Đặt chế độ proxy: "tracker" (chỉ HTTP tracker, mặc định), "peers" (chỉ kết nối peer), hoặc "full" (toàn bộ lưu lượng)

Ví dụ lệnh ghi đè đầy đủ các tham số (trên giá trị mặc định):

```bash
docker run --rm -d -e TS_PORT=5665 -e TS_DONTKILL=1 -e TS_HTTPAUTH=1 -e TS_RDB=1 -e TS_CONF_PATH=/opt/ts/config -e TS_LOG_PATH=/opt/ts/log -e TS_TORR_DIR=/opt/ts/torrents -e TS_PROXYURL=socks5h://user:password@example.com:2080 -e TS_PROXYMODE=tracker --name torrserver -v ~/ts:/opt/ts -p 5665:5665 ghcr.io/9000000/torrserver-lt:latest
```

#### Docker Compose

```yml
# docker-compose.yml

version: '3.3'
services:
    torrserver:
        image: ghcr.io/9000000/torrserver-lt
        container_name: torrserver
        network_mode: host    # để sử dụng tính năng DLNA
        environment:
            - TS_PORT=5665
            - TS_DONTKILL=1
            - TS_HTTPAUTH=0
            - TS_CONF_PATH=/opt/ts/config
            - TS_TORR_DIR=/opt/ts/torrents
        volumes:
            - './CACHE:/opt/ts/torrents'
            - './CONFIG:/opt/ts/config'
        ports:
            - '5665:5665'
        restart: unless-stopped
        

```

### Smart TV (sử dụng Media Station X)

1. Cài đặt **Media Station X** trên Smart TV của bạn (xem [hỗ trợ nền tảng](https://msx.benzac.de/info/?tab=PlatformSupport))

2. Mở ứng dụng và truy cập: **Settings -> Start Parameter -> Setup**

3. Nhập IP và cổng hiện tại của TorrServer, ví dụ `127.0.0.1:8090`

## Thiết lập cài đặt (Settings)

Phần lớn các thiết lập được cấu hình trên giao diện Web UI (**Settings**), được lưu trong DB cấu hình và áp dụng trực tiếp — việc lưu cài đặt sẽ kết nối lại phiên torrent, giúp thay đổi có hiệu lực mà không cần khởi động lại server. Các tùy chọn bộ nhớ đệm và phát trực tuyến quan trọng nhất đối với bản fork libtorrent này bao gồm:

| Cài đặt | Mặc định | Chức năng |
|---------|---------|--------------|
| **Dung lượng cache** (`CacheSize`) | 64 MB | Hạn mức RAM (hoặc đĩa) cho bộ nhớ đệm mảnh (piece cache). Bộ nhớ đệm phát trực tuyến giữ lại cửa sổ đọc phía trước + các mảnh vừa phát và giải phóng phần còn lại; một mảnh bị giải phóng sẽ bị bỏ trạng thái `have` trong libtorrent để khi tua lại vùng đó sẽ tải lại thay vì bị treo. |
| **Cache đọc trước** (`ReaderReadAHead`) | 95% | Cửa sổ phát trực tuyến phía trước tính theo tỷ lệ phần trăm của cache — mức độ ưu tiên tải các mảnh phía trước vị trí đang phát (được phân cấp để ưu tiên tải vị trí đang phát trước). |
| **Tải trước trước khi phát** (`PreloadCache`) | 50% | Nạp trước tỷ lệ phần trăm cache này ở đầu tệp trước khi bắt đầu phát (ví dụ: 64 MB × 50% = 32 MB). |
| **Bù đệm mảnh lẻ cuối tệp** (`PadTailPartial`) | Tắt | **Mới.** Khi mảnh cuối cùng của tệp là một mảnh lẻ ngắn (nhỏ hơn 1 mảnh đầy đủ *và* nhỏ hơn 5 MB), bộ nhớ đệm sẽ bị thiếu hụt; tùy chọn này ghim thêm 1 mảnh ở đuôi để lấp đầy (có thể vượt quá dung lượng cấu hình tối đa 1 mảnh). |
| **Chế độ End-game** (`DisableEndGame`) | Bật | **Mới.** Yêu cầu các mảnh bộ nhớ đệm cuối cùng từ tất cả các peer cùng lúc để hoàn tất/tua nhanh hơn; tắt đi để giảm lưu lượng trùng lặp. |
| **Cache trên đĩa** (`UseDisk` + `TorrentsSavePath`) | Tắt | Lưu các mảnh lên ổ đĩa theo đường dẫn `TorrentsSavePath/<hash>/<pieceID>` thay vì lưu trên RAM. |
| **Xóa cache khi gỡ torrent** (`RemoveCacheOnDrop`) | Tắt | Xóa bộ nhớ đệm trên đĩa khi torrent bị xóa. |
| **Tải lên** (`DisableUpload`) | Bật | Tắt đi để chạy ở chế độ chỉ tải (leech-only, không mở unchoke cho peer — không chia sẻ/seed). |

Chỉ số tua EOF (MP4 `moov` / MKV cues / AVI `idx1`) ở cuối tệp được đưa vào bộ nhớ đệm **tự động** — 1 mảnh đầy đủ nếu kích thước mảnh > 5 MB, ngược lại là 5 MB — để trình phát đọc chỉ số và tua tức thì; không cần cấu hình cài đặt (`PadTailPartial` chỉ bổ sung cache khi mảnh đuôi là một mảnh lẻ ngắn).
`PadTailPartial` nằm ở thẻ **Main** và bật/tắt End-game nằm ở thẻ **Additional** (thẻ Additional yêu cầu chế độ PRO). Thẻ Additional cũng quản lý giới hạn kết nối/tốc độ (bao gồm **Kết nối DHT tối đa**, `DHTConnectionsLimit` → `dht_max_peers` của libtorrent, mặc định 500), DHT, **PEX** (trao đổi peer — khi tắt sẽ thực sự loại bỏ plugin `ut_pex`), LSD/UPnP, mã hóa, DLNA, HTTPS, proxy và tìm kiếm Torznab.

Bản build `-gst` sẽ có thêm thẻ cài đặt **GStreamer** (chế độ PRO; chỉ hiển thị khi tính năng được biên dịch sẵn) chứa các tùy chọn chuyển mã: chọn codec chuyển mã (video H264/H265/AV1/VP9/VP8, bitrate/kênh AAC cho âm thanh), độ dài phân đoạn, giới hạn tác vụ song song và đè đường dẫn/phiên bản GStreamer. Phát lại thông qua `/gst/{hash}/master.m3u8` (HLS); hỗ trợ định dạng MKV/WebM (và AVI khi bật `TranscodeAVI`), âm thanh được chuyển mã sang AAC — dành cho các trình phát không thể giải mã AC3/EAC3/DTS. Ứng dụng cũng có thể trích xuất phụ đề văn bản nhúng dạng WebVTT, chuyển đổi không gian màu HDR sang SDR, và sử dụng bộ mã hóa H.264 phần cứng khi có sẵn.
Chi tiết đầy đủ — hướng dẫn cài đặt theo từng OS, các trường cấu hình và API — có trong mục [GStreamer](#gstreamer) bên dưới.

## Phát triển (Development)

Bản fork này liên kết **libtorrent 2.1.0 (arvidn)** vào máy chủ Go thông qua một lớp đệm CGo (`server/lt`). Không giống như bản thuần Go upstream, mọi bản build do đó đều là **CGo + C++** và cần toolchain libtorrent/Boost. Một tính năng của lớp đệm — hàm `we_dont_have` theo từng mảnh mà cache streaming sử dụng để tải lại các vùng đã bị giải phóng khi tua ngược — truy cập vào các hàm nội bộ của libtorrent mà bản động **shared** `libtorrent-rasterbar` (từ distro / Homebrew) không xuất ra ngoài. Tính năng này chỉ được biên dịch khi bản build định nghĩa `TSL_HAVE_LT_INTERNALS`, điều mà các script `build/*.sh` thực hiện vì chúng liên kết với bản **tĩnh (static)** libtorrent biên dịch từ mã nguồn. Một lệnh `go build` thông thường đối với `libtorrent-rasterbar-dev` hệ thống vẫn có thể liên kết (tính năng sẽ tự chuyển về dạng no-op), phục vụ cho quá trình phát triển; tuy nhiên việc tua lại vùng cache đã bị giải phóng sẽ không tải lại dữ liệu. Để có đầy đủ tính năng, hãy biên dịch qua các script bên dưới (hoặc truyền `CGO_CXXFLAGS=-DTSL_HAVE_LT_INTERNALS` khi bạn tự liên kết với libtorrent tĩnh).

### Yêu cầu tiên quyết (Prerequisites)

- **Go 1.25+**
- Bộ công cụ C/C++ trên máy host (`gcc`/`g++`), `curl`, `git` — để khởi tạo `b2` của Boost và biên dịch libtorrent. Không yêu cầu cmake, Docker hay QEMU.
- Đối với giao diện Web UI: **Node.js 18+** và **yarn**.

### Máy chủ cục bộ (linux-amd64)

Biên dịch phụ thuộc libtorrent + Boost một lần (lưu trong `_deps/linux-amd64/`); lệnh này cũng xuất tệp binary hoàn chỉnh tại `_out/`:

```bash
build/linux-amd64.sh
```

Để phát triển mã nguồn Go với `go run` / `go test`, trỏ pkg-config đến cây thư viện tĩnh đó để cgo liên kết đúng libtorrent:

```bash
cd server
export PKG_CONFIG_PATH=$PWD/../_deps/linux-amd64/lib/pkgconfig
export PKG_CONFIG_LIBDIR=$PWD/../_deps/linux-amd64/lib/pkgconfig
export CGO_LDFLAGS="-L$PWD/../_deps/linux-amd64/lib"
go run ./cmd        # hoặc: go test ./...
```

Sau đó truy cập <http://127.0.0.1:8090>.

### Biên dịch chéo (Cross-compilation - không dùng Docker)

Thư mục `build/` hỗ trợ biên dịch chéo cho **mọi** mục tiêu được hỗ trợ trên máy host Linux: biên dịch libtorrent và `boost_system` từ mã nguồn bằng Boost.Build (`b2`) vào `_deps/<target>/`, sau đó liên kết tệp thực thi Go với thư viện đó thông qua pkg-config. Kết quả xuất ra tại `_out/TorrServer-LT-<target>`; các nền tảng hỗ trợ GStreamer (linux amd64/arm64, windows amd64, macOS) cũng sẽ tạo thêm `_out/TorrServer-LT-<target>-gst` được biên dịch với `-tags gst` — biến thể này là thuần Go (GStreamer được nạp động khi chạy qua purego), do đó không cần thêm toolchain và tái sử dụng bộ nhớ đệm build.

```bash
build/all.sh                        # tất cả các mục tiêu máy host có thể build
TARGETS="linux-arm64" build/all.sh  # chỉ định một số mục tiêu
```

`all.sh` sẽ báo trạng thái `OK` / `FAIL` / `SKIP` (thiếu toolchain) cho từng mục tiêu.
Danh sách các mục tiêu và toolchain tương ứng cần cài đặt:

| Mục tiêu (Target) | Toolchain cần cài đặt |
|-------------------|------------------------------------------------------------|
| `linux-amd64`     | gcc/g++ trên máy host (không cần cài thêm) |
| `linux-arm64`     | `gcc-aarch64-linux-gnu g++-aarch64-linux-gnu` |
| `linux-armv7`     | `gcc-arm-linux-gnueabihf g++-arm-linux-gnueabihf` |
| `windows-amd64`   | `gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64` |
| `android-arm64` / `android-armv7` | Android NDK r26+ (`export ANDROID_NDK_HOME=…`) |
| `darwin-arm64` / `darwin-amd64`   | OSXCross + Apple macOS SDK (xem bên dưới) |

libtorrent được biên dịch với `crypto=openssl` và `webtorrent=on`: mỗi mục tiêu sẽ có một bản OpenSSL tĩnh được build từ mã nguồn (không cần OpenSSL hệ thống) cùng các phụ thuộc WebRTC (libdatachannel/usrsctp/libjuice), cho phép hỗ trợ tracker https/web seeds và WebTorrent (tracker `wss://`, peer trình duyệt). Máy host cần có `cmake` cho các phụ thuộc WebRTC. Tất cả đều được liên kết tĩnh — phụ thuộc động duy nhất trong tệp thực thi cuối cùng là libc/libstdc++/libgcc (trên Windows các thư viện này cũng được liên kết tĩnh). Phiên bản được cố định trong `build/_common.sh` (Boost 1.92.0, libtorrent v2.1.1, OpenSSL 3.5.7) và có thể ghi đè, ví dụ: `LIBTORRENT_TAG=v2.0.13 build/linux-arm64.sh`. Chi tiết đầy đủ và bảng yêu cầu tiên quyết cho từng mục tiêu: [`build/README.md`](build/README.md).

### macOS

Có 3 cách để tạo tệp thực thi cho macOS (`darwin-amd64` Intel, `darwin-arm64` Apple Silicon):

1. **Trực tiếp trên máy Mac** — cài đặt Go và Xcode command-line tools, sau đó chạy `build/darwin-native.sh arm64` (hoặc `amd64`). Lệnh này sẽ biên dịch phiên bản libtorrent chỉ định từ mã nguồn qua b2 — không lấy từ Homebrew (vốn có phiên bản libtorrent không cố định) — sau đó tạo cả bản cơ bản và bản `-gst`. Đây là cách chuẩn bị bản phân phối được hỗ trợ.
2. **CI** — `.github/workflows/build-macos.yml` chạy cùng script trên cho cả hai kiến trúc trên trình runner Apple Silicon (amd64 chạy qua Rosetta/`-arch x86_64`).
3. **Biên dịch chéo từ Linux qua OSXCross** — yêu cầu Apple macOS SDK (vốn chỉ được cấp phép sử dụng trên phần cứng Apple, do đó thuộc vùng xám pháp lý và chỉ phục vụ mục đích kiểm chứng nội bộ). Đặt `OSXCROSS_ROOT` và chạy script `darwin-arm64.sh` / `darwin-amd64.sh`; hướng dẫn trích xuất SDK có tại [`build/README.md`](build/README.md).

### Giao diện Web (Web UI)

Ứng dụng React trong thư mục `web/` được biên dịch và **nhúng (embedded)** trực tiếp vào tệp thực thi Go (`server/web/pages/template`), do đó các thay đổi giao diện chỉ hiển thị sau khi build lại giao diện *và* build lại server:

```bash
cd web
yarn install
NODE_OPTIONS=--openssl-legacy-provider CI=false yarn build   # CRA cần legacy OpenSSL trên Node 18+

cd ..
go run gen_web.go     # sao chép web/build → cây mã nguồn server, tạo lại bảng //go:embed + đường dẫn
```

`gen_web.go` sẽ tự chạy `yarn build` nếu không tìm thấy `web/build`. Bảng mã nhúng dựa trên tên tệp chunk mã hóa hash của CRA, do đó bạn **bắt buộc** phải tạo lại nó sau khi build — việc chép thủ công `web/build` là chưa đủ. Để phát triển UI trực tiếp mà không cần build lại server, dùng `cd web && yarn start` để proxy tới server đang chạy. Thông tin chi tiết: [`web/README.md`](web/README.md).

### Swagger

Cần cài đặt `swag` để biên dịch tài liệu API:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
cd server && swag init -g web/server.go
swag fmt   # kiểm tra và định dạng chú thích
```

## API

### Tài liệu API

Tài liệu API được lưu trữ dưới dạng định dạng Swagger, có thể truy cập tại đường dẫn `/swagger/index.html`.

## Xác thực (Authentication)

Tệp dữ liệu người dùng nằm cùng thư mục với cài đặt. Xác thực cơ bản (Basic auth), tìm hiểu thêm tại wiki <https://en.wikipedia.org/wiki/Basic_access_authentication>.

Tệp `accs.db` dưới định dạng JSON:

```json
{
    "User1": "Pass1",
    "User2": "Pass2"
}
```

Lưu ý: Bạn cần bật xác thực bằng tham số khởi động `-a` (`--httpauth`) cho TorrServer.

## Danh sách trắng/Danh sách đen IP (Whitelist/Blacklist IP)

Tệp danh sách nằm cùng thư mục với `config.db`.

- Tên tệp danh sách trắng: `wip.txt`
- Tên tệp danh sách đen: `bip.txt`

Danh sách trắng có quyền ưu tiên cao nhất.

Ví dụ:

```text
local:127.0.0.0-127.0.0.255
127.0.0.0-127.0.0.255
local:127.0.0.1
127.0.0.1
# dòng bắt đầu bằng dấu # là chú thích
```

## Torznab

TorrServer có thể kết nối với các trình đánh chỉ mục (indexer) **Torznab** để bạn tìm kiếm torrent từ các công cụ như **Jackett** và **Prowlarr**, bao gồm việc tìm kiếm trên nhiều indexer đã cấu hình cùng một lúc.

Cấu hình trong giao diện web: **Settings → Torznab**.

### Tham số Indexer

Mỗi indexer Torznab yêu cầu:

- **Host URL**: Đường dẫn URL đầy đủ tới điểm cuối Torznab API.
  - Ví dụ Jackett:

  ```shell
  http://192.168.1.10:9117/api/v2.0/indexers/all/results/torznab/
  ```

  - Ví dụ Prowlarr:
  
  ```shell
  http://localhost:9696/1
  ```
  
  - Đảm bảo bao gồm dấu gạch chéo cuối (`/`) chính xác trong URL indexer của bạn theo đúng yêu cầu từ nhà cung cấp Torznab. TorrServer sẽ cố gắng định dạng đường dẫn chính xác, nhưng khớp với định dạng mong đợi của indexer là tốt nhất để tránh lỗi kết nối.
  
- **API Key**: Mã khóa API từ trình quản lý indexer Torznab của bạn.

### Bật tìm kiếm Torznab

1. Mở **Settings**.
2. Chuyển sang thẻ **Torznab**.
3. Bật **Enable Torznab Search**.
4. Nhập **Host URL** và **API Key**, sau đó nhấn **Add Server** cho mỗi indexer.
5. Nhấn **Save** để lưu thiết lập.

## GStreamer

GStreamer cho phép **chuyển mã HLS** đối với các torrent Matroska/WebM (và AVI khi bật `TranscodeAVI`) khi thiết bị client không thể phát trực tiếp video hoặc codec âm thanh gốc (thường là âm thanh: AC3/EAC3/DTS → AAC; video H.264/H.265/AV1/VP9/VP8 sẽ truyền thẳng passthrough, hoặc được mã hóa lại khi tùy chọn `Transcode*` tương ứng bật). Nó cũng có thể chuyển đổi phụ đề văn bản nhúng sang WebVTT dạng phân đoạn (`Subtitles`), chuyển không gian màu HDR video sang SDR (`HDRToSDR`), và mã hóa trên GPU bằng bộ mã hóa H.264 phần cứng, tự động chuyển về x264 khi không có sẵn (`HardwareAcceleration`, `UseGPU`).

### Tệp thực thi `-gst` (Yêu cầu chính)

Cách **duy nhất** để bật GStreamer trong TorrServer-LT là chạy bản build được biên dịch với cờ `gst`. Không có công tắc bật/tắt tính năng này trong phần cài đặt.

| Tệp thực thi | GStreamer |
| --- | --- |
| `TorrServer-LT-<platform>-gst` | **Có** — các tuyến đường `/gst/*` và chuyển mã |
| `TorrServer-LT-<platform>` (tiêu chuẩn) | **Không** — `GET /gst/settings` trả về `built_in: false`; không có tuyến đường `/gst/*` nào khác được đăng ký và đường dẫn xử lý không chạy |

Tải xuống `TorrServer-LT-*-gst` từ trang [releases](https://github.com/9000000/TorrServer-LT/releases) (Windows amd64 / Linux amd64+arm64 / macOS amd64+arm64), sử dụng script cài đặt tự động (`--gst`, xem [Cài đặt](#cài-đặt)), hoặc tự biên dịch — các script trong `build/*.sh` sẽ tạo cả hai biến thể tự động, và nếu biên dịch thủ công chỉ cần thêm tag vào môi trường cgo thông thường:

```bash
cd server
go build -tags gst -o TorrServer-LT-gst ./cmd
```

Hành vi cho từng codec được cấu hình trên thẻ cài đặt **GStreamer** (`TranscodeH264`, `TranscodeH265`, `TranscodeAV1`, `TranscodeVP9`, `TranscodeVP8`, `TranscodeAVI`), cùng với các công tắc phụ đề (`Subtitles`), HDR→SDR (`HDRToSDR`) và mã hóa phần cứng (`HardwareAcceleration`, `UseGPU`).

### GStreamer đi kèm (Bundled) vs nạp động (Dynamic)

TorrServer-LT vẫn duy trì dạng tệp thực thi Go đơn lẻ. GStreamer **không** được liên kết tĩnh vào chương trình khi biên dịch — các thư viện được nạp khi chạy thông qua `dlopen` / `LoadLibrary` (purego). Điểm khác biệt là **nơi** lấy thư viện và plugin GStreamer:

| Chế độ | Cách thức hoạt động | Trường hợp sử dụng điển hình |
| --- | --- | --- |
| **Đi kèm (Bundled - portable)** | Đặt thư mục `gst-lib/` cạnh tệp thực thi TorrServer (cấu trúc giống runtime GStreamer trích xuất) | Cài đặt di động (portable), triển khai tùy chỉnh |
| **Nạp động (Dynamic - system)** | TorrServer nạp `libgstreamer` / DLL từ gói hệ điều hành hoặc đường dẫn cài đặt hệ thống (`GSTPath`, `/opt/gstreamer`, đường dẫn framework, v.v.) | Linux, macOS, Windows dùng [bộ cài GStreamer chính thức](https://gstreamer.freedesktop.org/download/) |
| **Đi kèm (Bundled - nhúng)** | GStreamer runtime được đóng gói bên trong binary và tự trích xuất vào bộ nhớ tạm khi chạy lần đầu — cần bản build Windows tùy chỉnh với tag `embed_gstlib`; tệp phát hành mặc định không chứa | Triển khai tự đóng gói hoàn chỉnh trên Windows |

Thứ tự tự động phát hiện: `GSTPath` từ cài đặt → `gst-lib/` cạnh tệp thực thi → các đường dẫn hệ thống phổ biến → `LD_LIBRARY_PATH` / `PATH`.

Kiểm tra chế độ đang sử dụng bằng `GET /gst/echo` hoặc dòng trạng thái trên thẻ cài đặt **GStreamer**.

### Cài đặt GStreamer để nạp động

Cần thiết cho các bản build `-gst` trừ khi bạn cung cấp thư mục `gst-lib/` di động. Phiên bản GStreamer tối thiểu: **1.22**.

**Debian / Ubuntu**

```bash
sudo apt update
sudo apt install -y \
  gstreamer1.0-tools \
  gstreamer1.0-plugins-base \
  gstreamer1.0-plugins-good \
  gstreamer1.0-plugins-bad \
  gstreamer1.0-plugins-ugly \
  gstreamer1.0-libav
```

**Fedora / RHEL / Rocky / AlmaLinux**

```bash
sudo dnf install -y \
  gstreamer1-tools \
  gstreamer1-plugins-base \
  gstreamer1-plugins-good \
  gstreamer1-plugins-bad-free \
  gstreamer1-plugins-ugly-free \
  gstreamer1-libav
```

`x264enc` (dùng khi chuyển mã video sang H.264) có thể yêu cầu bật [RPM Fusion](https://rpmfusion.org/) trên Fedora.

**Arch Linux**

```bash
sudo pacman -S --needed \
  gst-plugins-base \
  gst-plugins-good \
  gst-plugins-bad \
  gst-plugins-ugly \
  gst-libav
```

**macOS**

Dùng [Framework chính thức](https://gstreamer.freedesktop.org/download/) hoặc Homebrew:

```bash
brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly gst-libav
```

Đặt `GSTPath` nếu cần (ví dụ `/Library/Frameworks/GStreamer.framework/Versions/1.0`).

**Windows (nạp động)**

Cài đặt MSVC 64-bit runtime từ [gstreamer.freedesktop.org](https://gstreamer.freedesktop.org/download/) (mặc định: `C:\Program Files\gstreamer\1.0\mingw_x86_64`), hoặc đặt thư mục `gst-lib/` cạnh tệp exe.

### Cấu hình trên giao diện Web UI

1. Mở **Settings**.
2. Bật chế độ **PRO mode**.
3. Mở thẻ **GStreamer** (chỉ xuất hiện trên các bản build `-gst`).
4. Điều chỉnh các tùy chọn và nhấn **Save GStreamer Settings**.

Cài đặt GStreamer được lưu trữ riêng biệt với các thiết lập BitTorrent chính và có hiệu lực ngay lập tức cho các luồng dữ liệu mới.

### Các trường cấu hình

| Trường | Mô tả |
| --- | --- |
| `GSTVersion` | Phiên bản GStreamer đã cài đặt (tối thiểu `1.22`, ví dụ `1.22`, `1.28`). Được sử dụng để lựa chọn tính năng pipeline. |
| `GSTPath` | Đường dẫn đến cài đặt GStreamer. Để trống = tự động phát hiện. |
| `Source` | Chế độ URL đầu vào: `stream` (`/stream/...`) hoặc `play` (`/play/...`). |
| `MaxTasks` | Giới hạn số tác vụ chuyển mã song song (`0` = mặc định). |
| `InactiveMinutes` | Tạm dừng pipeline sau khoảng thời gian này (tính bằng phút) không phát nội dung. |
| `AACBitrateKbps` | Băng thông chuyển mã âm thanh (bitrate) tính bằng kbps. |
| `SegmentSeconds` | Độ dài phân đoạn HLS tính bằng giây. |
| `appsinkBuffers` | Số lượng bộ đệm trong hàng chờ appsink. |
| `TranscodeH264` / `TranscodeH265` / `TranscodeAV1` / `TranscodeVP9` | Mã hóa lại codec video tương ứng thay vì truyền thẳng passthrough. |
| `VideoBitrate` | Băng thông video mục tiêu tính bằng kbps khi chuyển mã video. |
| `tempfs` | Sử dụng bộ nhớ tempfs trên RAM cho các phân đoạn (Linux). |
| `tempfs_ring` | Số block ring tempfs bổ sung (`0` = mặc định). |
| `PlaylistHLS` | **Mở rộng của bản fork.** Danh sách phát M3U được tạo sẽ trỏ các tệp MKV/WebM tới `/gst/{hash}/master.m3u8` thay vì liên kết trực tiếp `/stream`, giúp mọi trình phát hỗ trợ HLS nhận được âm thanh đã chuyển mã AAC trực tiếp từ playlist; các định dạng container khác giữ nguyên liên kết trực tiếp. Không có tác dụng trên các bản build tiêu chuẩn. |

### API

**Thiết lập cài đặt** (yêu cầu xác thực khi bật `--httpauth`):

- `GET /gst/settings` — trên bản build `-gst`: trả về `built_in`, cấu hình hiện tại, và giá trị mặc định của nền tảng; trên bản tiêu chuẩn: chỉ trả về `{ "built_in": false }`
- `POST /gst/settings` — cập nhật hoặc đặt lại cấu hình (trả về `404` trên bản tiêu chuẩn)

  ```json
  { "action": "set", "config": { "GSTVersion": 1.22, "Source": "stream" } }
  ```

  Đặt lại về mặc định:

  ```json
  { "action": "def" }
  ```

**Phát trực tuyến** (chỉ có trên bản build `-gst`):

| Endpoint | Mô tả |
| --- | --- |
| `GET /gst/echo` | Kiểm tra sức khỏe GStreamer / gst-discoverer |
| `GET /gst/:hash/probe` | Phân tích codec tệp torrent (qua tham số `index`, `id`, hoặc `fileID`) |
| `GET /gst/:hash/master.m3u8` | Danh sách phát chính HLS (master playlist) |
| `GET /gst/:hash/init.mp4` | Phân đoạn khởi tạo (initialization segment) |
| `GET /gst/:hash/seg/*segment` | Phân đoạn truyền thông (media segment) |
| `GET /gst/:hash/heartbeat` | Giữ kết nối (keep-alive) cho tác vụ chuyển mã đang hoạt động |
| `GET /gst/remove` | Dừng tác vụ chuyển mã (qua tham số `hash` hoặc `id`) |

Các bản thực thi tiêu chuẩn sẽ phục vụ thông tin Swagger filtered khi chạy (chỉ có `/gst/settings`); bản build `-gst` sẽ cung cấp tài liệu đầy đủ các endpoint `/gst/*`.

## Ủng hộ (Donate)

- [YooMoney](https://yoomoney.ru/to/410013733697114/200)
- [Boosty](https://boosty.to/yourok)
- [TBank](https://www.tbank.ru/cf/742qEMhKhKn)

## Lời cảm ơn đến tất cả những ai đã kiểm thử và đóng góp

- [anacrolix](https://github.com/anacrolix) Matt Joiner
- [tsynik](https://github.com/tsynik) Nikk Gitanes
- [dancheskus](https://github.com/dancheskus) cho giao diện web React và mã nguồn PWA
- [kolsys](https://github.com/kolsys) cho hỗ trợ Media Station X ban đầu
- [damiva](https://github.com/damiva) cho cập nhật mã nguồn Media Station X
- [vladlenas](https://github.com/vladlenas) cho các bản build NAS
- [pavelpikta](https://github.com/pavelpikta) Pavel Pikta cho script cài đặt Linux và nhiều hơn thế
- [Nemiroff](https://github.com/Nemiroff) Tw1cker
- [spawnlmg](https://github.com/spawnlmg) SpAwN_LMG đã kiểm thử
- [TopperBG](https://github.com/TopperBG) Dimitar Maznekov cho bản dịch giao diện tiếng Bulgaria
- [FaintGhost](https://github.com/FaintGhost) Zhang Yaowei cho bản dịch giao diện tiếng Trung giản thể
- [Anton111111](https://github.com/Anton111111) Anton Potekhin cho các bản sửa lỗi chế độ ngủ trên Windows
- [lieranderl](https://github.com/lieranderl) Evgeni cho đóng góp mã hỗ trợ SSL
- [cocool97](https://github.com/cocool97) cho tài liệu openapi API và các phân loại torrent
- [shadeov](https://github.com/shadeov) cho các cải tiến tài liệu README
- [butaford](https://github.com/butaford) Pavel cho các script và tệp docker
- [filimonic](https://github.com/filimonic) Alexey D. Filimonov
- [leporel](https://github.com/leporel) Viacheslav Evseev
- và nhiều người khác
