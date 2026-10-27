// Package handlers implements api.StrictServerInterface.
package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/api"
)

// Pinger is the part of *sql.DB the handlers need to check database health.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Deps are the services the handlers use. Each feature adds its own field.
type Deps struct {
	DB Pinger
}

// Handlers implements every operation of the PLAID API.
type Handlers struct {
	db Pinger
}

var _ api.StrictServerInterface = (*Handlers)(nil)

// New returns the API handlers.
func New(d Deps) *Handlers {
	return &Handlers{db: d.DB}
}
