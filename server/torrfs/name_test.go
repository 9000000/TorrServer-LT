package torrfs

import "testing"

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"Movie (2024)":           "Movie (2024)",
		"AC/DC - Live":           "AC_DC - Live",
		`Show\Season 1`:          "Show_Season 1",
		"  padded  ":             "padded",
		"tab\there\x7f":          "tabhere",
		".":                      "",
		"..":                     "",
		"/":                      "_",
		"":                       "",
		"Сериал / Series S01E01": "Сериал _ Series S01E01",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCategoryNameFallsBackToOther(t *testing.T) {
	for _, in := range []string{"", "  ", ".", ".."} {
		if got := categoryName(in); got != "other" {
			t.Errorf("categoryName(%q) = %q, want other", in, got)
		}
	}
	if got := categoryName("tv/shows"); got != "tv_shows" {
		t.Errorf("categoryName(tv/shows) = %q", got)
	}
}
