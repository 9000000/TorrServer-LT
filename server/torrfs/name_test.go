package torrfs

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal name", "normal name"},
		{"movie / title", "movie _ title"},
		{"path\\with\\backslashes", "path_with_backslashes"},
		{"mix/of\\both", "mix_of_both"},
		{"..", ""},
		{".", ""},
		{"", ""},
		{"   ", ""},
		{"clean\x00control\x1fchars", "cleancontrolchars"},
		{"leading / trailing /", "leading _ trailing _"},
	}

	for _, tc := range tests {
		got := SanitizeName(tc.input)
		if got != tc.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
