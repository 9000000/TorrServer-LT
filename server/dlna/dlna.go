package dlna

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/dms/dlna/dms"
	"github.com/anacrolix/log"
	"github.com/wlynxg/anet"

	"server/netbind"
	"server/settings"
	"server/web/pages/template"
)

var (
	dmsMu     sync.Mutex
	dmsServer *dms.Server
)

// ssdpNoiseFilter wraps the real log handlers and drops the benign SSDP
// datagram-parse errors the dms multicast listener emits on every malformed
// read — almost always "EOF"/"unexpected EOF" from the M-SEARCH/NOTIFY packets
// other UPnP devices on the LAN broadcast (ssdp.go: me.Logger.Println(err) in
// handle()). They carry no level (logged at default), so they surfaced as a
// constant "ssdp ... msg=EOF" drip in the server log without being actionable.
// Every other dms/ssdp record (real errors, "started SSDP", HTTP, eventing) is
// forwarded untouched, so this only quiets that one noise source.
type ssdpNoiseFilter struct {
	next []log.Handler
}

func (f ssdpNoiseFilter) Handle(r log.Record) {
	if isSSDPNoise(r) {
		return
	}
	for _, h := range f.next {
		h.Handle(r)
	}
}

func isSSDPNoise(r log.Record) bool {
	isSSDP := false
	for _, n := range r.Names {
		if n == "ssdp" {
			isSSDP = true
			break
		}
	}
	return isSSDP && strings.Contains(r.Msg.String(), "EOF")
}

func Start() {
	dmsMu.Lock()
	if dmsServer != nil {
		dmsMu.Unlock()
		return
	}
	logger := log.Default.WithNames("dlna")
	dmsLogger := logger.WithNames("dms", "server")
	dmsLogger.SetHandlers(ssdpNoiseFilter{next: dmsLogger.Handlers})

	port := 9080
	for port < 9180 {
		logger.Levelf(log.Info, "Check dlna port %d", port)
		if err := netbind.CheckPort(settings.IPs, strconv.Itoa(port)); err == nil {
			break
		}
		port++
	}
	logger.Levelf(log.Info, "Set dlna port %d", port)
	httpConn, err := netbind.Listen(settings.IPs, strconv.Itoa(port))
	if err != nil {
		dmsMu.Unlock()
		logger.Levelf(log.Error, "dlna listen failed (DLNA disabled): %v", err)
		return
	}

	srv := &dms.Server{
		Logger: dmsLogger,
		Interfaces: func() (ifs []net.Interface) {
			ifaces, err := anet.Interfaces()
			if err != nil {
				logger.Levelf(log.Error, "%v", err)
				return
			}
			for _, i := range ifaces {
				// interface flags seem to always be 0 on Windows
				if runtime.GOOS != "windows" && (i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 || i.Flags&net.FlagMulticast == 0) {
					continue
				}
				ifs = append(ifs, i)
			}
			return
		}(),
		HTTPConn:            httpConn,
		FriendlyName:        getDefaultFriendlyName(),
		NoTranscode:         true,
		NoProbe:             true,
		StallEventSubscribe: false,
		Icons: []dms.Icon{
			{
				Width:    48,
				Height:   48,
				Depth:    24,
				Mimetype: "image/png",
				Bytes:    template.Dlnaicon48png,
			},
			{
				Width:    120,
				Height:   120,
				Depth:    24,
				Mimetype: "image/png",
				Bytes:    template.Dlnaicon120png,
			},
		},
		LogHeaders:     settings.BTsets().EnableDebug,
		NotifyInterval: 30 * time.Second,
		AllowedIpNets: func() []*net.IPNet {
			var nets []*net.IPNet
			_, ipnet, _ := net.ParseCIDR("0.0.0.0/0")
			nets = append(nets, ipnet)
			_, ipnet, _ = net.ParseCIDR("::/0")
			nets = append(nets, ipnet)
			return nets
		}(),
		OnBrowseDirectChildren: onBrowse,
		OnBrowseMetadata:       onBrowseMeta,
	}

	if err := srv.Init(); err != nil {
		_ = httpConn.Close()
		dmsMu.Unlock()
		logger.Levelf(log.Error, "error initing dms server (DLNA disabled): %v", err)
		return
	}
	dmsServer = srv
	dmsMu.Unlock()

	go func(s *dms.Server) {
		if err := s.Run(); err != nil {
			logger.Levelf(log.Error, "dms server stopped: %v", err)
		}
	}(srv)
}

func Stop() {
	dmsMu.Lock()
	srv := dmsServer
	dmsServer = nil
	dmsMu.Unlock()
	if srv != nil {
		srv.Close()
	}
}

func onBrowse(path, rootObjectPath, host, userAgent string) (ret []interface{}, err error) {
	if path == "/" {
		ret = getRoot()
		return
	} else if path == "/TR" {
		ret = getTorrentCategories()
		return
	} else if strings.HasPrefix(path, "/TR/") {
		ret = getTorrentsByCategory(path)
		return
	} else if isHashPath(path) {
		ret = getTorrent(path, host)
		return
	} else if filepath.Base(path) == "LD" {
		ret = loadTorrent(path, host)
	}
	return
}

func onBrowseMeta(path string, rootObjectPath string, host, userAgent string) (ret interface{}, err error) {
	ret = getTorrentMeta(path, host)
	if ret == nil {
		err = fmt.Errorf("meta not found")
	}
	return
}

func getDefaultFriendlyName() string {
	logger := log.Default.WithNames("dlna")

	if settings.BTsets().FriendlyName != "" {
		return settings.BTsets().FriendlyName
	}

	ret := "TorrServer"
	userName := ""
	user, err := user.Current()
	if err != nil {
		logger.Printf("getDefaultFriendlyName could not get username: %s", err)
	} else {
		userName = user.Name
	}
	host, err := os.Hostname()
	if err != nil {
		logger.Printf("getDefaultFriendlyName could not get hostname: %s", err)
	}

	if userName == "" && host == "" {
		return ret
	}

	if userName != "" && host != "" {
		if userName == host {
			return ret + ": " + userName
		}
		return ret + ": " + userName + " on " + host
	}

	if host == "localhost" { // useless host, use 1st IP
		ifaces, err := anet.Interfaces()
		if err != nil {
			return ret + ": " + userName + "@" + host
		}
		var list []string
		for _, i := range ifaces {
			// interface flags seem to always be 0 on Windows
			if runtime.GOOS != "windows" && (i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 || i.Flags&net.FlagMulticast == 0) {
				continue
			}
			addrs, _ := anet.InterfaceAddrsByInterface(&i)
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if !ip.IsLoopback() && ip.To4() != nil {
					list = append(list, ip.String())
				}
			}
		}
		if len(list) > 0 {
			sort.Strings(list)
			return ret + " " + list[0]
		}
	}
	return ret + ": " + userName + "@" + host
}
