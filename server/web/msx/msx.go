package msx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/settings"
	"server/torr"
	"server/utils"
	"server/version"
	"server/web/auth"

	"github.com/gin-gonic/gin"
)

const base, files = "tsmsx.yourok.ru", "media"

var (
	paramMu sync.RWMutex
	param   = "menu:request:interaction:{SERVER}@{PREFIX}" + base + "/start.html"
)

func isBlockedProxyIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

var msxHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var publicIP string
			for _, ipAddr := range ips {
				if isBlockedProxyIP(ipAddr.IP) {
					return nil, fmt.Errorf("msx proxy: blocked internal address %s", ipAddr.IP.String())
				}
				if publicIP == "" {
					publicIP = ipAddr.IP.String()
				}
			}
			if publicIP == "" {
				return nil, errors.New("msx proxy: no public address resolved")
			}
			var dialer net.Dialer
			dialer.Timeout = 10 * time.Second
			return dialer.DialContext(ctx, network, net.JoinHostPort(publicIP, port))
		},
	},
}

func trn(h string) (st, sc string) {
	if h := torr.GetTorrent(h); h != nil {
		if h := h.Status(); h != nil && h.Stat < 5 {
			switch h.Stat {
			case 4:
				sc = "msx-red"
			case 3:
				sc = "msx-green"
			default:
				sc = "msx-yellow"
			}
			st = "{ico:north} " + strconv.Itoa(h.ActivePeers) + " / " + strconv.Itoa(h.TotalPeers) + " {ico:south} " + strconv.Itoa(h.ConnectedSeeders)
		}
	}
	return
}

func rsp(c *gin.Context, r *http.Response, e error) {
	if e != nil {
		c.AbortWithError(http.StatusInternalServerError, e)
	} else {
		defer r.Body.Close()
		c.DataFromReader(r.StatusCode, r.ContentLength, r.Header.Get("Content-Type"), r.Body, nil)
	}
}

func SetupRoute(r gin.IRouter) {
	authorized := r.Group("/", auth.CheckAuth())
	// MSX:
	authorized.GET("/msx/", func(c *gin.Context) {
		r, e := msxHTTPClient.Get("http://" + base)
		rsp(c, r, e)
	})
	authorized.GET("/msx/start.json", func(c *gin.Context) {
		paramMu.RLock()
		curParam := param
		paramMu.RUnlock()
		c.JSON(http.StatusOK, map[string]any{
			"name":      "TorrServer",
			"version":   version.Version,
			"parameter": curParam,
			"launcher": map[string]any{
				"type":  "start",
				"image": utils.GetScheme(c) + "://" + c.Request.Host + "/logo.png",
			},
		})
	})
	authorized.POST("/msx/start.json", func(c *gin.Context) {
		if settings.ReadOnly {
			c.Status(http.StatusForbidden)
			return
		}
		var next string
		if e := c.BindJSON(&next); e != nil {
			c.AbortWithError(http.StatusBadRequest, e)
			return
		}
		paramMu.Lock()
		param = next
		paramMu.Unlock()
	})
	authorized.GET("/msx/trn", func(c *gin.Context) {
		r := false
		if h := strings.ToLower(strings.TrimSpace(c.Query("hash"))); h != "" {
			r = settings.GetTorrent(h) != nil
		}
		c.JSON(http.StatusOK, r)
	})
	authorized.POST("/msx/trn", func(c *gin.Context) {
		var r struct {
			R struct {
				S int            `json:"status"`
				T string         `json:"text"`
				M string         `json:"message,omitempty"`
				D map[string]any `json:"data,omitempty"`
			} `json:"response"`
		}
		if j := struct{ Data string }{Data: c.Query("hash")}; j.Data != "" {
			st, sc := trn(j.Data)
			if sc != "" {
				sc = "{col:" + sc + "}"
			}
			r.R.S, r.R.D = http.StatusOK, map[string]any{"action": "player:label:position:{VALUE}{tb}{tb}" + sc + st}
		} else if e := c.BindJSON(&j); e != nil {
			r.R.S, r.R.M = http.StatusBadRequest, e.Error()
		} else if j.Data == "" {
			r.R.S, r.R.M = http.StatusBadRequest, "data is not set"
		} else {
			st, sc := trn(j.Data[strings.LastIndexByte(j.Data, ':')+1:])
			r.R.D = map[string]any{"stamp": st, "stampColor": sc}
			if sc != "" {
				r.R.D["live"] = map[string]any{
					"type": "airtime", "duration": 3000, "over": map[string]any{
						"action": "execute:" + utils.GetScheme(c) + "://" + c.Request.Host + c.Request.URL.Path, "data": j.Data,
					},
				}
			}
			r.R.S, r.R.D = http.StatusOK, map[string]any{"action": j.Data, "data": r.R.D}
		}
		r.R.T = http.StatusText(r.R.S)
		c.JSON(http.StatusOK, &r)
	})
	authorized.Any("/msx/proxy", func(c *gin.Context) {
		u := strings.TrimSpace(c.Query("url"))
		if u == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		parsed, err := url.Parse(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid proxy url"})
			return
		}
		q, e := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, parsed.String(), c.Request.Body)
		if e != nil {
			c.AbortWithError(http.StatusInternalServerError, e)
			return
		}
		for _, v := range c.QueryArray("header") {
			if kv := strings.SplitN(v, ":", 2); len(kv) == 2 {
				q.Header.Add(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
			}
		}
		r, e := msxHTTPClient.Do(q)
		rsp(c, r, e)
	})
	authorized.GET("/msx/imdb/:id", func(c *gin.Context) {
		i, j := strings.TrimPrefix(c.Param("id"), "/"), false
		if strings.Contains(i, "/") || strings.Contains(i, "..") {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if j = strings.HasSuffix(i, ".json"); !j {
			i += ".json"
		}
		if r, e := msxHTTPClient.Get("https://v2.sg.media-imdb.com/suggestion/h/" + url.PathEscape(i)); e != nil || r.StatusCode != http.StatusOK || j {
			rsp(c, r, e)
		} else {
			defer r.Body.Close()
			var payload struct {
				D []struct{ I struct{ ImageUrl string } }
			}
			if e = json.NewDecoder(r.Body).Decode(&payload); e != nil {
				c.AbortWithError(http.StatusInternalServerError, e)
			} else if len(payload.D) == 0 || payload.D[0].I.ImageUrl == "" {
				c.Status(http.StatusNotFound)
			} else {
				c.Redirect(http.StatusMovedPermanently, payload.D[0].I.ImageUrl)
			}
		}
	})
	// Files: serve static directory inside settings.Path/media only; arbitrary
	// symlink creation over HTTP is disabled to prevent host filesystem exposure.
	authorized.StaticFS("/files/", gin.Dir(filepath.Join(settings.Path, files), false))
	authorized.GET("/files", func(c *gin.Context) {
		if l, e := os.Readlink(filepath.Join(settings.Path, files)); e == nil || os.IsNotExist(e) {
			c.JSON(http.StatusOK, l)
		} else {
			c.JSON(http.StatusInternalServerError, e.Error())
		}
	})
	authorized.POST("/files", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "modifying host filesystem symlinks via HTTP is disabled",
		})
	})
}
