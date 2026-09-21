package mcp

import (
	"context"
	"strings"
	"testing"

	set "server/settings"
)

func TestSearchTorrentsHonoursDisabledSources(t *testing.T) {
	old := set.BTsets()
	set.StoreBTsets(&set.BTSets{})
	t.Cleanup(func() { set.StoreBTsets(old) })

	ctx := context.Background()
	for source, want := range map[string]string{
		"":        "search is disabled",
		"all":     "search is disabled",
		"rutor":   "RuTor search is disabled",
		"torznab": "Torznab search is disabled",
		"jacred":  "JacRed search is disabled",
	} {
		_, out, err := searchTorrents(ctx, nil, searchTorrentsIn{Query: "x", Source: source})
		if err != nil {
			t.Fatalf("source %q: %v", source, err)
		}
		if !strings.Contains(out.Message, want) || len(out.Results) != 0 {
			t.Errorf("source %q: message=%q results=%d, want %q", source, out.Message, len(out.Results), want)
		}
	}

	if _, _, err := searchTorrents(ctx, nil, searchTorrentsIn{Query: "x", Source: "bogus"}); err == nil {
		t.Error("unknown source: want an error")
	}
	if _, _, err := searchTorrents(ctx, nil, searchTorrentsIn{Query: " "}); err == nil {
		t.Error("empty query: want an error")
	}
}

func TestServerInfoReportsJacRed(t *testing.T) {
	old := set.BTsets()
	set.StoreBTsets(&set.BTSets{EnableJacRedSearch: true, TrackTimecode: true})
	t.Cleanup(func() { set.StoreBTsets(old) })

	_, out, err := getServerInfo(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.JacRedSearch || !out.TrackTimecode || out.RutorSearch || out.TorznabSearch {
		t.Fatalf("server info = %+v", out)
	}
}
