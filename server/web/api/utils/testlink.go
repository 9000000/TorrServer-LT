package utils

import (
	"fmt"
	"net/url"
	"strings"

	"server/torrshash"
)

func isHex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func TestLink(link string, auth bool) error {
	if unescaped, err := url.QueryUnescape(link); err == nil {
		link = unescaped
	}
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Errorf("empty link")
	}

	lower := strings.ToLower(link)
	if strings.HasPrefix(lower, "file:") {
		return fmt.Errorf("file scheme is not allowed")
	}

	// Unauthenticated callers may only pass info-hashes, magnets, or torrs tokens
	// (never arbitrary HTTP/HTTPS URLs, preventing unauthenticated SSRF).
	if strings.HasPrefix(lower, "magnet:") ||
		strings.HasPrefix(lower, "torrs://") ||
		(len(link) > 45 && torrshash.IsBase62(link)) ||
		isHex40(link) {
		return nil
	}

	if !auth {
		return fmt.Errorf("auth required")
	}

	ur, err := url.Parse(link)
	if err != nil {
		return err
	}
	scheme := strings.ToLower(ur.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported link scheme")
	}
	return nil
}
