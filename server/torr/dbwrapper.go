package torr

import (
	"encoding/json"
	"strings"

	"server/settings"
	"server/torr/state"
	"server/torr/utils"
)

// tsFiles is the optional cached file list embedded inside TorrentDB.Data
// for clients that prefetch file metadata without fully loading the
// torrent into the session.
type tsFiles struct {
	TorrServer struct {
		Files []*state.TorrentFileStat `json:"Files"`
	} `json:"TorrServer"`
}

// fileStatsFromData recovers the cached file list embedded in a DB record's
// Data field (tsFiles JSON, written by AddTorrentDB). Returns nil when Data is
// empty or carries no file cache. Lets consumers that only need the file list
// (playlists, the web UI file tree) serve a DB-resident torrent without waking
// it in the session — a magnet-only record would otherwise block on a swarm
// metadata fetch.
func fileStatsFromData(data string) []*state.TorrentFileStat {
	if data == "" {
		return nil
	}
	var f tsFiles
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		return nil
	}
	return f.TorrServer.Files
}

// AddTorrentDB serialises a torrent record into the persistent DB.
func AddTorrentDB(torr *Torrent) {
	if torr == nil {
		return
	}
	torr.mu.Lock()
	var specCopy *TorrentSpec
	if torr.TorrentSpec != nil {
		specCopy = torr.TorrentSpec.Clone()
	}
	title := torr.Title
	category := torr.Category
	data := torr.Data
	poster := torr.Poster
	size := torr.Size
	timestamp := torr.Timestamp
	torr.mu.Unlock()

	t := new(settings.TorrentDB)
	t.TorrentSpec = settingsSpec(specCopy)
	t.Title = title
	t.Category = category
	if data == "" {
		var f tsFiles
		f.TorrServer.Files = torr.Status().FileStats
		if buf, err := json.Marshal(&f); err == nil {
			t.Data = string(buf)
			torr.mu.Lock()
			if torr.Data == "" {
				torr.Data = t.Data
			}
			torr.mu.Unlock()
		}
	} else {
		t.Data = data
	}
	// Keep the stored poster when the new one cannot be verified: a host that
	// answers slowly (or at all only sometimes) must not wipe a working poster.
	t.Poster = poster
	if t.Poster != "" {
		var existing string
		if db := GetTorrentDB(torr.Hash()); db != nil {
			existing = db.Poster
		}
		if existing != t.Poster {
			ok, verified := utils.CheckImgUrl(t.Poster)
			if !ok || (existing != "" && !verified) {
				t.Poster = existing
			}
		}
	}
	t.Size = size
	if t.Size == 0 {
		t.Size = torr.Length()
	}
	t.Timestamp = timestamp
	settings.AddTorrent(t)
}

// GetTorrentDB looks up a single torrent record by info hash using direct O(1)
// key lookup rather than scanning and unmarshalling the entire bucket.
func GetTorrentDB(hash Hash) *Torrent {
	if hash.IsZero() {
		return nil
	}
	hexLower := hash.HexString()
	if db := settings.GetTorrent(hexLower); db != nil {
		return torrentFromDB(db)
	}
	if db := settings.GetTorrent(strings.ToUpper(hexLower)); db != nil {
		return torrentFromDB(db)
	}
	return nil
}

// RemTorrentDB removes a torrent record by info hash.
func RemTorrentDB(hash Hash) {
	if hash.IsZero() {
		return
	}
	settings.RemTorrent(hash.HexString())
}

// ListTorrentsDB returns the full DB content keyed by info hash.
func ListTorrentsDB() map[Hash]*Torrent {
	out := make(map[Hash]*Torrent)
	for _, db := range settings.ListTorrent() {
		if db.TorrentSpec == nil {
			continue
		}
		h, ok := ParseHash(db.TorrentSpec.InfoHash)
		if !ok {
			continue
		}
		out[h] = torrentFromDB(db)
	}
	return out
}

func torrentFromDB(db *settings.TorrentDB) *Torrent {
	if db == nil {
		return nil
	}
	t := new(Torrent)
	t.TorrentSpec = torrSpec(db.TorrentSpec)
	t.Title = db.Title
	t.Poster = db.Poster
	t.Category = db.Category
	t.Timestamp = db.Timestamp
	t.Size = db.Size
	t.Data = db.Data
	t.Stat = state.TorrentInDB
	return t
}

func settingsSpec(s *TorrentSpec) *settings.TorrentSpec {
	if s == nil {
		return nil
	}
	return &settings.TorrentSpec{
		InfoHash:    s.InfoHash.HexString(),
		InfoBytes:   s.InfoBytes,
		Trackers:    s.Trackers,
		DisplayName: s.DisplayName,
	}
}

func torrSpec(s *settings.TorrentSpec) *TorrentSpec {
	if s == nil {
		return nil
	}
	return &TorrentSpec{
		InfoHash:    NewHashFromHex(s.InfoHash),
		InfoBytes:   s.InfoBytes,
		Trackers:    s.Trackers,
		DisplayName: s.DisplayName,
	}
}
