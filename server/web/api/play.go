package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	sets "server/settings"
	"server/torr"
	"server/torr/state"
	"server/web/api/utils"
)

// play godoc
//
//	@Summary		Play given torrent by infohash
//	@Description	Play given torrent referenced by infohash and file id.
//
//	@Tags			API
//
//	@Param			hash		path	string	true	"Torrent infohash"
//	@Param			id			path	string	true	"File index in torrent"
//
//	@Produce		application/octet-stream
//	@Success		200	"Torrent data"
//	@Router			/play/{hash}/{id} [get]
func play(c *gin.Context) {
	hash := c.Param("hash")
	indexStr := c.Param("id")
	notAuth := c.GetBool("auth_required") && c.GetString(gin.AuthUserKey) == ""

	if hash == "" || indexStr == "" {
		c.AbortWithError(http.StatusNotFound, errors.New("no infohash or file index in link"))
		return
	}

	spec, err := utils.ParseLink(hash)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	tor := torr.GetTorrent(spec.InfoHash.HexString())
	if tor == nil && notAuth {
		// StreamWA: let external players play an unknown torrent without auth by
		// auto-adding it (link is a magnet/hash — TestLink rejects remote URLs).
		if !sets.StreamWA {
			c.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		tor, err = torr.AddTorrent(spec, "", "", "", "")
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if tor == nil {
		c.AbortWithError(http.StatusInternalServerError, errors.New("error get torrent"))
		return
	}

	if tor.Stat == state.TorrentInDB {
		tor, err = torr.AddTorrent(spec, tor.Title, tor.Poster, tor.Data, tor.Category)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if !tor.GotInfo() {
		c.AbortWithError(http.StatusInternalServerError, errors.New("torrent connection timeout"))
		return
	}

	// find file
	index := -1
	catLower := strings.ToLower(tor.Category)
	if strings.Contains(catLower, "movie") {
		index = 1
	} else if ind, err := strconv.Atoi(indexStr); err == nil {
		index = ind
	} else if len(tor.Files()) == 1 {
		if st := tor.Status(); len(st.FileStats) == 1 {
			index = st.FileStats[0].Id
		} else {
			index = 1
		}
	}
	if index == -1 { // if file index not set and play file exec
		c.AbortWithError(http.StatusBadRequest, errors.New("file \"index\" is wrong"))
		return
	}

	tor.Stream(index, c.Request, c.Writer)
}
