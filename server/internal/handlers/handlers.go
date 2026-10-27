// Package handlers implements api.StrictServerInterface.
package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/library"
	"github.com/gh-prim/plaid/server/internal/metadata"
	"github.com/gh-prim/plaid/server/internal/playback"
)

// Pinger is the part of *sql.DB the handlers need to check database health.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Deps are the services the handlers use. Each feature adds its own field.
type Deps struct {
	DB         Pinger
	Accounts   *accounts.Service
	ServerName string
	Library    *library.Engine
	// Metadata is F005; nil leaves items with their parsed data only.
	Metadata *metadata.Service
	// Playback is F006; nil makes playback routes answer as if FFmpeg
	// and every session were missing.
	Playback *playback.Service
	// Streams is cancelled when the server shuts down, which ends
	// long-lived responses such as server-sent events. Nil never ends them.
	Streams context.Context
}

// Handlers implements every operation of the PLAID API.
type Handlers struct {
	db         Pinger
	accounts   *accounts.Service
	serverName string
	library    *library.Engine
	metadata   *metadata.Service
	playback   *playback.Service
	streams    context.Context
}

var _ api.StrictServerInterface = (*Handlers)(nil)

// New returns the API handlers.
func New(d Deps) *Handlers {
	return &Handlers{db: d.DB, accounts: d.Accounts, serverName: d.ServerName, library: d.Library, metadata: d.Metadata, playback: d.Playback, streams: d.Streams}
}
