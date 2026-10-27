// Package access holds the content rules that depend on who is watching:
// today the age limit of the active profile (F003 R18), enforced on every
// item read (F005 R25). F006 (playback) and F008 (search) apply the same
// Limit. It depends on nothing, so any package can use it.
package access

// Limit is what the active profile may see. The zero Limit allows
// everything.
type Limit struct {
	// MaxAge is the profile's age limit, nil for none.
	MaxAge *int
	// AllowUnrated lets the profile see items without a known age rating.
	// It only matters when MaxAge is set.
	AllowUnrated bool
}

// Unrestricted reports whether l lets everything through.
func (l Limit) Unrestricted() bool { return l.MaxAge == nil }

// Allows reports whether an item with this age rating (nil = unrated) can
// be shown.
func (l Limit) Allows(ageRating *int) bool {
	if l.MaxAge == nil {
		return true
	}
	if ageRating == nil {
		return l.AllowUnrated
	}
	return *ageRating <= *l.MaxAge
}

// SQL returns a WHERE condition and its arguments for items whose
// item_metadata row is joined as alias (LEFT JOIN, so items without a row
// count as unrated). Seasons and episodes carry their show's age_rating
// (F005 R24), so no parent lookup is needed. It returns "" when l is
// unrestricted.
func (l Limit) SQL(alias string) (cond string, args []any) {
	if l.MaxAge == nil {
		return "", nil
	}
	col := alias + ".age_rating"
	return "((" + col + " IS NOT NULL AND " + col + " <= ?) OR (" + col + " IS NULL AND ?))",
		[]any{*l.MaxAge, l.AllowUnrated}
}
