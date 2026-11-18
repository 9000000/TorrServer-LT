package utils

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"

	_ "golang.org/x/image/webp"

	"server/log"
)

const (
	maxPosterBytes  = 2 * 1024 * 1024
	maxPosterPixels = 16_000_000 // 16 MP cap prevents decompression-bomb OOM
)

var imgHTTPClient = &http.Client{
	Timeout: 6 * time.Second,
}

// CheckImgUrl reports whether link may be stored.
// verified is true only when a real image body was decoded; a timeout or
// transport error is ok but not verified, so a caller with an existing poster
// should keep it.
func CheckImgUrl(link string) (ok, verified bool) {
	if link == "" {
		return false, false
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", link, nil)
	if err != nil {
		log.TLogln("Error create request for image:", err)
		return false, false
	}
	// Some image hosts (fastpic.org) answer 404 to the default "Go-http-client" agent
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := imgHTTPClient.Do(req)
	if err != nil {
		log.TLogln("Error check image:", err)
		return true, false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, false
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxPosterBytes))
	if err != nil {
		log.TLogln("Error read image:", err)
		if ctx.Err() != nil {
			return true, false
		}
		return false, false
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		log.TLogln("Error decode image config:", err)
		if ctx.Err() != nil {
			return true, false
		}
		return false, false
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPosterPixels {
		log.TLogln("Image dimensions out of bounds:", cfg.Width, "x", cfg.Height)
		return false, false
	}

	if _, _, err = image.Decode(bytes.NewReader(buf)); err != nil {
		log.TLogln("Error decode image:", err)
		if ctx.Err() != nil {
			return true, false
		}
		return false, false
	}
	return true, true
}
