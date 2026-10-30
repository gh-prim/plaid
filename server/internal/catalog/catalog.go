// Package catalog answers what the browsing screens ask (F008): sorted and
// filtered library grids, facets, search, the home and up next. It reads
// what F004, F005 and F006 write, plus the tables migration 0007 keeps in
// sync with triggers, and never writes.
package catalog

import (
	"database/sql"
	"time"
)

// Options configures a Catalog.
type Options struct {
	DB *sql.DB
	// Now is the clock (the home's daily order); defaults to time.Now.
	Now func() time.Time
}

// Catalog runs the catalog queries.
type Catalog struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Catalog.
func New(o Options) *Catalog {
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Catalog{db: o.DB, now: o.Now}
}
