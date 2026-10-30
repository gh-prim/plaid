package handlers

import (
	"context"
	"net/http"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
)

// notImplemented answers the F008 catalog routes until their plan step lands.
func notImplemented() error {
	return apierr.New(http.StatusNotImplemented, "not_implemented", "not implemented yet")
}

// GetLibraryFacets lists what a library's grid can be filtered by (F008-R27).
func (h *Handlers) GetLibraryFacets(_ context.Context, _ api.GetLibraryFacetsRequestObject) (api.GetLibraryFacetsResponseObject, error) {
	return nil, notImplemented()
}

// Search finds movies, shows, episodes and people by word prefix (F008-R28).
func (h *Handlers) Search(_ context.Context, _ api.SearchRequestObject) (api.SearchResponseObject, error) {
	return nil, notImplemented()
}

// GetHome returns the active profile's home (F008-R31).
func (h *Handlers) GetHome(_ context.Context, _ api.GetHomeRequestObject) (api.GetHomeResponseObject, error) {
	return nil, notImplemented()
}

// GetHomeRow pages one home row (F008-R31).
func (h *Handlers) GetHomeRow(_ context.Context, _ api.GetHomeRowRequestObject) (api.GetHomeRowResponseObject, error) {
	return nil, notImplemented()
}

// GetUpNext returns the episode to play next (F008-R32).
func (h *Handlers) GetUpNext(_ context.Context, _ api.GetUpNextRequestObject) (api.GetUpNextResponseObject, error) {
	return nil, notImplemented()
}
