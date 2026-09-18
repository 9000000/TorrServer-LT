package waf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"server/settings"

	"github.com/gin-gonic/gin"
)

func refererStatus(t *testing.T, r *gin.Engine, referer string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("Referer", referer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestDefaultReferersCanBeDisabled(t *testing.T) {
	withTestWAFDB(t, func(dir string) {
		Load()
		r := gin.New()
		r.Use(WAF())
		r.GET("/", func(c *gin.Context) { c.String(200, "ok") })

		builtIn := "https://" + defaultBlockedReferers[0] + "/"
		if !GetSnapshot().DefaultReferersEnabled {
			t.Fatal("built-in referers must be on for a config without the waf key")
		}
		if code := refererStatus(t, r, builtIn); code != http.StatusForbidden {
			t.Fatalf("built-in referer status=%d, want 403", code)
		}

		off := false
		if _, err := Update(ListsUpdate{Referers: "example.com", DefaultReferersEnabled: &off}); err != nil {
			t.Fatal(err)
		}
		if code := refererStatus(t, r, builtIn); code != http.StatusOK {
			t.Fatalf("built-in referer after disabling status=%d, want 200", code)
		}
		if code := refererStatus(t, r, "https://example.com/x"); code != http.StatusForbidden {
			t.Fatalf("user referer with defaults off status=%d, want 403", code)
		}

		// An update that does not mention the switch keeps it off.
		if _, err := Update(ListsUpdate{Referers: "example.com"}); err != nil {
			t.Fatal(err)
		}
		if GetSnapshot().DefaultReferersEnabled {
			t.Fatal("an update without the switch re-enabled the built-in referers")
		}
		cfg, _, err := settings.GetWAFConfig()
		if err != nil || !cfg.DisableDefaultReferers {
			t.Fatalf("stored config = %+v, err=%v", cfg, err)
		}

		on := true
		if _, err := Update(ListsUpdate{DefaultReferersEnabled: &on}); err != nil {
			t.Fatal(err)
		}
		if code := refererStatus(t, r, builtIn); code != http.StatusForbidden {
			t.Fatalf("built-in referer after re-enabling status=%d, want 403", code)
		}
	})
}
