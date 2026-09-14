package utils

import (
	"crypto/rand"
	"encoding/base32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"server/log"
	"server/settings"
)

var (
	// Healthy, highly reliable public fallback trackers
	defTrackers = []string{
		"udp://tracker.opentrackr.org:1337/announce",
		"udp://open.demonii.com:1337/announce",
		"udp://open.stealth.si:80/announce",
		"udp://tracker.torrent.eu.org:451/announce",
		"udp://explodie.org:6969/announce",
		"udp://tracker.openbittorrent.com:6969/announce",
		"udp://tracker.cyberia.is:6969/announce",
		"udp://exodus.desync.com:6969/announce",
		"http://tracker.opentrackr.org:1337/announce",
		"http://bt4.t-ru.org/ann?magnet",
		"wss://tracker.btorrent.xyz",
		"wss://tracker.openwebtorrent.com",
	}

	// High-availability upstream tracker lists + CDN mirrors
	defaultTrackerURLs = []string{
		"https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_best_ip.txt",
		"https://cdn.jsdelivr.net/gh/ngosang/trackerslist@master/trackers_best_ip.txt",
		"https://raw.githubusercontent.com/XIU2/TrackersListCollection/master/best.txt",
		"https://cdn.jsdelivr.net/gh/XIU2/TrackersListCollection@master/best.txt",
		"https://newtrackon.com/api/stable",
	}

	trackerClient = &http.Client{
		Timeout: 7 * time.Second,
	}

	trackersMu      sync.RWMutex
	loadedTrackers  []string
	lastUpdated     time.Time
	isUpdating      bool
	updateMu        sync.Mutex
	trackerInitOnce sync.Once

	refreshInterval = 24 * time.Hour
)

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

func fetchTrackersFromURL(url string) ([]string, error) {
	resp, err := trackerClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ret []string
	for _, line := range strings.Split(string(buf), "\n") {
		if tr, ok := normalizeTracker(line); ok {
			ret = append(ret, tr)
		}
	}
	return ret, nil
}

func refreshTrackers() {
	updateMu.Lock()
	if isUpdating {
		updateMu.Unlock()
		return
	}
	isUpdating = true
	updateMu.Unlock()

	defer func() {
		updateMu.Lock()
		isUpdating = false
		updateMu.Unlock()
	}()

	var sources []string
	if env := os.Getenv("TS_TRACKERS_URL"); env != "" {
		for _, u := range strings.FieldsFunc(env, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\n'
		}) {
			if u = strings.TrimSpace(u); u != "" {
				sources = append(sources, u)
			}
		}
	}
	sources = append(sources, defaultTrackerURLs...)

	seen := make(map[string]bool)
	var fresh []string

	for _, src := range sources {
		list, err := fetchTrackersFromURL(src)
		if err != nil || len(list) == 0 {
			continue
		}
		log.TLogln("utils.trackers: fetched", len(list), "trackers from", src)
		for _, tr := range list {
			if !seen[tr] {
				seen[tr] = true
				fresh = append(fresh, tr)
			}
		}
		// If we've gathered a healthy list of trackers (>= 20), we have enough
		if len(fresh) >= 20 {
			break
		}
	}

	// Always ensure healthy fallback trackers are included
	for _, tr := range defTrackers {
		if !seen[tr] {
			seen[tr] = true
			fresh = append(fresh, tr)
		}
	}

	if len(fresh) > 0 {
		trackersMu.Lock()
		loadedTrackers = fresh
		lastUpdated = time.Now()
		trackersMu.Unlock()
		log.TLogln("utils.trackers: total active trackers:", len(fresh))
	}
}

// InitTrackers starts background fetching and sets up daily periodic refresh.
func InitTrackers() {
	trackerInitOnce.Do(func() {
		go func() {
			refreshTrackers()
			ticker := time.NewTicker(refreshInterval)
			for range ticker.C {
				refreshTrackers()
			}
		}()
	})
}

// GetTrackerFromFile loads optional trackers.txt from data dir.
func GetTrackerFromFile() []string {
	name := filepath.Join(settings.Path, "trackers.txt")
	buf, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	var ret []string
	seen := make(map[string]bool)
	for _, l := range strings.Split(string(buf), "\n") {
		if tr, ok := normalizeTracker(l); ok && !seen[tr] {
			seen[tr] = true
			ret = append(ret, tr)
		}
	}
	return ret
}

// GetDefTrackers returns the default tracker list, automatically refreshed in the background.
func GetDefTrackers() []string {
	InitTrackers()

	trackersMu.RLock()
	defer trackersMu.RUnlock()

	if len(loadedTrackers) == 0 {
		ret := make([]string, len(defTrackers))
		copy(ret, defTrackers)
		return ret
	}

	ret := make([]string, len(loadedTrackers))
	copy(ret, loadedTrackers)
	return ret
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
