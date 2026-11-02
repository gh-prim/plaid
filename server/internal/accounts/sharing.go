package accounts

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// Qualities a group can cap playback at, best first (F009).
var qualities = []string{"original", "2160p", "1080p", "720p", "480p"}

// Party rights, least first.
var partyRights = []string{"none", "join", "start"}

const maxGroupName = 40

// Rights are what a group, or a person through their groups, may do.
type Rights struct {
	MaxQuality string
	// MaxStreams is the number of streams at the same time, nil for no limit.
	MaxStreams        *int
	Downloads         bool
	Party             string
	CanCreateProfiles bool
}

// FullRights are an admin's and a household group's defaults.
func FullRights() Rights {
	return Rights{MaxQuality: "original", Downloads: true, Party: "start", CanCreateProfiles: true}
}

// noRights is the starting point of a union: the least of everything.
func noRights() Rights {
	one := 1
	return Rights{MaxQuality: qualities[len(qualities)-1], MaxStreams: &one, Party: "none"}
}

// union returns the most permissive of a and b, field by field.
func union(a, b Rights) Rights {
	out := a
	if slices.Index(qualities, b.MaxQuality) < slices.Index(qualities, a.MaxQuality) {
		out.MaxQuality = b.MaxQuality
	}
	switch {
	case a.MaxStreams == nil || b.MaxStreams == nil:
		out.MaxStreams = nil
	case *b.MaxStreams > *a.MaxStreams:
		out.MaxStreams = b.MaxStreams
	}
	out.Downloads = a.Downloads || b.Downloads
	if slices.Index(partyRights, b.Party) > slices.Index(partyRights, a.Party) {
		out.Party = b.Party
	}
	out.CanCreateProfiles = a.CanCreateProfiles || b.CanCreateProfiles
	return out
}

// RightsOverride are rights given to one person on top of their groups.
// They only add; nil takes the groups' value.
type RightsOverride struct {
	MaxQuality        *string
	MaxStreams        *int
	UnlimitedStreams  *bool
	Downloads         *bool
	Party             *string
	CanCreateProfiles *bool
}

// apply adds o to r.
func (o RightsOverride) apply(r Rights) Rights {
	add := r
	if o.MaxQuality != nil {
		add.MaxQuality = *o.MaxQuality
	}
	if o.MaxStreams != nil {
		add.MaxStreams = o.MaxStreams
	}
	if o.UnlimitedStreams != nil && *o.UnlimitedStreams {
		add.MaxStreams = nil
	}
	if o.Downloads != nil {
		add.Downloads = *o.Downloads
	}
	if o.Party != nil {
		add.Party = *o.Party
	}
	if o.CanCreateProfiles != nil {
		add.CanCreateProfiles = *o.CanCreateProfiles
	}
	return union(r, add)
}

func checkRights(field string, r Rights, p *problems) {
	if !slices.Contains(qualities, r.MaxQuality) {
		p.add(field+"/maxQuality", "invalid")
	}
	if r.MaxStreams != nil && (*r.MaxStreams < 1 || *r.MaxStreams > 20) {
		p.add(field+"/maxStreams", "out_of_range")
	}
	if !slices.Contains(partyRights, r.Party) {
		p.add(field+"/party", "invalid")
	}
}

func checkOverride(field string, o RightsOverride, p *problems) {
	if o.MaxQuality != nil && !slices.Contains(qualities, *o.MaxQuality) {
		p.add(field+"/maxQuality", "invalid")
	}
	if o.MaxStreams != nil && (*o.MaxStreams < 1 || *o.MaxStreams > 20) {
		p.add(field+"/maxStreams", "out_of_range")
	}
	if o.Party != nil && !slices.Contains(partyRights, *o.Party) {
		p.add(field+"/party", "invalid")
	}
}

// Group is a set of people who get the same access (F009).
type Group struct {
	ID        string
	Name      string
	Household bool
	// LibraryIDs are the shared libraries; a household group sees them all.
	LibraryIDs []string
	Rights     Rights
	MemberIDs  []string
}

// Access is what an account gets from its role, groups and exceptions.
type Access struct {
	// LibraryIDs are the visible libraries, nil for all of them.
	LibraryIDs []string
	Rights     Rights
}

// UserSharing is how an admin set up one person's access.
type UserSharing struct {
	GroupIDs        []string
	ExtraLibraryIDs []string
	ExtraRights     RightsOverride
}

// ListGroups returns every group by name, with libraries and members.
func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	return s.listGroups(ctx, s.db, "")
}

func (s *Service) listGroups(ctx context.Context, q querier, id string) ([]Group, error) {
	query := `SELECT id, name, household, max_quality, max_streams, downloads, party, can_create_profiles FROM groups`
	var args []any
	if id != "" {
		query += ` WHERE id = ?`
		args = append(args, id)
	}
	rows, err := q.QueryContext(ctx, query+` ORDER BY name COLLATE NOCASE, id`, args...)
	if err != nil {
		return nil, err
	}
	var out []Group
	index := map[string]int{}
	for rows.Next() {
		var (
			g                          Group
			household, dl, canProfiles int
			streams                    sql.NullInt64
		)
		if err := rows.Scan(&g.ID, &g.Name, &household, &g.Rights.MaxQuality, &streams, &dl, &g.Rights.Party, &canProfiles); err != nil {
			rows.Close()
			return nil, err
		}
		g.Household, g.Rights.Downloads, g.Rights.CanCreateProfiles = household == 1, dl == 1, canProfiles == 1
		if streams.Valid {
			n := int(streams.Int64)
			g.Rights.MaxStreams = &n
		}
		g.LibraryIDs, g.MemberIDs = []string{}, []string{}
		index[g.ID] = len(out)
		out = append(out, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, link := range []struct {
		query string
		add   func(g *Group, v string)
	}{
		{`SELECT group_id, library_id FROM group_libraries ORDER BY library_id`, func(g *Group, v string) { g.LibraryIDs = append(g.LibraryIDs, v) }},
		{`SELECT m.group_id, m.user_id FROM group_members m JOIN users u ON u.id = m.user_id ORDER BY u.created_at, u.id`,
			func(g *Group, v string) { g.MemberIDs = append(g.MemberIDs, v) }},
	} {
		rows, err := q.QueryContext(ctx, link.query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var gid, v string
			if err := rows.Scan(&gid, &v); err != nil {
				rows.Close()
				return nil, err
			}
			if i, ok := index[gid]; ok {
				link.add(&out[i], v)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) getGroup(ctx context.Context, q querier, id string) (Group, error) {
	groups, err := s.listGroups(ctx, q, id)
	if err != nil {
		return Group{}, err
	}
	if len(groups) == 0 {
		return Group{}, apierr.NotFound("no such group")
	}
	return groups[0], nil
}

// NewGroup is the input of CreateGroup.
type NewGroup struct {
	Name       string
	Household  bool
	LibraryIDs []string
	// Rights defaults to watching in 1080p, two streams, joining parties.
	Rights *Rights
}

// defaultGroupRights are a new non-household group's rights (canvas Share03).
func defaultGroupRights() Rights {
	two := 2
	return Rights{MaxQuality: "1080p", MaxStreams: &two, Party: "join"}
}

// CreateGroup creates a sharing group (F009).
func (s *Service) CreateGroup(ctx context.Context, actor auth.Principal, in NewGroup) (Group, error) {
	var p problems
	in.Name = checkLength("name", strings.TrimSpace(in.Name), maxGroupName, &p)
	rights := defaultGroupRights()
	if in.Household {
		rights = FullRights()
	}
	if in.Rights != nil {
		rights = *in.Rights
		checkRights("rights", rights, &p)
	}
	if err := p.err(); err != nil {
		return Group{}, err
	}
	id := newID()
	var g Group
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		now := ts(s.now())
		_, err := tx.ExecContext(ctx, `INSERT INTO groups (id, name, household, max_quality, max_streams, downloads, party,
			can_create_profiles, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.Name, boolInt(in.Household), rights.MaxQuality, rights.MaxStreams, boolInt(rights.Downloads), rights.Party,
			boolInt(rights.CanCreateProfiles), now, now)
		if isUniqueViolation(err, "groups.name") {
			return apierr.ValidationFailed(apierr.Detail{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		if err := s.setGroupLibraries(ctx, tx, id, in.Household, in.LibraryIDs); err != nil {
			return err
		}
		g, err = s.getGroup(ctx, tx, id)
		return err
	})
	if err == nil {
		s.logger.Info("group created", "group_id", id, "by", actor.UserID)
	}
	return g, err
}

// GroupPatch lists the fields of a group update; nil means unchanged.
type GroupPatch struct {
	Name       *string
	Household  *bool
	LibraryIDs *[]string
	Rights     *Rights
}

// UpdateGroup changes a group. It applies at once to everyone in it.
func (s *Service) UpdateGroup(ctx context.Context, actor auth.Principal, id string, patch GroupPatch) (Group, error) {
	var p problems
	if patch.Name != nil {
		v := checkLength("name", strings.TrimSpace(*patch.Name), maxGroupName, &p)
		patch.Name = &v
	}
	if patch.Rights != nil {
		checkRights("rights", *patch.Rights, &p)
	}
	if err := p.err(); err != nil {
		return Group{}, err
	}
	var g Group
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		cur, err := s.getGroup(ctx, tx, id)
		if err != nil {
			return err
		}
		if patch.Name != nil {
			cur.Name = *patch.Name
		}
		if patch.Household != nil {
			cur.Household = *patch.Household
		}
		if patch.Rights != nil {
			cur.Rights = *patch.Rights
		}
		r := cur.Rights
		_, err = tx.ExecContext(ctx, `UPDATE groups SET name = ?, household = ?, max_quality = ?, max_streams = ?, downloads = ?,
			party = ?, can_create_profiles = ?, updated_at = ? WHERE id = ?`,
			cur.Name, boolInt(cur.Household), r.MaxQuality, r.MaxStreams, boolInt(r.Downloads), r.Party,
			boolInt(r.CanCreateProfiles), ts(s.now()), id)
		if isUniqueViolation(err, "groups.name") {
			return apierr.ValidationFailed(apierr.Detail{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		libs := cur.LibraryIDs
		if patch.LibraryIDs != nil {
			libs = *patch.LibraryIDs
		}
		if err := s.setGroupLibraries(ctx, tx, id, cur.Household, libs); err != nil {
			return err
		}
		g, err = s.getGroup(ctx, tx, id)
		return err
	})
	if err == nil {
		s.logger.Info("group updated", "group_id", id, "by", actor.UserID)
	}
	return g, err
}

// DeleteGroup deletes a group; its members keep their other groups.
func (s *Service) DeleteGroup(ctx context.Context, actor auth.Principal, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apierr.NotFound("no such group")
	}
	s.logger.Info("group deleted", "group_id", id, "by", actor.UserID)
	return nil
}

// checkLibraries refuses unknown libraries, and household-only ones unless
// household is set (F009).
func checkLibraries(ctx context.Context, q querier, field string, ids []string, household bool) error {
	for i, id := range ids {
		var only int
		err := q.QueryRowContext(ctx, `SELECT household_only FROM libraries WHERE id = ?`, id).Scan(&only)
		if errors.Is(err, sql.ErrNoRows) {
			return apierr.ValidationFailed(apierr.Detail{Field: field + "/" + strconv.Itoa(i), Code: "not_found"})
		}
		if err != nil {
			return err
		}
		if only == 1 && !household {
			return apierr.ValidationFailed(apierr.Detail{Field: field + "/" + strconv.Itoa(i), Code: "household_only"})
		}
	}
	return nil
}

func (s *Service) setGroupLibraries(ctx context.Context, tx querier, groupID string, household bool, ids []string) error {
	ids = dedupe(ids)
	if err := checkLibraries(ctx, tx, "libraryIds", ids, household); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_libraries WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_libraries (group_id, library_id) VALUES (?, ?)`, groupID, id); err != nil {
			return err
		}
	}
	return nil
}

// setUserGroups replaces the groups of a user.
func setUserGroups(ctx context.Context, tx querier, userID string, ids []string) error {
	ids = dedupe(ids)
	for i, id := range ids {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM groups WHERE id = ?)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return apierr.ValidationFailed(apierr.Detail{Field: "groupIds/" + strconv.Itoa(i), Code: "not_found"})
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`, id, userID); err != nil {
			return err
		}
	}
	return nil
}

func setUserLibraries(ctx context.Context, tx querier, userID string, ids []string) error {
	ids = dedupe(ids)
	if err := checkLibraries(ctx, tx, "extraLibraryIds", ids, false); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_libraries WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_libraries (user_id, library_id) VALUES (?, ?)`, userID, id); err != nil {
			return err
		}
	}
	return nil
}

func setUserRights(ctx context.Context, tx querier, userID string, o RightsOverride) error {
	nb := func(b *bool) any {
		if b == nil {
			return nil
		}
		return boolInt(*b)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO user_rights (user_id, max_quality, max_streams, unlimited_streams, downloads, party,
		can_create_profiles) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET max_quality = excluded.max_quality, max_streams = excluded.max_streams,
		unlimited_streams = excluded.unlimited_streams, downloads = excluded.downloads, party = excluded.party,
		can_create_profiles = excluded.can_create_profiles`,
		userID, o.MaxQuality, o.MaxStreams, nb(o.UnlimitedStreams), nb(o.Downloads), o.Party, nb(o.CanCreateProfiles))
	return err
}

// Sharing returns how every account's access is set up, by user id.
func (s *Service) Sharing(ctx context.Context) (map[string]UserSharing, error) {
	out := map[string]UserSharing{}
	get := func(id string) UserSharing {
		u, ok := out[id]
		if !ok {
			u = UserSharing{GroupIDs: []string{}, ExtraLibraryIDs: []string{}}
		}
		return u
	}
	for _, link := range []struct {
		query string
		add   func(u *UserSharing, v string)
	}{
		{`SELECT m.user_id, m.group_id FROM group_members m JOIN groups g ON g.id = m.group_id ORDER BY g.name COLLATE NOCASE`,
			func(u *UserSharing, v string) { u.GroupIDs = append(u.GroupIDs, v) }},
		{`SELECT user_id, library_id FROM user_libraries ORDER BY library_id`,
			func(u *UserSharing, v string) { u.ExtraLibraryIDs = append(u.ExtraLibraryIDs, v) }},
	} {
		rows, err := s.db.QueryContext(ctx, link.query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var uid, v string
			if err := rows.Scan(&uid, &v); err != nil {
				rows.Close()
				return nil, err
			}
			u := get(uid)
			link.add(&u, v)
			out[uid] = u
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, `+overrideColumns+` FROM user_rights`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid string
		o, err := scanOverride(rows, &uid)
		if err != nil {
			return nil, err
		}
		u := get(uid)
		u.ExtraRights = o
		out[uid] = u
	}
	return out, rows.Err()
}

const overrideColumns = `max_quality, max_streams, unlimited_streams, downloads, party, can_create_profiles`

func scanOverride(row interface{ Scan(...any) error }, lead ...any) (RightsOverride, error) {
	var (
		quality, party                sql.NullString
		streams, unlimited, dl, canPr sql.NullInt64
		o                             RightsOverride
	)
	if err := row.Scan(append(lead, &quality, &streams, &unlimited, &dl, &party, &canPr)...); err != nil {
		return o, err
	}
	if quality.Valid {
		o.MaxQuality = &quality.String
	}
	if streams.Valid {
		n := int(streams.Int64)
		o.MaxStreams = &n
	}
	b := func(v sql.NullInt64) *bool {
		if !v.Valid {
			return nil
		}
		x := v.Int64 == 1
		return &x
	}
	o.UnlimitedStreams, o.Downloads, o.CanCreateProfiles = b(unlimited), b(dl), b(canPr)
	if party.Valid {
		o.Party = &party.String
	}
	return o, nil
}

// Access returns what userID may see and do: everything for an admin,
// else the union of their groups plus their exceptions (F009).
func (s *Service) Access(ctx context.Context, userID string) (Access, error) {
	var role auth.Role
	err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return Access{}, apierr.NotFound("no such user")
	}
	if err != nil {
		return Access{}, err
	}
	if role == auth.RoleAdmin {
		return Access{Rights: FullRights()}, nil
	}

	rights, household, grouped := noRights(), false, false
	rows, err := s.db.QueryContext(ctx, `SELECT g.household, g.max_quality, g.max_streams, g.downloads, g.party, g.can_create_profiles
		FROM groups g JOIN group_members m ON m.group_id = g.id WHERE m.user_id = ?`, userID)
	if err != nil {
		return Access{}, err
	}
	for rows.Next() {
		var (
			r                   Rights
			hh, dl, canProfiles int
			streams             sql.NullInt64
		)
		if err := rows.Scan(&hh, &r.MaxQuality, &streams, &dl, &r.Party, &canProfiles); err != nil {
			rows.Close()
			return Access{}, err
		}
		if streams.Valid {
			n := int(streams.Int64)
			r.MaxStreams = &n
		}
		r.Downloads, r.CanCreateProfiles = dl == 1, canProfiles == 1
		household = household || hh == 1
		if grouped {
			rights = union(rights, r)
		} else {
			rights, grouped = r, true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Access{}, err
	}
	o, err := scanOverride(s.db.QueryRowContext(ctx, `SELECT `+overrideColumns+` FROM user_rights WHERE user_id = ?`, userID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Access{}, err
	}
	rights = o.apply(rights)
	if household {
		return Access{Rights: rights}, nil
	}

	// Household-only libraries never reach a non-household person, even if
	// a row says otherwise.
	libs := []string{}
	rows, err = s.db.QueryContext(ctx, `SELECT l.id FROM libraries l WHERE l.household_only = 0 AND (
			l.id IN (SELECT gl.library_id FROM group_libraries gl JOIN group_members m ON m.group_id = gl.group_id WHERE m.user_id = ?)
			OR l.id IN (SELECT library_id FROM user_libraries WHERE user_id = ?))
		ORDER BY l.id`, userID, userID)
	if err != nil {
		return Access{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return Access{}, err
		}
		libs = append(libs, id)
	}
	return Access{LibraryIDs: libs, Rights: rights}, rows.Err()
}

// Seen is an account's most recently used device.
type Seen struct {
	At         string
	SessionID  string
	DeviceName string
	DeviceType string
}

// LastSeen returns, by user id, the account's most recently used session.
func (s *Service) LastSeen(ctx context.Context) (map[string]Seen, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, last_seen_at, id, device_name, device_type FROM (
		SELECT user_id, last_seen_at, id, device_name, device_type,
			ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY last_seen_at DESC, id DESC) AS rn FROM sessions
	) WHERE rn = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Seen{}
	for rows.Next() {
		var (
			id string
			v  Seen
		)
		if err := rows.Scan(&id, &v.At, &v.SessionID, &v.DeviceName, &v.DeviceType); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}

func dedupe(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
