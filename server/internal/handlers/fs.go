package handlers

import (
	"context"
	"errors"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
)

// BrowseFolders lists sub-folders for the folder picker (F004-R3).
func (h *Handlers) BrowseFolders(ctx context.Context, req api.BrowseFoldersRequestObject) (api.BrowseFoldersResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	path := deref(req.Params.Path)
	res, err := library.Browse(path, h.library.BrowseRoots())
	switch {
	case errors.Is(err, library.ErrBadPath):
		return nil, apierr.BadRequest(err.Error())
	case errors.Is(err, library.ErrNotFound):
		return nil, apierr.NotFound("no such folder")
	case err != nil:
		if e, ok := errors.AsType[*library.FolderInvalidError](err); ok {
			return nil, apierr.FolderUnreadable("path", e.Path)
		}
		return nil, err
	}
	out := api.BrowseResult{Path: res.Path, Parent: nullable.NewNullNullable[string](),
		Entries: make([]api.BrowseEntry, 0, len(res.Entries))}
	if res.Parent != "" {
		out.Parent = nullable.NewNullableWithValue(res.Parent)
	}
	for _, e := range res.Entries {
		out.Entries = append(out.Entries, api.BrowseEntry{Name: e.Name, Path: e.Path, Readable: e.Readable})
	}
	return api.BrowseFolders200JSONResponse(out), nil
}
