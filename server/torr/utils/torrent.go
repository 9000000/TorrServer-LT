package utils

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"server/log"
	"server/settings"
)

// trackersFetchTimeout bounds one remote trackers-list download. The fetch
// runs in the background, but a blocked mirror must still give up quickly so
// the next mirror in the chain gets its turn.
const trackersFetchTimeout = 5 * time.Second

// trackersRefreshInterval controls how often the remote trackers list is
// re-fetched in the background. On refresh failure the existing cache is kept.
var trackersRefreshInterval = 12 * time.Hour

// defaultTrackersListURLs is the built-in mirror chain. Tests stub it to keep
// off the network.
var defaultTrackersListURLs = append([]string(nil), settings.DefaultTrackersListURLs...)

var (
	// fallbackTrackers is used when BTsets is not loaded yet or its
	// DefaultTrackers list is empty.
	fallbackTrackers = parseTrackerLines(settings.DefaultTrackersText)

	trackersMu         sync.RWMutex
	loadedTrackers     []string // nil until the first fetch settles; GetDefTrackers serves the local list meanwhile
	trackersFetchGen   atomic.Uint64
	prefetchMu         sync.Mutex
	prefetchStartedGen uint64 = ^uint64(0)
	refreshLoopOnce    sync.Once

	fileTrackersMu sync.Mutex
)

func getTrackersFilePath() string {
	dir := settings.Path
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "trackers.txt")
}

func normalizeTracker(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\x00", "")
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
		return "", false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "udp://") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "wss://") {
		return s, true
	}
	return "", false
}

// GetTrackerFromFile loads optional trackers.txt from data dir.
func GetTrackerFromFile() []string {
	targetFile := getTrackersFilePath()
	buf, err := os.ReadFile(targetFile)
	if err != nil {
		return nil
	}
	var ret []string
	seen := make(map[string]bool)
	for _, l := range strings.Split(string(buf), "\n") {
		if tr, ok := normalizeTracker(l); ok && !seen[strings.ToLower(tr)] {
			seen[strings.ToLower(tr)] = true
			ret = append(ret, tr)
		}
	}
	return ret
}

// SaveTrackersToFile saves new unique trackers from incoming torrents/magnets to trackers.txt.
// It ensures no duplicates are written if the tracker is already in the file.
func SaveTrackersToFile(trackers []string) {
	if len(trackers) == 0 {
		return
	}

	fileTrackersMu.Lock()
	defer fileTrackersMu.Unlock()

	targetFile := getTrackersFilePath()
	dir := filepath.Dir(targetFile)

	// Collect existing trackers from trackers.txt to avoid duplicates
	seen := make(map[string]bool)
	var existingTargetContent []byte

	if buf, err := os.ReadFile(targetFile); err == nil {
		existingTargetContent = buf
		for _, line := range strings.Split(string(buf), "\n") {
			if tr, ok := normalizeTracker(line); ok {
				seen[strings.ToLower(tr)] = true
			}
		}
	}

	var newTrackers []string
	for _, raw := range trackers {
		tr, ok := normalizeTracker(raw)
		if !ok {
			continue
		}
		lower := strings.ToLower(tr)
		if !seen[lower] {
			seen[lower] = true
			newTrackers = append(newTrackers, tr)
		}
	}

	if len(newTrackers) == 0 {
		return
	}

	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.TLogln("utils.trackers: error opening", targetFile, ":", err)
		return
	}
	defer f.Close()

	var writeBuf strings.Builder
	// If the file already exists, has content and doesn't end with a newline, append a newline first
	if len(existingTargetContent) > 0 && existingTargetContent[len(existingTargetContent)-1] != '\n' {
		writeBuf.WriteString("\n")
	}

	for _, tr := range newTrackers {
		writeBuf.WriteString(tr)
		writeBuf.WriteString("\n")
	}

	if _, err := f.WriteString(writeBuf.String()); err != nil {
		log.TLogln("utils.trackers: error writing to", targetFile, ":", err)
		return
	}

	log.TLogln("utils.trackers: auto-saved", len(newTrackers), "new trackers to", filepath.Base(targetFile))

	// Immediately update in-memory loadedTrackers so running sessions can use them
	trackersMu.Lock()
	if len(loadedTrackers) > 0 {
		memSeen := make(map[string]bool)
		for _, tr := range loadedTrackers {
			memSeen[strings.ToLower(tr)] = true
		}
		for _, tr := range newTrackers {
			if !memSeen[strings.ToLower(tr)] {
				memSeen[strings.ToLower(tr)] = true
				loadedTrackers = append(loadedTrackers, tr)
			}
		}
	}
	trackersMu.Unlock()
}

// InitTrackers initializes and starts background prefetching of trackers.
func InitTrackers() {
	PrefetchTrackers()
}

// GetDefTrackers returns the remote list merged with the local defaults once a
// background fetch has settled, and the local defaults before that. It never
// touches the network: it sits on the torrent add path, which used to block on
// a synchronous download (with no timeout) whenever GitHub was unreachable.
func GetDefTrackers() []string {
	trackersMu.RLock()
	if loadedTrackers != nil {
		out := append([]string(nil), loadedTrackers...)
		trackersMu.RUnlock()
		return out
	}
	trackersMu.RUnlock()
	startPrefetch()
	return configuredDefaultTrackers()
}

// PrefetchTrackers loads the remote trackers list in the background. Safe to
// call repeatedly: fetches of the same generation are deduplicated. It also
// starts the periodic refresh loop once.
func PrefetchTrackers() {
	startPrefetch()
	refreshLoopOnce.Do(func() {
		go trackersRefreshLoop()
	})
}

// InvalidateTrackersCache drops the cached list and starts a new background
// fetch from the current settings.
func InvalidateTrackersCache() {
	trackersMu.Lock()
	loadedTrackers = nil
	trackersMu.Unlock()
	trackersFetchGen.Add(1)
	startPrefetch()
}

func parseTrackerLines(text string) []string {
	var ret []string
	for _, s := range strings.Split(text, "\n") {
		s = strings.TrimSpace(s)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "udp") || strings.HasPrefix(s, "http") || strings.HasPrefix(s, "wss") {
			ret = append(ret, s)
		}
	}
	return ret
}

func configuredDefaultTrackers() []string {
	if sets := settings.BTsets(); sets != nil && strings.TrimSpace(sets.DefaultTrackers) != "" {
		if parsed := parseTrackerLines(sets.DefaultTrackers); len(parsed) > 0 {
			return parsed
		}
	}
	return append([]string(nil), fallbackTrackers...)
}

// configuredTrackersListURLs returns the custom list URL (if any) followed by
// the built-in mirrors, without duplicates.
func configuredTrackersListURLs() []string {
	var custom string
	if sets := settings.BTsets(); sets != nil {
		custom = strings.TrimSpace(sets.TrackersListURL)
	}

	seen := make(map[string]struct{}, len(defaultTrackersListURLs)+1)
	var urls []string
	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	add(custom)
	for _, u := range defaultTrackersListURLs {
		add(u)
	}
	return urls
}

func startPrefetch() {
	gen := trackersFetchGen.Load()

	prefetchMu.Lock()
	if prefetchStartedGen == gen {
		prefetchMu.Unlock()
		return
	}
	prefetchStartedGen = gen
	prefetchMu.Unlock()

	local := configuredDefaultTrackers()
	urls := configuredTrackersListURLs()
	if len(urls) == 0 {
		setLoadedTrackers(gen, local, "")
		return
	}

	go fetchTrackersAsync(gen, urls, local)
}

// setLoadedTrackers publishes a fetch result unless the cache was invalidated
// (settings changed) while the fetch was in flight.
func setLoadedTrackers(gen uint64, trackers []string, logMsg string) bool {
	if trackersFetchGen.Load() != gen {
		return false
	}
	trackersMu.Lock()
	if trackersFetchGen.Load() != gen {
		trackersMu.Unlock()
		return false
	}
	loadedTrackers = append([]string(nil), trackers...)
	trackersMu.Unlock()
	if logMsg != "" {
		log.TLogln(logMsg)
	}
	return true
}

func fetchTrackersAsync(gen uint64, urls []string, local []string) {
	merged, usedURL, err := fetchTrackersFromURLs(urls, local)
	if err != nil {
		setLoadedTrackers(gen, local, "trackerslist fetch failed, using DefaultTrackers: "+err.Error())
		return
	}
	remoteCount := len(merged) - len(local)
	setLoadedTrackers(gen, merged, fmt.Sprintf("trackerslist loaded from %s: %d remote + %d local", usedURL, remoteCount, len(local)))
}

func fetchTrackersFromURLs(urls []string, local []string) ([]string, string, error) {
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("no trackers list URLs")
	}
	var errs []string
	for _, url := range urls {
		merged, err := fetchTrackersFromURL(url, local)
		if err == nil {
			return merged, url, nil
		}
		log.TLogln("trackerslist fetch failed (" + url + "): " + err.Error())
		errs = append(errs, url+": "+err.Error())
	}
	return nil, "", fmt.Errorf("all URLs failed: %s", strings.Join(errs, "; "))
}

func fetchTrackersFromURL(url string, local []string) ([]string, error) {
	client := &http.Client{Timeout: trackersFetchTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	remote := parseTrackerLines(string(buf))
	if len(remote) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return append(remote, local...), nil
}

func trackersRefreshLoop() {
	for {
		time.Sleep(trackersRefreshInterval)
		urls := configuredTrackersListURLs()
		if len(urls) == 0 {
			continue
		}
		gen := trackersFetchGen.Load()
		local := configuredDefaultTrackers()
		merged, usedURL, err := fetchTrackersFromURLs(urls, local)
		if err != nil {
			log.TLogln("trackerslist refresh failed:", err.Error())
			continue
		}
		remoteCount := len(merged) - len(local)
		setLoadedTrackers(gen, merged, fmt.Sprintf("trackerslist refreshed from %s: %d remote + %d local", usedURL, remoteCount, len(local)))
	}
}

// PeerIDRandom builds a peer id with the given prefix padded to 20 chars
// of random base32. Kept for parity with the legacy code; libtorrent now
// generates its own peer fingerprint via the `peer_fingerprint` setting.
func PeerIDRandom(prefix string) string {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return prefix + base32.StdEncoding.EncodeToString(randomBytes)[:20-len(prefix)]
}
