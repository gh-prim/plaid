package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/gh-prim/plaid/server/internal/db"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// TestKeyOrder: the admin's key, then the environment, then the key built
// into release binaries (ADR 0007).
func TestKeyOrder(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	st := settings.New(d)
	defer func(old string) { builtinTMDBKey = old }(builtinTMDBKey)

	builtinTMDBKey = ""
	if k, src, _ := resolveKey(ctx, st, settings.TMDBAPIKey, "", builtinTMDBKey); k != "" || src != KeyNone {
		t.Errorf("nothing set = %q %q", k, src)
	}
	builtinTMDBKey = "built-in"
	if k, src, _ := resolveKey(ctx, st, settings.TMDBAPIKey, "", builtinTMDBKey); k != "built-in" || src != KeyBuiltin {
		t.Errorf("built in = %q %q", k, src)
	}
	if k, src, _ := resolveKey(ctx, st, settings.TMDBAPIKey, "from-env", builtinTMDBKey); k != "from-env" || src != KeyEnv {
		t.Errorf("env = %q %q", k, src)
	}
	if err := st.Set(ctx, settings.TMDBAPIKey, "saved"); err != nil {
		t.Fatal(err)
	}
	if k, src, _ := resolveKey(ctx, st, settings.TMDBAPIKey, "from-env", builtinTMDBKey); k != "saved" || src != KeyCustom {
		t.Errorf("saved = %q %q", k, src)
	}
}

// TestCheckKeyAgainstTheProvider: a key TMDB rejects isn't accepted.
func TestCheckKeyAgainstTheProvider(t *testing.T) {
	e := newApplyEnv(t, "movies", "en", "US", true)
	h := e.srv.HTTP()
	ctx := context.Background()
	if err := checkKeyWith(ctx, h, e.srv.URL, e.srv.URL+"/v3", "tmdb", "test-key-0000"); err != nil {
		t.Errorf("good TMDB key: %v", err)
	}
	if err := checkKeyWith(ctx, e.srv.HTTP(), e.srv.URL, e.srv.URL+"/v3", "tmdb", "wrong"); !errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("bad TMDB key: %v", err)
	}
	if err := checkKeyWith(ctx, e.srv.HTTP(), e.srv.URL, e.srv.URL+"/v3", "fanart", "wrong"); !errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("bad fanart.tv key: %v", err)
	}
}
