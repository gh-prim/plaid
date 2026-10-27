// Package handlers implements api.StrictServerInterface.
package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/api"
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
}

// Handlers implements every operation of the PLAID API.
type Handlers struct {
	db         Pinger
	accounts   *accounts.Service
	serverName string
}

var _ api.StrictServerInterface = (*Handlers)(nil)

// New returns the API handlers.
func New(d Deps) *Handlers {
	return &Handlers{db: d.DB, accounts: d.Accounts, serverName: d.ServerName}
}
