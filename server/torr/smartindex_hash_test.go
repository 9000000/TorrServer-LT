package torr

import (
	"os"
	"path/filepath"
	"testing"

	"server/settings"
)

func TestParseHash(t *testing.T) {
	valid := "0123456789abcdef0123456789abcdef01234567"
	if h, ok := ParseHash(valid); !ok || h.HexString() != valid {
		t.Fatalf("ParseHash(%q) = (%s, %v), want (%s, true)", valid, h.HexString(), ok, valid)
	}
	if h, ok := ParseHash("  0123456789ABCDEF0123456789ABCDEF01234567\n"); !ok || h.HexString() != valid {
		t.Fatalf("ParseHash uppercase/padded failed: (%s, %v)", h.HexString(), ok)
	}

	bad := []string{
		"",
		"abc",
		"../outside",
		"../../etc",
		"0000000000000000000000000000000000000000",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		"0123456789abcdef0123456789abcdef012345678",
	}
	for _, in := range bad {
		if _, ok := ParseHash(in); ok {
			t.Errorf("ParseHash(%q) = ok, want false", in)
		}
	}
}

func TestRemTorrentRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "cache")
	outsideDir := filepath.Join(root, "outside")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outsideDir, "keep.txt")
	if err := os.WriteFile(victim, []byte("important"), 0o644); err != nil {
		t.Fatal(err)
	}

	prevSets := settings.BTsets()
	prevRO := settings.ReadOnly
	t.Cleanup(func() {
		settings.StoreBTsets(prevSets)
		settings.ReadOnly = prevRO
	})
	settings.ReadOnly = false
	settings.StoreBTsets(&settings.BTSets{
		UseDisk:          true,
		TorrentsSavePath: cacheDir,
	})

	RemTorrent("../outside")
	RemTorrent("..")
	RemTorrent(".")

	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("victim file deleted via RemTorrent traversal: %v", err)
	}
}

func TestBuildFileStatsTVSeriesSubtitlesAndHighEpisodes(t *testing.T) {
	// Case 1: External subtitle appears after video (or before video) with the same S01E01 tag.
	// Video must claim ID 101 and external subtitle must still be retained at 10000+ for /subs.
	files := []*File{
		{Index: 0, Path: "Show/Show.S01E01.mkv", Length: 1_500_000_000},
		{Index: 1, Path: "Show/Show.S01E01.srt", Length: 45_000},
		{Index: 2, Path: "Show/Show.S01E02.srt", Length: 42_000},
		{Index: 3, Path: "Show/Show.S01E02.mkv", Length: 1_400_000_000},
	}
	stats := buildFileStats(files, "tv", "Show")
	if len(stats) != 4 {
		t.Fatalf("len(stats) = %d, want 4 (no displaced files dropped)", len(stats))
	}
	byID := make(map[int]string, len(stats))
	for _, s := range stats {
		byID[s.Id] = s.Path
	}
	if byID[101] != "Show/Show.S01E01.mkv" {
		t.Fatalf("ID 101 = %q, want Show/Show.S01E01.mkv", byID[101])
	}
	if byID[102] != "Show/Show.S01E02.mkv" {
		t.Fatalf("ID 102 = %q, want Show/Show.S01E02.mkv", byID[102])
	}
	if byID[10000] != "Show/Show.S01E01.srt" || byID[10001] != "Show/Show.S01E02.srt" {
		t.Fatalf("subtitles not preserved at fallback IDs: %#v", byID)
	}

	// Case 2: Episodes >= 100 must not collide S01E105 with S02E05.
	longFiles := []*File{
		{Index: 0, Path: "Show/S01E105.mkv", Length: 500_000_000},
		{Index: 1, Path: "Show/S02E05.mkv", Length: 500_000_000},
	}
	longStats := buildFileStats(longFiles, "tv", "Show")
	if len(longStats) != 2 {
		t.Fatalf("len(longStats) = %d, want 2", len(longStats))
	}
	if longStats[0].Id == longStats[1].Id {
		t.Fatalf("episode ID collision: both got %d", longStats[0].Id)
	}
	if longStats[0].Id != 1105 || longStats[1].Id != 2005 {
		t.Fatalf("got IDs (%d, %d), want (1105, 2005)", longStats[0].Id, longStats[1].Id)
	}
}
