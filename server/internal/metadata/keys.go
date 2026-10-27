package metadata

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/settings"
)

// Built-in provider keys, set at release time with
// -ldflags "-X …/internal/metadata.builtinTMDBKey=…" (ADR 0007). Empty in
// dev builds and forks.
var builtinTMDBKey string

// Key sources, as GET /metadata/settings reports them (R34).
const (
	KeyCustom  = "custom"
	KeyEnv     = "env"
	KeyBuiltin = "builtin"
	KeyNone    = ""
)

// resolveKey applies ADR 0007's order: the key the admin saved, then the
// environment, then the built-in one.
func resolveKey(ctx context.Context, st *settings.Store, name, env, builtin string) (key, source string, err error) {
	if st != nil {
		v, ok, err := st.Get(ctx, name)
		if err != nil {
			return "", KeyNone, err
		}
		if ok && v != "" {
			return v, KeyCustom, nil
		}
	}
	if env != "" {
		return env, KeyEnv, nil
	}
	if builtin != "" {
		return builtin, KeyBuiltin, nil
	}
	return "", KeyNone, nil
}
