// Package access holds the content rules that depend on who is watching:
// the age limit of the active profile (F003 R18) and the libraries the
// account was given (F009), enforced on every item read (F005 R25). F006
// (playback) and F008 (search) apply the same Limit. It depends on nothing,
// so any package can use it.
package access

import "strings"

// Limit is what the active profile may see. The zero Limit allows
// everything.
type Limit struct {
	// MaxAge is the profile's age limit, nil for none.
	MaxAge *int
	// AllowUnrated lets the profile see items without a known age rating.
	// It only matters when MaxAge is set.
	AllowUnrated bool
	// Libraries are the ids of the libraries the account may see, nil for
	// every library (admins). A non-nil empty slice allows none (F009).
	Libraries []string
}

// Unrestricted reports whether l lets everything through.
func (l Limit) Unrestricted() bool { return l.MaxAge == nil && l.Libraries == nil }

// AllowsLibrary reports whether the library's items can be shown.
func (l Limit) AllowsLibrary(libraryID string) bool {
	if l.Libraries == nil {
		return true
	}
	for _, id := range l.Libraries {
		if id == libraryID {
			return true
		}
	}
	return false
}

// AllowsAge reports whether an item with this age rating (nil = unrated)
// can be shown, whatever its library.
func (l Limit) AllowsAge(ageRating *int) bool {
	if l.MaxAge == nil {
		return true
	}
	if ageRating == nil {
		return l.AllowUnrated
	}
	return *ageRating <= *l.MaxAge
}

// Allows reports whether an item of this library with this age rating (nil
// = unrated) can be shown.
func (l Limit) Allows(libraryID string, ageRating *int) bool {
	return l.AllowsLibrary(libraryID) && l.AllowsAge(ageRating)
}

// SQL returns a WHERE condition and its arguments for items aliased as
// items whose item_metadata row is joined as meta (LEFT JOIN, so items
// without a row count as unrated). Seasons and episodes carry their show's
// age_rating (F005 R24) and library, so no parent lookup is needed. It
// returns "" when l is unrestricted.
func (l Limit) SQL(items, meta string) (cond string, args []any) {
	var conds []string
	if l.Libraries != nil {
		if len(l.Libraries) == 0 {
			conds = append(conds, "0")
		} else {
			conds = append(conds, items+".library_id IN (?"+strings.Repeat(",?", len(l.Libraries)-1)+")")
			for _, id := range l.Libraries {
				args = append(args, id)
			}
		}
	}
	if l.MaxAge != nil {
		col := meta + ".age_rating"
		conds = append(conds, "(("+col+" IS NOT NULL AND "+col+" <= ?) OR ("+col+" IS NULL AND ?))")
		args = append(args, *l.MaxAge, l.AllowUnrated)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "(" + strings.Join(conds, " AND ") + ")", args
}
