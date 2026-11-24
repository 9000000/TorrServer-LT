package torr

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"server/log"
	"server/lt"
	"server/settings"
	"server/torr/state"
	storageState "server/torr/storage/state"
	"server/torr/storage/torrstor"
	"server/torr/utils"
	"server/torrshash"
	utils2 "server/utils"
)

// Torrent is the public engine-agnostic wrapper. Composes a libtorrent
// handle with the metadata (title/poster/category) the rest of the
// project decorates torrents with.
type Torrent struct {
	Title    string
	Category string
	Poster   string
	Data     string
	*TorrentSpec

	Stat      state.TorrentStat
	Timestamp int64
	Size      int64

	bt *BTServer
	// lh is the libtorrent handle (nil while the torrent is in DB only). Close
	// clears it while readers all over the package are using it, so it is an
	// atomic pointer rather than a plain field: a mutex here would deadlock the
	// paths that already hold t.mu (Status, for one).
	lh atomic.Pointer[lt.Torrent]

	mu sync.Mutex

	// runtime metrics, updated by watch()
	lastTimeSpeed       time.Time
	DownloadSpeed       float64
	UploadSpeed         float64
	BytesReadUsefulData int64
	BytesWrittenData    int64

	// counters driven by the alert pump (atomic-friendly under mu)
	piecesDirtiedGood int64
	piecesDirtiedBad  int64

	PreloadSize    int64
	PreloadedBytes int64

	// play-gate state. The fill itself runs detached (context.Background) so an
	// impatient external player disconnecting mid-buffer can't abort it; the gate
	// tracks which file is filling and a channel that closes when it finishes, so
	// a reconnect for the SAME file WAITS for that same fill instead of skipping
	// the gate and streaming a half-buffer. preloadGateLast keeps the post-fill
	// debounce that suppresses the per-file preload storm a playlist client
	// triggers when it walks every &play entry of a multi-file torrent.
	preloadGateMu    sync.Mutex
	preloadGateIndex int
	preloadGateDone  chan struct{}
	preloadGateLast  time.Time

	// playStarted records that this torrent has begun playback once (on playStartIndex),
	// so the start-preload fires only at torrent START, not on every series switch: a
	// multi-file torrent is treated as a single unit. A play for a DIFFERENT file once
	// started streams directly (the reader's window buffers it), matching upstream
	// TorrServer, which preloads only on the explicit &preload — never on a play. Guarded
	// by preloadGateMu; reset implicitly when the torrent is dropped (fresh struct).
	playStarted    bool
	playStartIndex int

	DurationSeconds float64
	BitRate         string

	expiredTime time.Time

	gotInfoCh   chan struct{}
	gotInfoOnce sync.Once
	posterOnce  sync.Once

	closeCh   chan struct{}
	closeOnce sync.Once

	watcher *time.Ticker
}

// handle returns the libtorrent handle, or nil once the torrent is closed.
func (t *Torrent) handle() *lt.Torrent {
	if t == nil {
		return nil
	}
	return t.lh.Load()
}

// NewTorrent installs a new torrent in the session.
func NewTorrent(spec *TorrentSpec, bt *BTServer) (*Torrent, error) {
	if bt == nil || bt.Session() == nil {
		return nil, errors.New("torr.NewTorrent: BT client not connected")
	}
	if spec == nil {
		return nil, errors.New("torr.NewTorrent: nil spec")
	}

	// Deep-copy spec so mutating Trackers/InfoBytes never races with shared DB
	// or caller structs.
	spec = spec.Clone()

	// If metadata is known at add time (InfoBytes present), parse piece
	// geometry, private-flag, and on-disk resume bits up-front.
	var (
		havePieces []byte
		pieceCount int
		totalSize  int64
		isPrivate  bool
	)
	if len(spec.InfoBytes) > 0 {
		if pt, err := lt.ParseTorrentBytes(spec.InfoBytes); err == nil {
			isPrivate = pt.Private
			if pt.HasMetadata && pt.NumPieces > 0 {
				pieceCount = pt.NumPieces
				totalSize = pt.TotalSize
				havePieces = torrstor.ScanHavePieces(spec.InfoHash, pt.NumPieces, pt.PieceLength, pt.TotalSize)
			}
		}
	}

	// Auto-save incoming trackers from public torrents/magnets into trackers.txt.
	// Private torrents are excluded so user passkeys never leak to trackers.txt.
	if !isPrivate && len(spec.Trackers) > 0 {
		var incoming []string
		for _, tier := range spec.Trackers {
			incoming = append(incoming, tier...)
		}
		if len(incoming) > 0 {
			utils.SaveTrackersToFile(incoming)
		}
	}

	// Trackers: applied via session-level retrackers settings in addition
	// to per-torrent. Mirror legacy RetrackersMode semantics, except private
	// torrents keep their announce list untouched per BEP-27.
	if !isPrivate {
		defTrackers := utils.GetDefTrackers()
		fileTrackers := utils.GetTrackerFromFile()
		switch settings.BTsets().RetrackersMode {
		case 1:
			spec.Trackers = append(spec.Trackers, defTrackers)
		case 2:
			spec.Trackers = nil
		case 3:
			spec.Trackers = [][]string{defTrackers}
		}
		if len(fileTrackers) > 0 {
			spec.Trackers = append(spec.Trackers, fileTrackers)
		}
	}

	// Hold the registry lock across check + session add + register. The
	// check-then-register used to be two separate critical sections, so two
	// concurrent adds of the same hash (e.g. an HTTP handler re-adding an
	// idle-dropped torrent racing GetTorrent's async promotion) both passed
	// the check and produced two Go Torrents over one libtorrent torrent.
	// The loser was orphaned — alerts route by hash to the registered
	// instance only — so its GotInfo timed out 90s later and Close()d the
	// shared lt torrent out from under an active stream.
	bt.mu.Lock()
	if existing := bt.torrents[spec.InfoHash]; existing != nil {
		existing.mu.Lock()
		closed := existing.Stat == state.TorrentClosed
		existing.mu.Unlock()
		if !closed {
			bt.mu.Unlock()
			return existing, nil
		}
		// A closed instance left in the registry (e.g. a magnet whose metadata
		// fetch timed out: GotInfo's failure path Close()s without deregistering,
		// and expired() skips TorrentClosed so expireWatch never reaps it) must
		// not shadow the hash forever — every re-add would get the zombie back
		// and insta-fail. Its lt torrent is already removed; replace the entry.
		delete(bt.torrents, spec.InfoHash)
	}

	// Re-check under the lock: the session may have been torn down between
	// the check above and here (a settings save disconnects and reconnects),
	// and AddTorrent on a nil session panics.
	if bt.session == nil {
		bt.mu.Unlock()
		return nil, errors.New("torr.NewTorrent: BT client not connected")
	}

	lh, err := bt.session.AddTorrent(lt.AddTorrentParams{
		Link:                magnetFromSpec(spec),
		InfoBytes:           spec.InfoBytes,
		Trackers:            spec.FlatTrackers(),
		SavePath:            legacySavePath(spec.InfoHash),
		Paused:              false,
		DefaultDontDownload: true,
		HavePieces:          havePieces,
		PieceCount:          pieceCount,
	})
	if err != nil {
		bt.mu.Unlock()
		return nil, err
	}

	if totalSize > 0 {
		if c := torrstor.Global().CacheByHash([20]byte(spec.InfoHash)); c != nil {
			c.SetTotalSize(totalSize)
		}
	}

	timeout := torrentExpireTimeout()
	t := &Torrent{
		TorrentSpec:   spec,
		bt:            bt,
		Stat:          state.TorrentAdded,
		Timestamp:     time.Now().Unix(),
		lastTimeSpeed: time.Now(),
		gotInfoCh:     make(chan struct{}),
		closeCh:       make(chan struct{}),
	}
	t.lh.Store(lh)
	bt.torrents[spec.InfoHash] = t
	bt.mu.Unlock()

	t.AddExpiredTime(timeout)

	// BTsets.ConnectionsLimit is a per-torrent peer cap (anacrolix legacy);
	// the session-wide connections_limit is set separately in
	// buildSessionConfig. See lt_shim.h on lt_torrent_set_max_connections.
	if lh != nil && settings.BTsets() != nil && settings.BTsets().ConnectionsLimit > 0 {
		_ = lh.SetMaxConnections(settings.BTsets().ConnectionsLimit)
	}

	// If the .torrent payload was provided up-front, metadata is already
	// known — signal immediately. Otherwise this is a magnet and we must pull
	// the info-dict from peers before anything (file list, playlist, stream)
	// can proceed; kick an immediate DHT + tracker announce so peer discovery
	// for the metadata starts aggressively right away instead of waiting for a
	// Reader to attach (which only happens at playback start, long after the
	// playlist is requested). This is the difference between a magnet whose
	// playlist appears in a couple of seconds and one that times out.
	if len(spec.InfoBytes) > 0 {
		t.signalGotInfo()
	} else if lh != nil {
		go func() {
			_ = lh.ForceReannounce()
			if settings.BTsets() == nil || !settings.BTsets().DisableDHT {
				_ = lh.ForceDhtAnnounce()
			}
		}()
	}

	go t.watch()
	return t, nil
}

func magnetFromSpec(spec *TorrentSpec) string {
	if len(spec.InfoBytes) > 0 {
		return ""
	}
	return "magnet:?xt=urn:btih:" + spec.InfoHash.HexString()
}

func torrentExpireTimeout() time.Duration {
	t := time.Second * time.Duration(settings.BTsets().TorrentDisconnectTimeout)
	if t > time.Minute {
		t = time.Minute
	}
	return t
}

func legacySavePath(h Hash) string {
	if settings.BTsets() != nil && settings.BTsets().UseDisk && settings.BTsets().TorrentsSavePath != "" {
		return settings.BTsets().TorrentsSavePath + "/" + h.HexString()
	}
	return ""
}

// ----- metadata ready signalling -----

func (t *Torrent) signalGotInfo() {
	// Only fire once metadata is actually parsed. The add_torrent alert (and a
	// magnet's early alerts) arrive BEFORE the info-dict is known; consuming the
	// once here would burn it on a no-op SetAllPiecesPriority and then make the
	// real metadata_received alert a no-op — leaving the torrent at libtorrent's
	// default piece priority 1, which downloads the WHOLE torrent and thrashes
	// the bounded cache. So bail until metadata is ready and wait for a later
	// alert (metadata_received / torrent_finished) or WaitInfo's fast path.
	//
	// Snapshot the handle ONCE: Close() nils t.handle() with no synchronization, and
	// an instance that loses a concurrent add race (remove → immediate re-add,
	// e.g. /gst/remove followed by the next playlist request) is Closed while
	// its NewTorrent goroutine is still in here — the re-read of t.handle() between
	// the nil check and the call segfaulted on the nil receiver. A call on a
	// CLOSED handle is harmless (the C slot lookup returns an error).
	lh := t.handle()
	if lh == nil {
		return
	}
	if have, _ := lh.HaveMetadata(); !have {
		return
	}
	t.gotInfoOnce.Do(func() {
		// Switch to lazy/streaming mode: download nothing until a Reader's
		// window or Preload bumps the specific pieces it needs.
		_ = lh.SetAllPiecesPriority(0)
		if ts := lh.TotalSize(); ts > 0 {
			if c := torrstor.Global().CacheByHash([20]byte(t.Hash())); c != nil {
				c.SetTotalSize(ts)
			}
		}
		// Backfill the spec with the just-received info-dict: a magnet-added
		// torrent's spec has no InfoBytes, so without this every DB save stores
		// the bare magnet and every server restart re-fetches metadata from the
		// swarm (slow playlists / slow first play after restart).
		t.mu.Lock()
		needInfo := t.TorrentSpec != nil && len(t.TorrentSpec.InfoBytes) == 0
		t.mu.Unlock()
		if needInfo {
			if mb, err := lh.Metadata(); err == nil && len(mb) > 0 {
				t.mu.Lock()
				if t.TorrentSpec != nil && len(t.TorrentSpec.InfoBytes) == 0 {
					t.TorrentSpec.InfoBytes = mb
				}
				t.mu.Unlock()
				if pt, err := lt.ParseTorrentBytes(mb); err == nil && !pt.Private && len(pt.Trackers) > 0 {
					utils.SaveTrackersToFile(pt.Trackers)
				}
			}
		}
		if t.gotInfoCh != nil {
			close(t.gotInfoCh)
		}
	})
}

// WaitInfo blocks until metadata is received or the torrent is closed or
// the configured timeout elapses. Returns true on success.
func (t *Torrent) WaitInfo() bool {
	if t == nil {
		return false
	}
	// Same handle snapshot as signalGotInfo: Close() nils t.handle() concurrently.
	lh := t.handle()
	if lh == nil {
		return false
	}
	// Fast path: already have metadata (e.g. info bytes were passed in).
	if have, _ := lh.HaveMetadata(); have {
		t.signalGotInfo()
		return true
	}
	deadline := time.Minute + time.Second*time.Duration(settings.BTsets().TorrentDisconnectTimeout)
	select {
	case <-t.gotInfoCh:
		return true
	case <-t.closeCh:
		return false
	case <-time.After(deadline):
		return false
	}
}

// GotInfo wraps WaitInfo with state transitions matching the legacy API.
//
// Players open a file with several requests at once (the head, then the
// Matroska cues near the end) while the UI polls the status, so this runs
// concurrently with itself, with Status and with Close. Stat is therefore read
// and written under t.mu, and a torrent closed while the metadata was awaited
// stays closed instead of being turned back into a working one.
func (t *Torrent) GotInfo() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	switch t.Stat {
	case state.TorrentClosed:
		t.mu.Unlock()
		return false
	case state.TorrentPreload:
		t.mu.Unlock()
		return true
	case state.TorrentWorking:
		if lh := t.handle(); lh != nil {
			if have, _ := lh.HaveMetadata(); have {
				t.mu.Unlock()
				t.AddExpiredTime(torrentExpireTimeout())
				return true
			}
		}
	}
	t.Stat = state.TorrentGettingInfo
	t.mu.Unlock()

	if t.WaitInfo() {
		t.mu.Lock()
		if t.Stat == state.TorrentClosed {
			t.mu.Unlock()
			return false
		}
		if t.Stat == state.TorrentGettingInfo {
			t.Stat = state.TorrentWorking
		}
		t.mu.Unlock()
		t.AddExpiredTime(torrentExpireTimeout())
		// Metadata (and so the release name) is now known — backfill a TMDB
		// poster for torrents that came in without one (bare magnets, tgbot,
		// autoload). No-op unless a TMDB API key is configured.
		go t.posterOnce.Do(t.fetchPosterIfMissing)
		return true
	}
	// Deregister before closing: expired() skips TorrentClosed, so a closed
	// instance left in the registry is never reaped and shadows its hash —
	// every later add of the same magnet returns the zombie and fails
	// instantly instead of retrying the metadata fetch.
	if t.bt != nil {
		t.bt.dropInstance(t)
	} else {
		t.Close()
	}
	return false
}

// AddExpiredTime pushes the auto-drop deadline forward.
func (t *Torrent) AddExpiredTime(d time.Duration) {
	newDeadline := time.Now().Add(d)
	t.mu.Lock()
	if t.expiredTime.Before(newDeadline) {
		t.expiredTime = newDeadline
	}
	t.mu.Unlock()
}

// expired reports whether the torrent has passed its auto-drop deadline and is
// safe to drop from the session: it must not be mid-handshake (getting metadata
// or preloading) or already closed, and must have no active reader. Playback,
// status polls and preload all push expiredTime forward, so an in-use torrent
// is never seen as expired. Used by BTServer.expireWatch.
func (t *Torrent) expired(now time.Time) bool {
	t.mu.Lock()
	stat := t.Stat
	deadline := t.expiredTime
	t.mu.Unlock()
	switch stat {
	case state.TorrentClosed, state.TorrentGettingInfo, state.TorrentPreload, state.TorrentInDB:
		return false
	}
	if deadline.IsZero() || now.Before(deadline) {
		return false
	}
	if c := torrstor.Global().CacheByHash([20]byte(t.Hash())); c != nil && c.ActiveReaders() > 0 {
		return false
	}
	return true
}

// ----- watch / progress -----

func (t *Torrent) watch() {
	t.watcher = time.NewTicker(time.Second)
	defer t.watcher.Stop()
	for {
		select {
		case <-t.closeCh:
			return
		case <-t.watcher.C:
			t.progressTick()
		}
	}
}

func (t *Torrent) progressTick() {
	if t.handle() == nil {
		return
	}
	st, err := t.handle().Status()
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	dt := now.Sub(t.lastTimeSpeed).Seconds()
	if dt > 0 {
		dlDelta := st.TotalPayloadDownload - t.BytesReadUsefulData
		upDelta := st.TotalPayloadUpload - t.BytesWrittenData
		t.DownloadSpeed = float64(dlDelta) / dt
		t.UploadSpeed = float64(upDelta) / dt
	}
	t.BytesReadUsefulData = st.TotalPayloadDownload
	t.BytesWrittenData = st.TotalPayloadUpload
	t.lastTimeSpeed = now
}

// ----- shutdown -----

// Close pauses the underlying libtorrent torrent and marks it closed.
// Actual removal from the session is performed by BTServer.RemoveTorrent.
func (t *Torrent) Close() bool {
	if t == nil {
		return false
	}
	// Check and set under the lock: two closes racing each other, or a close
	// racing GotInfo, must leave exactly one winner and a closed torrent.
	t.mu.Lock()
	if t.Stat == state.TorrentClosed {
		t.mu.Unlock()
		return true
	}
	t.Stat = state.TorrentClosed
	t.mu.Unlock()
	t.markClosed()
	if t.handle() != nil && t.bt != nil && t.bt.Session() != nil {
		// Only remove the libtorrent torrent if no OTHER live instance owns
		// it. A duplicate Torrent that lost an add race (or any stale copy)
		// shares the same underlying lt torrent with the registered one;
		// removing it here would kill an active stream. The registry entry
		// is either us or already deleted (RemoveTorrent deletes before
		// closing) — both mean we own the removal.
		if cur := t.bt.GetTorrent(t.Hash()); cur == nil || cur == t {
			_ = t.handle().Remove(false)
		}
	}
	t.lh.Store(nil)
	return true
}

func (t *Torrent) markClosed() {
	t.closeOnce.Do(func() {
		if t.closeCh != nil {
			close(t.closeCh)
		}
	})
}

// ----- accessors -----

// Hash returns the v1 info hash. Falls back to the spec if libtorrent
// handle is no longer valid.
func (t *Torrent) Hash() Hash {
	if t == nil {
		return Hash{}
	}
	if t.TorrentSpec != nil && !t.TorrentSpec.InfoHash.IsZero() {
		return t.TorrentSpec.InfoHash
	}
	if t.handle() != nil {
		return NewHashFromHex(t.handle().InfoHash())
	}
	return Hash{}
}

// Name returns the torrent's display name (from metadata if available,
// otherwise from the spec's display_name / file name).
func (t *Torrent) Name() string {
	if t.handle() != nil {
		if name := t.handle().DisplayName(); name != "" {
			return name
		}
	}
	if t.TorrentSpec != nil {
		return t.TorrentSpec.DisplayName
	}
	return ""
}

// Length returns the total payload size in bytes (0 before metadata).
func (t *Torrent) Length() int64 {
	if t == nil || t.handle() == nil {
		return 0
	}
	return t.handle().TotalSize()
}

// Files returns the file list once metadata is known.
func (t *Torrent) Files() []*File {
	if t == nil || t.handle() == nil {
		return nil
	}
	raw, err := t.handle().Files()
	if err != nil || len(raw) == 0 {
		return nil
	}
	out := make([]*File, 0, len(raw))
	for i := range raw {
		f := raw[i]
		out = append(out, &File{
			Index:  f.Index,
			Path:   f.Path,
			Length: f.Size,
			Offset: f.Offset,
		})
	}
	sort.Slice(out, func(i, j int) bool { return utils2.CompareStrings(out[i].Path, out[j].Path) })
	return out
}

// LTHandle returns the underlying libtorrent handle for callers that
// need to set piece priorities / deadlines. May be nil for DB-only
// torrents.
func (t *Torrent) LTHandle() *lt.Torrent { return t.handle() }

var (
	reTVSeasonEp      = regexp.MustCompile(`(?i)Season\W*(\d+).*\WEpisode\W*(\d+)`)
	reTVSE            = regexp.MustCompile(`(?i)\bS(\d+)(?:[^0-9E]+)?E(\d+)\b`)
	reTVX             = regexp.MustCompile(`(?i)\b(\d+)x(\d+)\b`)
	reTVSeasonGen     = regexp.MustCompile(`(?i)\bS(?:eason)?\W*(\d+)`)
	reTVSeasonRU      = regexp.MustCompile(`(?i)Сезон\W*(\d+)\b`)
	reTVSeasonReverse = regexp.MustCompile(`(?i)(\d+)\W*(?:-[йяе]?\s+)?(?:Сезон|Season)`)
	reTVEpGen         = regexp.MustCompile(`(?i)\bE(?:p|pisode)?\W*(\d+)\b`)
	reTVEpLeading     = regexp.MustCompile(`^\s*(\d+)`)
)

func isVideoFileExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".avi", ".m4v", ".mov", ".webm", ".ts", ".m2ts", ".mts", ".mpg", ".mpeg", ".wmv", ".flv", ".ogv", ".vob", ".3gp":
		return true
	}
	return false
}

func extractTVSeason(s string) int {
	if m := reTVSeasonGen.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := reTVSeasonReverse.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := reTVSeasonRU.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func extractTVEpisode(s string) int {
	if m := reTVEpGen.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if m := reTVEpLeading.FindStringSubmatch(s); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func buildFileStats(files []*File, category, title string) []*state.TorrentFileStat {
	if len(files) == 0 {
		return nil
	}
	files = append([]*File(nil), files...)
	catLower := strings.ToLower(category)
	isMovie := strings.Contains(catLower, "movie")

	// Check for Smart Indexing (Movie Mode):
	// Trigger if Category is explicitly "movie" OR if the largest file dominates
	// the torrent (> 85% of total size), implying it's the main movie file.
	if len(files) > 1 {
		totalSize := int64(0)
		maxSize := int64(0)
		maxIndex := -1
		for i, f := range files {
			totalSize += f.Length
			if f.Length > maxSize {
				maxSize = f.Length
				maxIndex = i
			}
		}
		if !isMovie && totalSize > 0 && float64(maxSize) > float64(totalSize)*0.85 {
			isMovie = true
		}
		if isMovie && maxIndex > 0 {
			largest := files[maxIndex]
			files = append(files[:maxIndex], files[maxIndex+1:]...)
			files = append([]*File{largest}, files...)
		}
	}

	// Smart Indexing (TV Series Mode) — exclude "anime" to keep default indexing.
	if strings.Contains(catLower, "tv") && !strings.Contains(catLower, "anime") {
		parseSeasonEp := func(inputStr string) (int, int) {
			for _, re := range []*regexp.Regexp{reTVSeasonEp, reTVSE, reTVX} {
				if matches := re.FindStringSubmatch(inputStr); len(matches) == 3 {
					season, _ := strconv.Atoi(matches[1])
					episode, _ := strconv.Atoi(matches[2])
					if season > 0 && episode > 0 {
						return season, episode
					}
				}
			}
			dir := filepath.Dir(inputStr)
			base := filepath.Base(inputStr)
			if ep := extractTVEpisode(base); ep > 0 {
				season := extractTVSeason(dir)
				if season == 0 {
					season = extractTVSeason(category)
				}
				if season == 0 {
					season = extractTVSeason(title)
				}
				if season > 0 {
					return season, ep
				}
			}
			return 0, 0
		}

		if len(files) == 1 {
			f := files[0]
			s, ep := parseSeasonEp(f.Path)
			if s == 0 || ep == 0 {
				s, ep = parseSeasonEp(title)
			}
			if s == 0 || ep == 0 {
				s, ep = parseSeasonEp(category)
			}
			if s > 0 && ep > 0 {
				mult := 100
				if ep >= 1000 {
					mult = 10000
				} else if ep >= 100 {
					mult = 1000
				}
				return []*state.TorrentFileStat{{
					Id:     s*mult + ep,
					Path:   f.Path,
					Length: f.Length,
				}}
			}
		} else {
			type sePair struct{ s, ep int }
			parsed := make([]sePair, len(files))
			maxEp := 0
			for i, f := range files {
				s, ep := parseSeasonEp(f.Path)
				parsed[i] = sePair{s: s, ep: ep}
				if ep > maxEp {
					maxEp = ep
				}
			}
			mult := 100
			if maxEp >= 1000 {
				mult = 10000
			} else if maxEp >= 100 {
				mult = 1000
			}

			customIDs := make(map[int]*File)
			customIdx := make(map[int]int)
			usedIndices := make(map[int]bool)

			for i, f := range files {
				p := parsed[i]
				if p.s <= 0 || p.ep <= 0 {
					continue
				}
				id := p.s*mult + p.ep
				existing, exists := customIDs[id]
				if !exists {
					customIDs[id] = f
					customIdx[id] = i
					usedIndices[i] = true
					continue
				}
				// Prefer video containers over external subtitles/audio tracks,
				// and larger files on ties; never drop the displaced file from
				// FileStats so /subs external track discovery keeps seeing it.
				fVid, exVid := isVideoFileExt(f.Path), isVideoFileExt(existing.Path)
				if (fVid && !exVid) || (fVid == exVid && f.Length > existing.Length) {
					usedIndices[customIdx[id]] = false
					customIDs[id] = f
					customIdx[id] = i
					usedIndices[i] = true
				}
			}

			if len(customIDs) > 0 {
				out := make([]*state.TorrentFileStat, 0, len(files))
				sortedIDs := make([]int, 0, len(customIDs))
				for id := range customIDs {
					sortedIDs = append(sortedIDs, id)
				}
				sort.Ints(sortedIDs)

				for _, id := range sortedIDs {
					f := customIDs[id]
					out = append(out, &state.TorrentFileStat{
						Id:     id,
						Path:   f.Path,
						Length: f.Length,
					})
				}

				defaultID := 10000
				if maxID := sortedIDs[len(sortedIDs)-1]; maxID >= defaultID {
					defaultID = maxID + 1
				}
				for i, f := range files {
					if !usedIndices[i] {
						out = append(out, &state.TorrentFileStat{
							Id:     defaultID,
							Path:   f.Path,
							Length: f.Length,
						})
						defaultID++
					}
				}
				return out
			}
		}
	}

	out := make([]*state.TorrentFileStat, 0, len(files))
	for i, f := range files {
		id := f.Index + 1
		if isMovie {
			id = i + 1 // Main movie (files[0]) is always Id: 1
		}
		out = append(out, &state.TorrentFileStat{
			Id:     id, // legacy: 0 means undefined in the web UI
			Path:   f.Path,
			Length: f.Length,
		})
	}
	return out
}

// Status builds a state.TorrentStatus snapshot consumed by the web API.
func (t *Torrent) Status() *state.TorrentStatus {
	t.mu.Lock()
	st := new(state.TorrentStatus)
	st.Stat = t.Stat
	st.StatString = t.Stat.String()
	st.Title = t.Title
	st.Category = t.Category
	st.Poster = t.Poster
	st.Data = t.Data
	st.Timestamp = t.Timestamp
	st.TorrentSize = t.Size
	st.BitRate = t.BitRate
	st.DurationSeconds = t.DurationSeconds
	dlSpeed := t.DownloadSpeed
	upSpeed := t.UploadSpeed
	preloadedBytes := t.PreloadedBytes
	preloadSize := t.PreloadSize
	dirtiedGood := t.piecesDirtiedGood
	dirtiedBad := t.piecesDirtiedBad
	var specInfoBytes []byte
	var specTrackers0 []string
	if t.TorrentSpec != nil {
		st.Hash = t.TorrentSpec.InfoHash.HexString()
		specInfoBytes = t.TorrentSpec.InfoBytes
		if len(t.TorrentSpec.Trackers) > 0 {
			specTrackers0 = append([]string(nil), t.TorrentSpec.Trackers[0]...)
		}
	}
	t.mu.Unlock()

	populateTorrsHash := func() {
		if len(st.FileStats) == 0 || st.Hash == "" {
			return
		}
		th := torrshash.New(st.Hash)
		th.AddField(torrshash.TagTitle, st.Title)
		th.AddField(torrshash.TagPoster, st.Poster)
		th.AddField(torrshash.TagCategory, st.Category)
		th.AddField(torrshash.TagSize, strconv.FormatInt(st.TorrentSize, 10))
		for _, tr := range specTrackers0 {
			th.AddField(torrshash.TagTracker, tr)
		}
		if tok, err := torrshash.Pack(th); err == nil {
			st.TorrsHash = tok
		}
	}

	lh := t.handle()
	if lh == nil {
		// DB-resident torrent (not in the session): recover the file list from
		// the record's cached Data, or fall back to InfoBytes when Data holds
		// external client JSON (e.g. Lampa metadata) rather than tsFiles.
		st.FileStats = fileStatsFromData(st.Data)
		if len(st.FileStats) == 0 && len(specInfoBytes) > 0 {
			if pt, err := lt.ParseTorrentBytes(specInfoBytes); err == nil && pt.HasMetadata && len(pt.Files) > 0 {
				if st.TorrentSize == 0 && pt.TotalSize > 0 {
					st.TorrentSize = pt.TotalSize
				}
				if st.Name == "" {
					st.Name = pt.DisplayName
				}
				files := make([]*File, 0, len(pt.Files))
				for i := range pt.Files {
					f := pt.Files[i]
					files = append(files, &File{
						Index:  f.Index,
						Path:   f.Path,
						Length: f.Size,
						Offset: f.Offset,
					})
				}
				sort.Slice(files, func(i, j int) bool { return utils2.CompareStrings(files[i].Path, files[j].Path) })
				st.FileStats = buildFileStats(files, st.Category, st.Title)
			}
		}
		populateTorrsHash()
		return st
	}

	// Query libtorrent outside t.mu so CGo round-trips never block GotInfo,
	// Preload progress updates, or Close.
	lst, err := lh.Status()
	if err != nil {
		return st
	}
	st.Name = lst.Name
	if st.Hash == "" {
		st.Hash = lst.InfoHash
	}
	st.LoadedSize = lst.TotalDone
	st.DownloadSpeed = dlSpeed
	st.UploadSpeed = upSpeed
	st.TotalPeers = lst.ListPeers
	st.PendingPeers = lst.ConnectCandidates
	st.ActivePeers = lst.NumPeers
	st.ConnectedSeeders = lst.NumSeeds
	st.HalfOpenPeers = 0
	st.BytesWritten = lst.TotalUpload
	st.BytesWrittenData = lst.TotalPayloadUpload
	st.BytesRead = lst.TotalDownload
	st.BytesReadData = lst.TotalPayloadDownload
	st.BytesReadUsefulData = lst.TotalPayloadDownload
	st.PreloadedBytes = preloadedBytes
	st.PreloadSize = preloadSize

	// libtorrent doesn't surface chunk counters directly via
	// torrent_status; approximate by dividing payload bytes by the
	// 16 KiB BitTorrent block size. Good enough for /cache + UI.
	const chunkSize = int64(16 * 1024)
	st.ChunksRead = lst.TotalPayloadDownload / chunkSize
	st.ChunksReadUseful = st.ChunksRead
	st.ChunksReadWasted = (lst.TotalDownload - lst.TotalPayloadDownload) / chunkSize
	st.ChunksWritten = lst.TotalPayloadUpload / chunkSize
	st.PiecesDirtiedGood = dirtiedGood
	st.PiecesDirtiedBad = dirtiedBad

	if !lst.HasMetadata {
		// Metadata still in flight — fall back to the DB-cached file list (if
		// this torrent was ever saved) so the UI isn't blank meanwhile.
		st.FileStats = fileStatsFromData(st.Data)
	} else {
		st.TorrentSize = lst.TotalSize
		if files := t.Files(); len(files) > 0 {
			st.FileStats = buildFileStats(files, st.Category, st.Title)
			populateTorrsHash()
		}
	}
	return st
}

// ----- streaming / reader -----

// NewReader hands out an io.ReadSeekCloser over the requested file,
// backed by the Cache registered for this torrent. Returns nil if the
// cache hasn't been opened yet (metadata still in flight) or file is
// nil.
func (t *Torrent) NewReader(file *File) Reader {
	return t.NewReaderGroup(file, "")
}

// waitCache returns this torrent's torrstor.Cache, waiting briefly when
// libtorrent has just received magnet metadata and construct_storage() ->
// cb_.open() is still finishing on the network thread.
func (t *Torrent) waitCache(ctx context.Context) *torrstor.Cache {
	if t == nil {
		return nil
	}
	h := [20]byte(t.Hash())
	if c := torrstor.Global().CacheByHash(h); c != nil {
		return c
	}
	lh := t.handle()
	if lh == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if t.closeCh != nil {
		go func() {
			select {
			case <-t.closeCh:
				cancel()
			case <-wctx.Done():
			}
		}()
	}
	c := torrstor.Global().WaitForCache(wctx, h)
	if c != nil {
		if ts := lh.TotalSize(); ts > 0 {
			c.SetTotalSize(ts)
		}
	}
	return c
}

// NewReaderGroup is NewReader with an explicit device/session key (the client IP)
// so concurrent streams of the same torrent from different devices each get their
// own isolated sliding window in the cache. The streaming HTTP path passes the
// caller's IP; internal consumers (DLNA index, tgbot, FUSE) use NewReader and
// share the default group.
func (t *Torrent) NewReaderGroup(file *File, group string) Reader {
	if t == nil || t.handle() == nil || file == nil {
		return nil
	}
	cache := t.waitCache(context.Background())
	if cache == nil {
		log.TLogln("torr.NewReader: no cache for", t.Hash().HexString())
		return nil
	}
	return torrstor.NewReader(cache, t.handle(), torrstor.FileInfo{
		Index:  file.Index,
		Path:   file.Path,
		Offset: file.Offset,
		Length: file.Length,
	}, group)
}

// CloseReader releases a previously handed-out reader and pushes the
// torrent's auto-drop deadline forward.
func (t *Torrent) CloseReader(r Reader) {
	if r != nil {
		_ = r.Close()
	}
	t.AddExpiredTime(torrentExpireTimeout())
}

// CacheState returns a stubbed CacheState during Etap 3 — the real cache
// is reborn in Etap 4. Fields are filled enough to keep /cache and the
// tgbot snake command rendering something coherent (an empty piece map).
func (t *Torrent) CacheState() *storageState.CacheState {
	if t == nil {
		return &storageState.CacheState{Pieces: map[int]storageState.ItemState{}}
	}
	// Pull the real stats (Capacity, Filled, per-piece state) from the live
	// torrstor cache; fall back to an empty shell if it isn't open yet.
	var st *storageState.CacheState
	if c := torrstor.Global().CacheByHash([20]byte(t.Hash())); c != nil {
		st = c.State()
	} else {
		st = &storageState.CacheState{Pieces: map[int]storageState.ItemState{}}
	}
	st.Torrent = t.Status()
	if t.TorrentSpec != nil {
		st.Hash = t.TorrentSpec.InfoHash.HexString()
	}
	if t.handle() != nil {
		st.PiecesCount = t.handle().NumPieces()
		st.PiecesLength = t.handle().PieceLength()
	}
	// Never hand the web UI a nil Pieces/Readers: useCreateCacheMap iterates both
	// without a null guard, so JSON null crashes the torrent info dialog.
	if st.Pieces == nil {
		st.Pieces = map[int]storageState.ItemState{}
	}
	if st.Readers == nil {
		st.Readers = []*storageState.ReaderState{}
	}
	return st
}
