// Package party is Plaid Party (F010): watching together. The store keeps
// what must outlive a connection (parties, invitations, guests, votes,
// follows); rooms live in memory (room.go).
package party

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/google/uuid"
)

// Party statuses (R1).
const (
	StatusScheduled = "scheduled"
	StatusVoting    = "voting"
	StatusLobby     = "lobby"
	StatusLive      = "live"
	StatusEnded     = "ended"
	StatusCancelled = "cancelled"
)

// Control modes (R14).
const (
	ControlHost     = "host"
	ControlEveryone = "everyone"
)

// Invitation answers (R28).
const (
	AnswerPending = "pending"
	AnswerYes     = "yes"
	AnswerMaybe   = "maybe"
	AnswerNo      = "no"
)

// Store errors.
var (
	ErrNotFound = errors.New("party_not_found")
	// ErrCodeTaken is returned when a fresh code collides with a live one;
	// callers draw another.
	ErrCodeTaken = errors.New("party_code_taken")
)

// Party is a stored party.
type Party struct {
	ID                 string
	Code               string
	Status             string
	ItemID             string
	FutureShowID       string
	FutureSeason       int
	FutureEpisode      int
	OrganizerUserID    string
	OrganizerProfileID string
	ControlMode        string
	ScheduledAt        *time.Time
	Note               string
	CreatedAt          time.Time
	StartedAt          *time.Time
	EndedAt            *time.Time
	SummaryJSON        string
}

// Invite is one person's invitation and answer.
type Invite struct {
	PartyID    string
	UserID     string
	Answer     string
	InvitedAt  time.Time
	AnsweredAt *time.Time
	RemindedAt *time.Time
}

// Guest is someone without an account who joined from the link (R5).
type Guest struct {
	ID          string
	PartyID     string
	FirstName   string
	AvatarColor int
	CreatedAt   time.Time
	RemovedAt   *time.Time
}

// Store is the party tables.
type Store struct {
	db *sql.DB
}

// NewStore returns a store on d.
func NewStore(d *sql.DB) *Store { return &Store{db: d} }

// timeLayout is fixed-width RFC 3339 UTC, so stored times sort as text.
const timeLayout = "2006-01-02T15:04:05.000Z"

func ts(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

func nullTS(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func optTS(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t := parseTS(s.String)
	return &t
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// NewID returns a UUIDv7.
func NewID() string { return uuid.Must(uuid.NewV7()).String() }

// codeAlphabet is F003-R20's: no 0 O 1 I L.
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// NewCode draws a 6-character party code (R2).
func NewCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// NormalizeCode accepts "k7p-4qx", "K7P4QX" or " K7P 4QX ".
func NormalizeCode(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(codeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatCode shows a code as XXX-XXX.
func FormatCode(code string) string {
	if len(code) != 6 {
		return code
	}
	return code[:3] + "-" + code[3:]
}

// NewToken returns a guest token and its hash: 32 random bytes, base64url,
// behind auth.GuestTokenPrefix so the middleware knows which lookup it needs.
func NewToken() (token string, hash []byte) {
	b := make([]byte, 32)
	rand.Read(b)
	token = auth.GuestTokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken is the stored form of a guest token.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

const partyCols = `id, code, status, item_id, future_show_id, future_season, future_episode,
	organizer_user_id, organizer_profile_id, control_mode, scheduled_at, note,
	created_at, started_at, ended_at, summary_json`

type scanner interface{ Scan(...any) error }

func scanParty(r scanner) (Party, error) {
	var p Party
	var item, fshow, oprof, note, summary sql.NullString
	var fseason, fep sql.NullInt64
	var sched, started, ended sql.NullString
	var created string
	err := r.Scan(&p.ID, &p.Code, &p.Status, &item, &fshow, &fseason, &fep,
		&p.OrganizerUserID, &oprof, &p.ControlMode, &sched, &note,
		&created, &started, &ended, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return Party{}, ErrNotFound
	}
	if err != nil {
		return Party{}, err
	}
	p.ItemID, p.FutureShowID, p.OrganizerProfileID = item.String, fshow.String, oprof.String
	p.FutureSeason, p.FutureEpisode = int(fseason.Int64), int(fep.Int64)
	p.Note, p.SummaryJSON = note.String, summary.String
	p.CreatedAt = parseTS(created)
	p.ScheduledAt, p.StartedAt, p.EndedAt = optTS(sched), optTS(started), optTS(ended)
	return p, nil
}

// Create inserts p with its invitations. A code collision with a live party
// returns ErrCodeTaken.
func (s *Store) Create(ctx context.Context, p Party, invitees []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO parties (`+partyCols+`)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Code, p.Status, nullStr(p.ItemID), nullStr(p.FutureShowID), nullInt(p.FutureSeason), nullInt(p.FutureEpisode),
		p.OrganizerUserID, nullStr(p.OrganizerProfileID), p.ControlMode, nullTS(p.ScheduledAt), nullStr(p.Note),
		ts(p.CreatedAt), nullTS(p.StartedAt), nullTS(p.EndedAt), nullStr(p.SummaryJSON))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: parties.code") {
			return ErrCodeTaken
		}
		return err
	}
	if err := addInvites(ctx, tx, p.ID, invitees, p.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func addInvites(ctx context.Context, tx *sql.Tx, partyID string, users []string, at time.Time) error {
	for _, u := range users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO party_invites (party_id, user_id, invited_at)
			VALUES (?,?,?) ON CONFLICT DO NOTHING`, partyID, u, ts(at)); err != nil {
			return err
		}
	}
	return nil
}

// Get returns a party by id.
func (s *Store) Get(ctx context.Context, id string) (Party, error) {
	return scanParty(s.db.QueryRowContext(ctx, `SELECT `+partyCols+` FROM parties WHERE id = ?`, id))
}

// ByCode returns the party, not ended, holding code.
func (s *Store) ByCode(ctx context.Context, code string) (Party, error) {
	return scanParty(s.db.QueryRowContext(ctx, `SELECT `+partyCols+` FROM parties
		WHERE code = ? AND status NOT IN ('ended','cancelled')`, NormalizeCode(code)))
}

func (s *Store) list(ctx context.Context, where string, args ...any) ([]Party, error) {
	//nolint:gosec // G202: where is one of this file's constant conditions
	rows, err := s.db.QueryContext(ctx, `SELECT `+partyCols+` FROM parties WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Party
	for rows.Next() {
		p, err := scanParty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ForUser lists the parties userID organizes or is invited to that aren't
// over, or ended in the last 30 days when withEnded.
func (s *Store) ForUser(ctx context.Context, userID string) ([]Party, error) {
	return s.list(ctx, `status NOT IN ('ended','cancelled') AND (organizer_user_id = ?
		OR id IN (SELECT party_id FROM party_invites WHERE user_id = ?))
		ORDER BY COALESCE(scheduled_at, created_at)`, userID, userID)
}

// Active lists every party not over (scheduler, admin).
func (s *Store) Active(ctx context.Context) ([]Party, error) {
	return s.list(ctx, `status NOT IN ('ended','cancelled') ORDER BY COALESCE(scheduled_at, created_at)`)
}

// Update writes p's mutable fields.
func (s *Store) Update(ctx context.Context, p Party) error {
	res, err := s.db.ExecContext(ctx, `UPDATE parties SET status = ?, item_id = ?, future_show_id = ?,
		future_season = ?, future_episode = ?, control_mode = ?, scheduled_at = ?, note = ?,
		started_at = ?, ended_at = ?, summary_json = ? WHERE id = ?`,
		p.Status, nullStr(p.ItemID), nullStr(p.FutureShowID), nullInt(p.FutureSeason), nullInt(p.FutureEpisode),
		p.ControlMode, nullTS(p.ScheduledAt), nullStr(p.Note), nullTS(p.StartedAt), nullTS(p.EndedAt),
		nullStr(p.SummaryJSON), p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// EndLive marks every live or lobby party ended: rooms don't survive a
// restart (spec, watch-outs).
func (s *Store) EndLive(ctx context.Context, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE parties SET status = 'ended', ended_at = ?
		WHERE status IN ('live','lobby') AND scheduled_at IS NULL`, ts(at))
	return err
}

// PurgeSummaries drops end summaries older than 30 days (R21).
func (s *Store) PurgeSummaries(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM parties WHERE status IN ('ended','cancelled')
		AND COALESCE(ended_at, created_at) < ?`, ts(before))
	return err
}

// Invite adds invitations; existing ones keep their answer.
func (s *Store) Invite(ctx context.Context, partyID string, users []string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := addInvites(ctx, tx, partyID, users, at); err != nil {
		return err
	}
	return tx.Commit()
}

// Invites lists a party's invitations.
func (s *Store) Invites(ctx context.Context, partyID string) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT party_id, user_id, answer, invited_at, answered_at, reminded_at
		FROM party_invites WHERE party_id = ? ORDER BY invited_at, user_id`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Invite
	for rows.Next() {
		var in Invite
		var invited string
		var answered, reminded sql.NullString
		if err := rows.Scan(&in.PartyID, &in.UserID, &in.Answer, &invited, &answered, &reminded); err != nil {
			return nil, err
		}
		in.InvitedAt, in.AnsweredAt, in.RemindedAt = parseTS(invited), optTS(answered), optTS(reminded)
		out = append(out, in)
	}
	return out, rows.Err()
}

// IsInvited reports whether userID holds an invitation to partyID.
func (s *Store) IsInvited(ctx context.Context, partyID, userID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM party_invites WHERE party_id = ? AND user_id = ?`,
		partyID, userID).Scan(&n)
	return n > 0, err
}

// Answer records userID's answer; ErrNotFound when they aren't invited.
func (s *Store) Answer(ctx context.Context, partyID, userID, answer string, at time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE party_invites SET answer = ?, answered_at = ?
		WHERE party_id = ? AND user_id = ?`, answer, ts(at), partyID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkReminded records that partyID's reminder went out.
func (s *Store) MarkReminded(ctx context.Context, partyID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE party_invites SET reminded_at = ?
		WHERE party_id = ? AND reminded_at IS NULL`, ts(at), partyID)
	return err
}

// AddGuest stores a guest and returns their token, which exists only here.
func (s *Store) AddGuest(ctx context.Context, g Guest) (Guest, string, error) {
	token, hash := NewToken()
	if g.ID == "" {
		g.ID = NewID()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO party_guests (id, party_id, first_name, avatar_color, token_hash, created_at)
		VALUES (?,?,?,?,?,?)`, g.ID, g.PartyID, g.FirstName, g.AvatarColor, hash, ts(g.CreatedAt))
	return g, token, err
}

// GuestByToken returns the live guest holding token, with their party's
// status. A removed guest or an ended party answers ErrNotFound.
func (s *Store) GuestByToken(ctx context.Context, token string) (Guest, error) {
	var g Guest
	var created string
	var removed sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT g.id, g.party_id, g.first_name, g.avatar_color, g.created_at, g.removed_at
		FROM party_guests g JOIN parties p ON p.id = g.party_id
		WHERE g.token_hash = ? AND g.removed_at IS NULL AND p.status NOT IN ('ended','cancelled')`,
		HashToken(token)).Scan(&g.ID, &g.PartyID, &g.FirstName, &g.AvatarColor, &created, &removed)
	if errors.Is(err, sql.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	g.CreatedAt, g.RemovedAt = parseTS(created), optTS(removed)
	return g, err
}

// Remove bars a person (userID or guestID) from coming back to partyID
// (R16). A removed guest's token dies.
func (s *Store) Remove(ctx context.Context, partyID, userID, guestID string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO party_removed (party_id, user_id, guest_id) VALUES (?,?,?)`,
		partyID, nullStr(userID), nullStr(guestID)); err != nil {
		return err
	}
	if guestID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE party_guests SET removed_at = ? WHERE id = ?`, ts(at), guestID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LookupGuest resolves a guest token for auth.Authenticate.
func (s *Store) LookupGuest(ctx context.Context, token string) (auth.Principal, error) {
	g, err := s.GuestByToken(ctx, token)
	if err != nil {
		return auth.Principal{}, auth.ErrNoSession
	}
	return auth.Principal{GuestID: g.ID, PartyID: g.PartyID}, nil
}

// IsRemoved reports whether userID was removed from partyID.
func (s *Store) IsRemoved(ctx context.Context, partyID, userID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM party_removed WHERE party_id = ? AND user_id = ?`,
		partyID, userID).Scan(&n)
	return n > 0, err
}

// GuestActive reports whether a guest can still use their token: not
// removed, party not over.
func (s *Store) GuestActive(ctx context.Context, guestID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM party_guests g JOIN parties p ON p.id = g.party_id
		WHERE g.id = ? AND g.removed_at IS NULL AND p.status NOT IN ('ended','cancelled')`, guestID).Scan(&n)
	return n > 0, err
}

// Item is what a party shows of its title.
type Item struct {
	ID        string
	Kind      string
	Title     string
	Year      int
	LibraryID string
	AgeRating *int
	// Episodes: their show, season and number.
	ShowID    string
	ShowTitle string
	Season    int
	Episode   int
	RuntimeMs int64
}

// Item returns an item with its matched title, or ErrNotFound.
func (s *Store) Item(ctx context.Context, id string) (Item, error) {
	var it Item
	var year, season, episode, runtime sql.NullInt64
	var age sql.NullInt64
	var show, showTitle sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT i.id, i.kind, COALESCE(m.title, i.title, ''), COALESCE(CAST(substr(m.release_date, 1, 4) AS INTEGER), i.year), i.library_id, m.age_rating,
		       sh.id, COALESCE(shm.title, sh.title), se.season_number, i.episode_number, m.runtime_minutes
		FROM items i
		LEFT JOIN item_metadata m ON m.item_id = i.id
		LEFT JOIN items se ON i.kind = 'episode' AND se.id = i.parent_id
		LEFT JOIN items sh ON sh.id = se.parent_id
		LEFT JOIN item_metadata shm ON shm.item_id = sh.id
		WHERE i.id = ?`, id).Scan(&it.ID, &it.Kind, &it.Title, &year, &it.LibraryID, &age,
		&show, &showTitle, &season, &episode, &runtime)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	it.Year, it.Season, it.Episode = int(year.Int64), int(season.Int64), int(episode.Int64)
	it.ShowID, it.ShowTitle = show.String, showTitle.String
	it.RuntimeMs = runtime.Int64 * 60_000
	if age.Valid {
		a := int(age.Int64)
		it.AgeRating = &a
	}
	return it, nil
}

// NextEpisode returns the episode after itemID in its show (next number in
// the season, else the first of the next season), "" when none.
func (s *Store) NextEpisode(ctx context.Context, itemID string) (string, error) {
	var next string
	err := s.db.QueryRowContext(ctx, `
		SELECT n.id FROM items cur
		JOIN items cs ON cs.id = cur.parent_id
		JOIN items ns ON ns.parent_id = cs.parent_id AND ns.kind = 'season' AND ns.hidden = 0
		JOIN items n ON n.parent_id = ns.id AND n.kind = 'episode' AND n.hidden = 0
		WHERE cur.id = ? AND cur.kind = 'episode'
		  AND (ns.season_number > cs.season_number
		       OR (ns.season_number = cs.season_number AND n.episode_number > cur.episode_number))
		  AND ns.season_number > 0
		ORDER BY ns.season_number, n.episode_number LIMIT 1`, itemID).Scan(&next)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return next, err
}

// Guest returns a guest by id.
func (s *Store) Guest(ctx context.Context, id string) (Guest, error) {
	var g Guest
	var created string
	var removed sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, party_id, first_name, avatar_color, created_at, removed_at
		FROM party_guests WHERE id = ?`, id).Scan(&g.ID, &g.PartyID, &g.FirstName, &g.AvatarColor, &created, &removed)
	if errors.Is(err, sql.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	g.CreatedAt, g.RemovedAt = parseTS(created), optTS(removed)
	return g, err
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Unwatched returns movies none of users' profiles has watched or started,
// newest first.
func (s *Store) Unwatched(ctx context.Context, users []string, limit int) ([]string, error) {
	if len(users) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(users)+1)
	for _, u := range users {
		args = append(args, u)
	}
	args = append(args, limit)
	//nolint:gosec // G202: only "?" placeholders are concatenated
	rows, err := s.db.QueryContext(ctx, `SELECT i.id FROM items i WHERE i.kind = 'movie' AND i.hidden = 0
		AND NOT EXISTS (SELECT 1 FROM watch_state w JOIN profiles pr ON pr.id = w.profile_id
			WHERE w.item_id = i.id AND (w.watched = 1 OR w.position_ms > 0)
			AND pr.user_id IN (?`+strings.Repeat(",?", len(users)-1)+`))
		ORDER BY i.created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
