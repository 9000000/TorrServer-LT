// Package utils is a thin alias layer for callers that historically
// reached into web/api/utils for ParseLink-style helpers. The actual
// parsing now lives in server/torr, on top of the libtorrent shim.
//
// Kept so web handlers, settings migration and tgbot don't need to be
// aware of the package boundary moving.
package utils

import (
	"errors"
	"mime/multipart"
	"strings"

	"server/torr"
	"server/torrshash"
)

// ParseLink dispatches a string (magnet/url/hash/torrs token) to the
// appropriate parser, rejecting local file:// URIs on network-facing paths.
func ParseLink(link string) (*torr.TorrentSpec, error) {
	trimmed := strings.TrimSpace(link)
	if strings.HasPrefix(strings.ToLower(trimmed), "file:") {
		return nil, errors.New("utils.ParseLink: file:// scheme is not allowed")
	}
	return torr.ParseLink(trimmed)
}

// ParseFile reads a multipart upload and returns the spec.
func ParseFile(file multipart.File) (*torr.TorrentSpec, error) {
	return torr.ParseReader(file)
}

// ParseFromBytes parses an in-memory .torrent payload.
func ParseFromBytes(data []byte) (*torr.TorrentSpec, error) {
	return torr.ParseBytes(data)
}

// ParseTorrsHash decodes a torrs:// (or bare base62) token.
func ParseTorrsHash(token string) (*torr.TorrentSpec, *torrshash.TorrsHash, error) {
	return torr.ParseTorrsHash(token)
}
