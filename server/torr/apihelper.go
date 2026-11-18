package torr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"server/log"
	"server/lt"
	sets "server/settings"
	"server/torr/utils"
	"server/version"
)

// bts is the package-level engine handle initialised by BTServer.Connect.
var bts *BTServer

// InitApiHelper is called by BTServer.Connect to publish the engine
// instance to the rest of this package.
// InitApiHelper publishes the engine handle the API helpers use. Request
// handlers read bts without synchronisation of their own, so it must be
// written only when it actually changes: Connect calls this on every
// settings save with the same receiver, and the repeated assignment is a
// data race against every in-flight request.
func InitApiHelper(bt *BTServer) {
	if bts == bt {
		return
	}
	bts = bt
}

// LoadTorrent re-adds a DB-only torrent into the running session.
func LoadTorrent(tor *Torrent) *Torrent {
	if tor == nil || tor.TorrentSpec == nil || bts == nil {
		return nil
	}
	tor.mu.Lock()
	spec := tor.TorrentSpec.Clone()
	title := tor.Title
	poster := tor.Poster
	data := tor.Data
	category := tor.Category
	ts := tor.Timestamp
	size := tor.Size
	tor.mu.Unlock()

	hadInfo := len(spec.InfoBytes) > 0
	out, err := NewTorrent(spec, bts)
	if err != nil {
		log.TLogln("torr.LoadTorrent:", err)
		return nil
	}
	if !out.WaitInfo() {
		return nil
	}
	out.mu.Lock()
	if out.Title == "" {
		out.Title = title
	}
	if out.Poster == "" {
		out.Poster = poster
	}
	if out.Data == "" {
		out.Data = data
	}
	if out.Category == "" {
		out.Category = category
	}
	if ts != 0 {
		out.Timestamp = ts
	}
	if size > 0 && out.Size == 0 {
		out.Size = size
	}
	hasBackfilledInfo := !hadInfo && out.TorrentSpec != nil && len(out.TorrentSpec.InfoBytes) > 0
	out.mu.Unlock()
	// A magnet-only DB record just produced its info-dict (backfilled into the
	// spec by signalGotInfo): persist it so the next server start serves this
	// torrent instantly instead of re-fetching metadata from the swarm.
	if hasBackfilledInfo && GetTorrentDB(out.Hash()) != nil {
		AddTorrentDB(out)
	}
	return out
}

// AddTorrent installs a torrent (creating or re-using the in-memory
// handle), backfilling user metadata from the DB if needed.
func AddTorrent(spec *TorrentSpec, title, poster, data, category string) (*Torrent, error) {
	if spec == nil || spec.InfoHash.IsZero() {
		return nil, errors.New("torr.AddTorrent: invalid torrent spec")
	}
	spec = spec.Clone()
	dbt := GetTorrentDB(spec.InfoHash)

	// A spec parsed from a request link is usually hash-only; if the DB record
	// carries the info-dict, add with it so the torrent comes up with metadata
	// instantly instead of re-fetching it from the swarm (slow first play /
	// playlist after restart).
	if dbt != nil && dbt.TorrentSpec != nil && len(spec.InfoBytes) == 0 && len(dbt.TorrentSpec.InfoBytes) > 0 {
		spec.InfoBytes = append([]byte(nil), dbt.TorrentSpec.InfoBytes...)
	}

	t, err := NewTorrent(spec, bts)
	if err != nil {
		log.TLogln("torr.AddTorrent:", err)
		return nil, err
	}

	fallbackName := ""
	if t.handle() != nil {
		fallbackName = t.Name()
	}

	t.mu.Lock()
	if t.Title == "" {
		t.Title = title
		if title == "" && dbt != nil {
			t.Title = dbt.Title
		}
		if t.Title == "" {
			t.Title = fallbackName
		}
	}
	if t.Category == "" {
		t.Category = category
		if category == "" && dbt != nil {
			t.Category = dbt.Category
		}
	}
	if t.Poster == "" {
		t.Poster = poster
		if poster == "" && dbt != nil {
			t.Poster = dbt.Poster
		}
	}
	if t.Data == "" {
		t.Data = data
		if data == "" && dbt != nil {
			t.Data = dbt.Data
		}
	}
	t.mu.Unlock()
	return t, nil
}

// SaveTorrentToDB persists a torrent record into config.db / json.
func SaveTorrentToDB(torr *Torrent) {
	if torr == nil {
		return
	}
	log.TLogln("torr.SaveTorrentToDB:", torr.Hash().HexString())
	AddTorrentDB(torr)
}

// GetTorrentInfo returns a torrent for read-only display — the live session
// instance if present, otherwise the DB record (cheap direct lookup) — WITHOUT
// promoting it into the session or extending its auto-drop deadline. For
// status/observation endpoints (e.g. the /cache piece-map dialog, which polls
// every 100ms): such polling must not keep a torrent resident after playback
// ends, but must still be able to render a dropped torrent's info (with an empty
// cache map) rather than 404. Only actual playback (an active reader) keeps a
// torrent alive. nil only when the hash is unknown to both session and DB.
func GetTorrentInfo(hashHex string) *Torrent {
	hash, ok := ParseHash(hashHex)
	if !ok {
		return nil
	}
	if bts != nil {
		if t := bts.GetTorrent(hash); t != nil {
			return t
		}
	}
	if dbt := GetTorrentDB(hash); dbt != nil {
		return dbt
	}
	return nil
}

// GetTorrent returns the in-memory torrent if present, or revives a
// DB-only one (asynchronously promoted to a live session torrent).
func GetTorrent(hashHex string) *Torrent {
	hash, ok := ParseHash(hashHex)
	if !ok {
		return nil
	}
	timeout := torrentExpireTimeout()

	if bts != nil {
		if tor := bts.GetTorrent(hash); tor != nil {
			tor.AddExpiredTime(timeout)
			return tor
		}
	}
	dbt := GetTorrentDB(hash)
	if dbt == nil {
		return nil
	}
	if bts == nil {
		return dbt
	}
	tor := dbt
	specCopy := tor.TorrentSpec.Clone()
	title, poster, data, category := tor.Title, tor.Poster, tor.Data, tor.Category
	size, ts := tor.Size, tor.Timestamp
	go func() {
		log.TLogln("torr.GetTorrent: promoting DB torrent", hash.HexString())
		hadInfo := specCopy != nil && len(specCopy.InfoBytes) > 0
		fresh, err := NewTorrent(specCopy, bts)
		if err != nil || fresh == nil {
			log.TLogln("torr.GetTorrent: promote failed:", hash.HexString(), err)
			return
		}
		fresh.mu.Lock()
		if fresh.Title == "" {
			fresh.Title = title
		}
		if fresh.Poster == "" {
			fresh.Poster = poster
		}
		if fresh.Data == "" {
			fresh.Data = data
		}
		if fresh.Size == 0 {
			fresh.Size = size
		}
		if ts != 0 {
			fresh.Timestamp = ts
		}
		if fresh.Category == "" {
			fresh.Category = category
		}
		fresh.mu.Unlock()
		if fresh.GotInfo() && !hadInfo {
			fresh.mu.Lock()
			hasInfo := fresh.TorrentSpec != nil && len(fresh.TorrentSpec.InfoBytes) > 0
			fresh.mu.Unlock()
			if hasInfo {
				// Magnet-only record just got its info-dict — persist it so future
				// server starts don't re-fetch metadata from the swarm.
				AddTorrentDB(fresh)
			}
		}
	}()
	return tor
}

// SetTorrent updates the in-memory and DB-side metadata of a torrent.
func SetTorrent(hashHex, title, poster, category, data string) *Torrent {
	hash, ok := ParseHash(hashHex)
	if !ok {
		return nil
	}
	var tor *Torrent
	if bts != nil {
		tor = bts.GetTorrent(hash)
	}
	dbt := GetTorrentDB(hash)

	if title == "" && tor == nil && dbt != nil {
		tor = GetTorrent(hash.HexString())
		if tor != nil {
			tor.GotInfo()
			if tor.handle() != nil {
				title = tor.Name()
			}
		}
	}

	if tor != nil {
		if title == "" && tor.handle() != nil {
			title = tor.Name()
		}
		tor.mu.Lock()
		tor.Title = title
		tor.Poster = poster
		tor.Category = category
		if data != "" {
			tor.Data = data
		}
		tor.mu.Unlock()
	}
	if dbt != nil {
		dbt.Title = title
		dbt.Poster = poster
		dbt.Category = category
		if data != "" {
			dbt.Data = data
		}
		AddTorrentDB(dbt)
	}
	if tor != nil {
		return tor
	}
	return dbt
}

// RemTorrent removes a torrent from memory, the DB and (when configured)
// the on-disk cache directory. Strict 40-char hex hash validation prevents
// arbitrary directory traversal under TorrentsSavePath.
func RemTorrent(hashHex string) {
	if sets.ReadOnly {
		log.TLogln("torr.RemTorrent: read-only DB mode:", hashHex)
		return
	}
	hash, ok := ParseHash(hashHex)
	if !ok {
		return
	}
	hexHash := hash.HexString()

	var tor *Torrent
	if bts != nil {
		tor = bts.GetTorrent(hash)
	}
	if tor == nil {
		RemTorrentDB(hash)
		if s := sets.BTsets(); s != nil && s.UseDisk && s.TorrentsSavePath != "" {
			os.RemoveAll(filepath.Join(s.TorrentsSavePath, hexHash))
		}
		return
	}

	closeCh := tor.closeCh
	if bts != nil && bts.RemoveTorrent(hash) {
		select {
		case <-closeCh:
		case <-time.After(5 * time.Second):
			log.TLogln("torr.RemTorrent: timeout waiting for close:", hexHash)
		}
		if s := sets.BTsets(); s != nil && s.UseDisk && s.TorrentsSavePath != "" {
			name := filepath.Join(s.TorrentsSavePath, hexHash)
			if _, err := os.Stat(name); err == nil {
				log.TLogln("torr.RemTorrent: removing cache files for", hexHash)
				os.RemoveAll(name)
			}
		}
	}
	RemTorrentDB(hash)
}

// ListTorrent merges in-memory torrents with DB-only records.
func ListTorrent() []*Torrent {
	var live map[Hash]*Torrent
	if bts != nil {
		live = bts.ListTorrents()
	} else {
		live = make(map[Hash]*Torrent)
	}
	dbm := ListTorrentsDB()
	for h, t := range dbm {
		if _, ok := live[h]; !ok {
			live[h] = t
		}
	}
	type item struct {
		tor   *Torrent
		ts    int64
		title string
	}
	items := make([]item, 0, len(live))
	for _, t := range live {
		if t == nil {
			continue
		}
		t.mu.Lock()
		items = append(items, item{tor: t, ts: t.Timestamp, title: t.Title})
		t.mu.Unlock()
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ts != items[j].ts {
			return items[i].ts > items[j].ts
		}
		return items[i].title > items[j].title
	})
	out := make([]*Torrent, len(items))
	for i := range items {
		out[i] = items[i].tor
	}
	return out
}

// DropTorrent removes from the running session without touching the DB.
func DropTorrent(hashHex string) {
	hash, ok := ParseHash(hashHex)
	if !ok || bts == nil {
		return
	}
	bts.RemoveTorrent(hash)
}

// SetSettings applies a new settings_pack and bounces the session.
func SetSettings(set *sets.BTSets) {
	if sets.ReadOnly {
		log.TLogln("torr.SetSettings: read-only DB mode")
		return
	}
	// The storage-backend preferences (json vs config.db for Settings/Viewed)
	// are owned exclusively by the /storage/settings switch endpoint, which
	// migrates the data when they change. A regular settings save must not
	// touch them: the web settings dialog echoes back a StoreSettingsInJson it
	// cached before the switch, which would otherwise overwrite the chosen
	// backend and silently revert it to json on the next restart. Preserve the
	// live values.
	trackersChanged := true
	if set != nil {
		if cur := sets.BTsets(); cur != nil {
			set.StoreSettingsInJson = cur.StoreSettingsInJson
			set.StoreViewedInJson = cur.StoreViewedInJson
			trackersChanged = set.TrackersListURL != cur.TrackersListURL || set.DefaultTrackers != cur.DefaultTrackers
		}
	}
	sets.SetBTSets(set)
	if trackersChanged {
		utils.InvalidateTrackersCache()
	}
	if bts == nil {
		return
	}
	log.TLogln("torr.SetSettings: dropping all torrents")
	dropAllTorrent()
	time.Sleep(time.Second)
	log.TLogln("torr.SetSettings: disconnect")
	bts.Disconnect()
	log.TLogln("torr.SetSettings: reconnect")
	reconnect("torr.SetSettings")
}

// reconnect retries Connect after a Disconnect. A single failed attempt (the
// peer port of the closed session still bound, a transient bind error) would
// otherwise leave the engine without a session until the server is restarted:
// every add then answers "BT client not connected".
func reconnect(who string) {
	if bts == nil {
		return
	}
	var err error
	for i := 0; i < 30; i++ {
		if err = bts.Connect(); err == nil {
			if i > 0 {
				log.TLogln(who+": connect ok, attempt", i+1)
			}
			return
		}
		if i == 0 {
			log.TLogln(who+": connect error, retrying:", err)
		}
		time.Sleep(time.Second)
	}
	log.TLogln(who+": connect failed:", err)
}

// SetDefSettings resets settings to defaults and bounces the session.
func SetDefSettings() {
	if sets.ReadOnly {
		log.TLogln("torr.SetDefSettings: read-only DB mode")
		return
	}
	sets.SetDefaultConfig()
	utils.InvalidateTrackersCache()
	if bts == nil {
		return
	}
	log.TLogln("torr.SetDefSettings: dropping all torrents")
	dropAllTorrent()
	time.Sleep(time.Second)
	bts.Disconnect()
	reconnect("torr.SetDefSettings")
}

func dropAllTorrent() {
	if bts == nil {
		return
	}
	for _, t := range bts.ListTorrents() {
		t.markClosed()
		if t.handle() != nil {
			_ = t.handle().Remove(false)
		}
	}
}

// Shutdown closes the engine and the DB, then exits the process. Teardown is
// bounded: libtorrent session destruction can stall on tracker/DHT stop
// announces, and a wedged teardown must not leave a half-dead server that
// still answers HTTP but can never be stopped via the API.
func Shutdown() {
	done := make(chan struct{})
	go func() {
		if bts != nil {
			bts.Disconnect()
		}
		sets.CloseDB()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		log.TLogln("torr.Shutdown: teardown timed out — forcing exit")
	}
	log.TLogln("torr.Shutdown: received shutdown — quit")
	os.Exit(0)
}

// WriteStatus dumps server, per-torrent and libtorrent session stats to an
// io.Writer (the /stat page). Session counters are a fresh session_stats
// snapshot fetched through the alert pump (bounded wait, so /stat cannot
// hang); torrent details come from the same state the web UI uses.
func WriteStatus(w io.Writer) {
	if bts == nil || bts.Session() == nil {
		w.Write([]byte("session not running\n"))
		return
	}

	fmt.Fprintf(w, "TorrServer-LT %s (libtorrent %s)\n", version.Version, lt.Version())

	live := bts.ListTorrents()
	fmt.Fprintf(w, "Torrents: %d in session, %d in DB\n", len(live), len(ListTorrentsDB()))

	hashes := make([]Hash, 0, len(live))
	tsByHash := make(map[Hash]int64, len(live))
	for h, t := range live {
		hashes = append(hashes, h)
		if t != nil {
			t.mu.Lock()
			tsByHash[h] = t.Timestamp
			t.mu.Unlock()
		}
	}
	sort.Slice(hashes, func(i, j int) bool {
		return tsByHash[hashes[i]] < tsByHash[hashes[j]]
	})
	for _, h := range hashes {
		t := live[h]
		st := t.Status()
		title := st.Title
		if title == "" {
			title = st.Name
		}
		fmt.Fprintf(w, "\n%s  %s\n", h.HexString(), title)
		fmt.Fprintf(w, "  status: %s, peers: %d/%d (active/known), seeders: %d\n",
			st.StatString, st.ActivePeers, st.TotalPeers, st.ConnectedSeeders)
		fmt.Fprintf(w, "  speed down/up: %.1f / %.1f KB/s, size: %d, preloaded: %d/%d\n",
			st.DownloadSpeed/1024, st.UploadSpeed/1024,
			st.TorrentSize, st.PreloadedBytes, st.PreloadSize)
		if cs := t.CacheState(); cs != nil {
			fmt.Fprintf(w, "  cache: %d/%d bytes (%d pieces), readers: %d\n",
				cs.Filled, cs.Capacity, cs.PiecesCount, len(cs.Readers))
		}
	}

	counters := bts.SessionStats(2 * time.Second)
	if len(counters) == 0 {
		w.Write([]byte("\nlibtorrent session counters: not available\n"))
		return
	}
	w.Write([]byte("\nlibtorrent session counters:\n"))
	names := make([]string, 0, len(counters))
	for name := range counters {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "  %s: %d\n", name, counters[name])
	}
}
