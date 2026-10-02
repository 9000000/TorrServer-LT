package web

import (
	"net"
	"os"
	"sort"

	gstreamer "server/gstreamer/bridge"
	"server/netbind"

	"server/torrfs/fuse"
	"server/torrfs/webdav"

	"server/rutor"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/location/v2"
	"github.com/gin-gonic/gin"
	"github.com/wlynxg/anet"

	"server/bonjour"
	"server/dlna"
	"server/settings"
	"server/web/msx"

	"server/log"
	"server/lt"
	"server/mcp"
	"server/torr"
	"server/version"
	"server/web/api"
	"server/web/auth"
	"server/web/pages"
	"server/web/sslcerts"
	"server/web/waf"
)

var (
	BTS      = torr.NewBTS()
	waitChan = make(chan error)
)

//	@title			Swagger Torrserver API
//	@version		{version.Version}
//	@description	Torrent streaming server.

//	@license.name	GPL 3.0

//	@BasePath	/

//	@securityDefinitions.basic	BasicAuth

// @externalDocs.description	OpenAPI
// @externalDocs.url			https://swagger.io/resources/open-api/
func Start() {
	log.TLogln("Start TorrServer-LT " + version.Version + " libtorrent " + lt.Version())
	ips := GetLocalIps()
	if len(ips) > 0 {
		log.TLogln("Local IPs:", ips)
	}
	err := BTS.Connect()
	if err != nil {
		log.TLogln("BTS.Connect() error!", err) // waitChan <- err
		os.Exit(1)                              // return
	}
	rutor.Start()

	gin.SetMode(gin.ReleaseMode)

	// corsCfg := cors.DefaultConfig()
	// corsCfg.AllowAllOrigins = true
	// corsCfg.AllowHeaders = []string{"*"}
	// corsCfg.AllowMethods = []string{"*"}
	corsCfg := cors.DefaultConfig()
	corsCfg.AllowAllOrigins = true
	corsCfg.AllowPrivateNetwork = true
	corsCfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "HEAD", "OPTIONS", "DELETE"}
	corsCfg.AllowHeaders = []string{
		"Origin", "Content-Length", "Content-Type", "X-Requested-With", "Accept", "Authorization",
		// MCP Streamable HTTP (browser-based agents)
		"Mcp-Protocol-Version", "Mcp-Session-Id", "Last-Event-ID", "Mcp-Method", "Mcp-Name",
	}

	route := gin.New()
	route.Use(log.WebLogger(), waf.WAF(), gin.Recovery(), cors.New(corsCfg), location.Default())
	auth.SetupAuth(route)

	route.GET("/echo", echo)

	api.SetupRoute(route)
	mcp.Mount(route.Group("/", auth.CheckAuth()))
	gstreamer.SetupRoute(route)
	msx.SetupRoute(route)
	pages.SetupRoute(route)
	if settings.Args.WebDAV {
		webdav.MountWebDAV(route)
	}

	if settings.BTsets().EnableDLNA {
		dlna.Start()
	}
	if settings.BTsets().EnableBonjour {
		bonjour.Start()
	}

	// Auto-mount FUSE filesystem if enabled
	fuse.FuseAutoMount()

	route.GET("/swagger/*any", swaggerHandler())

	// check if https enabled
	if settings.Ssl {
		// Generate a self-signed pair only when none is configured, and never
		// replace a certificate TorrServer did not generate itself.
		cert, key, changed, certErr := sslcerts.EnsureCert(settings.BTsets().SslCert, settings.BTsets().SslKey, ips)
		if certErr != nil {
			// Keep serving plain HTTP instead of taking the whole server down:
			// the certificate is the user's to fix. settings.Ssl goes off so
			// nothing else (force-https redirect, Bonjour, generated links)
			// points at an HTTPS port nobody listens on.
			log.TLogln("HTTPS disabled, certificate unusable:", certErr)
			log.TLogln("Fix --sslcert/--sslkey, or clear them in settings to get a self-signed certificate. HTTP keeps running.")
			settings.Ssl = false
		} else {
			if changed {
				settings.BTsets().SslCert, settings.BTsets().SslKey = cert, key
				log.TLogln("Saving path to ssl cert and key in db", cert, key)
				settings.SetBTSets(settings.BTsets())
			}
			go func() {
				for _, ip := range netbind.Normalize(settings.IPs) {
					addr := netbind.Addr(ip, settings.SslPort)
					go func(addr string) {
						log.TLogln("Start https server at", addr)
						waitChan <- route.RunTLS(addr, cert, key)
					}(addr)
				}
			}()
		}
	}

	go func() {
		if settings.Args != nil && settings.Args.ForceHTTPS && settings.Ssl {
			for _, ip := range netbind.Normalize(settings.IPs) {
				addr := netbind.Addr(ip, settings.Port)
				go func(addr string) {
					waitChan <- runHTTPRedirectToHTTPS(addr)
				}(addr)
			}
			return
		}
		for _, ip := range netbind.Normalize(settings.IPs) {
			addr := netbind.Addr(ip, settings.Port)
			go func(addr string) {
				log.TLogln("Start http server at", addr)
				waitChan <- route.Run(addr)
			}(addr)
		}
	}()
}

func Wait() error {
	return <-waitChan
}

func Stop() {
	gstreamer.Stop()
	dlna.Stop()
	bonjour.Stop()
	// Unmount FUSE filesystem if mounted
	fuse.FuseCleanup()
	BTS.Disconnect()
	waitChan <- nil
}

// echo godoc
//
//	@Summary		Tests server status
//	@Description	Tests whether server is alive or not
//
//	@Tags			API
//
//	@Produce		plain
//	@Success		200	{string}	string	"Server version"
//	@Router			/echo [get]
func echo(c *gin.Context) {
	c.String(200, "%v", version.Version)
}

func GetLocalIps() []string {
	ifaces, err := anet.Interfaces()
	if err != nil {
		log.TLogln("Error get local IPs")
		return nil
	}
	var list []string
	for _, i := range ifaces {
		addrs, _ := anet.InterfaceAddrsByInterface(&i)
		if i.Flags&net.FlagUp == net.FlagUp {
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() {
					list = append(list, ip.String())
				}
			}
		}
	}
	sort.Strings(list)
	return list
}
