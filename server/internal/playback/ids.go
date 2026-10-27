package playback

import "github.com/google/uuid"

// newID returns a UUIDv7, like every PLAID id (F003 R24).
func newID() string { return uuid.Must(uuid.NewV7()).String() }
