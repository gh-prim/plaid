package handlers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oapi-codegen/nullable"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/library"
)

// Library limits (F004-R1, R19).
const (
	maxLibraryName    = 64
	maxLibraryFolders = 16
)

// libraryErr turns library errors into API errors. folders is the request's
// folder list, used to point at the invalid one.
func libraryErr(err error, folders []string) error {
	if errors.Is(err, library.ErrNotFound) {
		return apierr.NotFound("no such library")
	}
	if e, ok := errors.AsType[*library.FolderInvalidError](err); ok {
		field := "folders"
		if i := slices.Index(folders, e.Path); i >= 0 {
			field = fmt.Sprintf("folders/%d", i)
		}
		if e.Code == library.CodeFolderNotFound {
			return apierr.FolderNotFound(field, e.Path)
		}
		return apierr.FolderUnreadable(field, e.Path)
	}
	if e, ok := errors.AsType[*library.FolderOverlapError](err); ok {
		return apierr.FolderOverlap(e.Path, e.Other)
	}
	if errors.Is(err, library.ErrBadCursor) {
		return apierr.BadRequest("invalid cursor")
	}
	return err
}

type problems []apierr.Detail

func (p *problems) add(field, code string) { *p = append(*p, apierr.Detail{Field: field, Code: code}) }

func (p problems) err() error {
	if len(p) == 0 {
		return nil
	}
	return apierr.ValidationFailed(p...)
}

func checkLibraryName(name string, p *problems) string {
	v := strings.TrimSpace(name)
	switch n := utf8.RuneCountInString(v); {
	case n == 0:
		p.add("name", "too_short")
	case n > maxLibraryName:
		p.add("name", "too_long")
	}
	return v
}

func checkFolders(folders []string, p *problems) {
	switch {
	case len(folders) == 0:
		p.add("folders", "too_short")
	case len(folders) > maxLibraryFolders:
		p.add("folders", "too_long")
	}
}

func checkInterval(s int, p *problems) *int64 {
	d := time.Duration(s) * time.Second
	if d < library.MinScanInterval || d > library.MaxScanInterval {
		p.add("scanIntervalSeconds", "out_of_range")
	}
	v := int64(s)
	return &v
}

// ListLibraries lists every library; any signed-in account can (F004-R1).
func (h *Handlers) ListLibraries(ctx context.Context, _ api.ListLibrariesRequestObject) (api.ListLibrariesResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	views, err := h.library.Libraries(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListLibraries200JSONResponse, 0, len(views))
	for _, v := range views {
		out = append(out, toLibrary(v))
	}
	return out, nil
}

// CreateLibrary validates the folders, creates the library and queues a
// full scan.
func (h *Handlers) CreateLibrary(ctx context.Context, req api.CreateLibraryRequestObject) (api.CreateLibraryResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	var p problems
	n := library.NewLibrary{Name: checkLibraryName(b.Name, &p), Type: string(b.Type), Folders: b.Folders}
	if n.Type != library.TypeMovies && n.Type != library.TypeShows {
		p.add("type", "invalid")
	}
	checkFolders(b.Folders, &p)
	if b.ScanIntervalSeconds != nil {
		n.ScanIntervalS = checkInterval(*b.ScanIntervalSeconds, &p)
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	lib, err := h.library.CreateLibrary(ctx, n)
	if err != nil {
		return nil, libraryErr(err, b.Folders)
	}
	v, err := h.library.View(ctx, lib.ID)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.CreateLibrary201JSONResponse(toLibrary(v)), nil
}

// GetLibrary returns one library.
func (h *Handlers) GetLibrary(ctx context.Context, req api.GetLibraryRequestObject) (api.GetLibraryResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	v, err := h.library.View(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.GetLibrary200JSONResponse(toLibrary(v)), nil
}

// UpdateLibrary renames a library or changes its folders or interval.
func (h *Handlers) UpdateLibrary(ctx context.Context, req api.UpdateLibraryRequestObject) (api.UpdateLibraryResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	b, err := body(req.Body)
	if err != nil {
		return nil, err
	}
	current, err := h.library.Store().GetLibrary(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	if b.Type != nil && string(*b.Type) != current.Type {
		return nil, apierr.LibraryTypeImmutable()
	}
	var (
		p     problems
		patch library.Patch
	)
	if b.Name != nil {
		name := checkLibraryName(*b.Name, &p)
		patch.Name = &name
	}
	if b.Folders != nil {
		checkFolders(*b.Folders, &p)
		patch.Folders = *b.Folders
	}
	if b.ScanIntervalSeconds.IsSpecified() {
		patch.SetInterval = true
		if !b.ScanIntervalSeconds.IsNull() {
			patch.ScanIntervalS = checkInterval(b.ScanIntervalSeconds.MustGet(), &p)
		}
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	if _, err := h.library.UpdateLibrary(ctx, req.LibraryId, patch); err != nil {
		return nil, libraryErr(err, patch.Folders)
	}
	v, err := h.library.View(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.UpdateLibrary200JSONResponse(toLibrary(v)), nil
}

// DeleteLibrary deletes a library's rows; files on disk stay (F004-R23).
func (h *Handlers) DeleteLibrary(ctx context.Context, req api.DeleteLibraryRequestObject) (api.DeleteLibraryResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := h.library.DeleteLibrary(ctx, req.LibraryId); err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.DeleteLibrary204Response{}, nil
}

// GetLibraryScan returns the scan progress (F004-R21).
func (h *Handlers) GetLibraryScan(ctx context.Context, req api.GetLibraryScanRequestObject) (api.GetLibraryScanResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	pr, err := h.library.Progress(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.GetLibraryScan200JSONResponse(toScanProgress(pr)), nil
}

// ScanLibrary starts or queues a scan (F004-R20).
func (h *Handlers) ScanLibrary(ctx context.Context, req api.ScanLibraryRequestObject) (api.ScanLibraryResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	full := req.Body != nil && req.Body.Full != nil && *req.Body.Full
	if err := h.library.Scan(ctx, req.LibraryId, full); err != nil {
		return nil, libraryErr(err, nil)
	}
	pr, err := h.library.Progress(ctx, req.LibraryId)
	if err != nil {
		return nil, libraryErr(err, nil)
	}
	return api.ScanLibrary202JSONResponse(toScanProgress(pr)), nil
}

func toLibrary(v library.View) api.Library {
	out := api.Library{
		Id: v.ID, Name: v.Name, Type: api.LibraryType(v.Type), LastScanAt: v.LastScanAt,
		ScanIntervalSeconds: nullable.NewNullNullable[int](),
		Folders:             make([]api.LibraryFolder, 0, len(v.Folders)),
		Counts: api.LibraryCounts{
			Movies: v.Counts.Movies, Shows: v.Counts.Shows, Episodes: v.Counts.Episodes,
			Files: v.Counts.Files, Unrecognized: v.Counts.Unrecognized,
		},
		Scan: toScanProgress(v.Scan),
	}
	if v.ScanIntervalS != nil {
		out.ScanIntervalSeconds = nullable.NewNullableWithValue(int(*v.ScanIntervalS))
	}
	for _, f := range v.Folders {
		out.Folders = append(out.Folders, api.LibraryFolder{
			Id: f.ID, Path: f.Path, FsType: optional(f.FSType), Watched: f.Watched,
			Status: api.LibraryFolderStatus(f.Status),
		})
	}
	return out
}

func toScanProgress(p library.Progress) api.ScanProgress {
	r := p.Report
	out := api.ScanProgress{
		State: api.ScanState(p.State), Queued: p.Queued, Full: p.Full, CurrentFolder: optional(p.Folder),
		Counters: api.ScanCounters{
			Found: r.Found, New: r.New, Changed: r.Changed, Moved: r.Moved, Missing: r.Missing,
			Analyzed: r.Analyzed, Failed: r.Failed, Unrecognized: r.Unrecognized,
		},
	}
	if out.State == "" {
		out.State = api.Idle
	}
	if !p.StartedAt.IsZero() {
		t := p.StartedAt
		out.StartedAt = &t
	}
	return out
}
