package party

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
)

// The Upcoming tab (R35–R39): releases of the shows a profile follows and
// its parties.

// Releases is where episode air dates come from (TMDB).
type Releases interface {
	Show(ctx context.Context, id int, loc provider.Locale) (*provider.Details, error)
	Season(ctx context.Context, showID, number int, loc provider.Locale) (*provider.Season, error)
}

// Release window kept around today (R36).
const (
	releasesBack  = 7 * 24 * time.Hour
	releasesAhead = 60 * 24 * time.Hour
	releasesEvery = 24 * time.Hour
)

// Episode statuses (R37).
const (
	ReleaseOnServer = "released_on_server"
	ReleasePending  = "released_pending_file"
	ReleaseUpcoming = "upcoming"
	dateLayout      = "2006-01-02"
)

// followedSQL lists the shows a profile follows: started and not finished,
// or followed from the show page (R35). ? is the profile id, twice.
const followedSQL = `
	SELECT show_id FROM show_follows WHERE profile_id = ?
	UNION
	SELECT sh.id FROM items sh
	WHERE sh.kind = 'show' AND EXISTS (
		SELECT 1 FROM watch_state w JOIN items e ON e.id = w.item_id JOIN items se ON se.id = e.parent_id
		WHERE w.profile_id = ? AND se.parent_id = sh.id AND (w.watched = 1 OR w.position_ms > 0))
	AND EXISTS (
		SELECT 1 FROM items e JOIN items se ON se.id = e.parent_id
		WHERE se.parent_id = sh.id AND e.kind = 'episode' AND e.hidden = 0 AND se.season_number > 0
		AND NOT EXISTS (SELECT 1 FROM watch_state w WHERE w.item_id = e.id AND w.watched = 1 AND w.profile_id = ?))`

// Followed returns the show ids a profile follows.
func (st *Store) Followed(ctx context.Context, profileID string) ([]string, error) {
	return st.ids(ctx, followedSQL, profileID, profileID, profileID)
}

func (st *Store) ids(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := st.db.QueryContext(ctx, q, args...)
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

// SetFollow follows or unfollows a show.
func (st *Store) SetFollow(ctx context.Context, profileID, showID string, on bool, at time.Time) error {
	var err error
	if on {
		_, err = st.db.ExecContext(ctx, `INSERT INTO show_follows (profile_id, show_id, created_at) VALUES (?,?,?)
			ON CONFLICT DO NOTHING`, profileID, showID, ts(at))
	} else {
		_, err = st.db.ExecContext(ctx, `DELETE FROM show_follows WHERE profile_id = ? AND show_id = ?`, profileID, showID)
	}
	return err
}

// Follows reports whether a profile follows a show explicitly.
func (st *Store) Follows(ctx context.Context, profileID, showID string) (bool, error) {
	var n int
	err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM show_follows WHERE profile_id = ? AND show_id = ?`,
		profileID, showID).Scan(&n)
	return n > 0, err
}

// AllFollowed lists every followed show with its TMDB id, for the daily fetch.
func (st *Store) AllFollowed(ctx context.Context) (map[string]int, error) {
	rows, err := st.db.QueryContext(ctx, `
		SELECT DISTINCT sh.id, m.tmdb_id FROM items sh JOIN item_metadata m ON m.item_id = sh.id
		WHERE sh.kind = 'show' AND m.tmdb_id IS NOT NULL AND (
			sh.id IN (SELECT show_id FROM show_follows) OR EXISTS (
				SELECT 1 FROM watch_state w JOIN items e ON e.id = w.item_id JOIN items se ON se.id = e.parent_id
				WHERE se.parent_id = sh.id AND (w.watched = 1 OR w.position_ms > 0)))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var tmdb int
		if err := rows.Scan(&id, &tmdb); err != nil {
			return nil, err
		}
		out[id] = tmdb
	}
	return out, rows.Err()
}

// UpcomingEpisode is a stored release.
type UpcomingEpisode struct {
	ShowID  string
	Season  int
	Episode int
	Title   string
	AirDate string
}

// ReplaceReleases stores a show's releases in the window.
func (st *Store) ReplaceReleases(ctx context.Context, showID string, eps []UpcomingEpisode, at time.Time) error {
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM upcoming_episodes WHERE show_id = ?`, showID); err != nil {
		return err
	}
	for _, e := range eps {
		if _, err := tx.ExecContext(ctx, `INSERT INTO upcoming_episodes (show_id, season, episode, title, air_date, fetched_at)
			VALUES (?,?,?,?,?,?)`, showID, e.Season, e.Episode, nullStr(e.Title), e.AirDate, ts(at)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Releases lists stored releases of shows between two dates (inclusive).
func (st *Store) Releases(ctx context.Context, shows []string, from, to string) ([]UpcomingEpisode, error) {
	if len(shows) == 0 {
		return nil, nil
	}
	args := []any{from, to}
	for _, s := range shows {
		args = append(args, s)
	}
	//nolint:gosec // G202: only "?" placeholders are concatenated
	rows, err := st.db.QueryContext(ctx, `SELECT show_id, season, episode, COALESCE(title, ''), air_date FROM upcoming_episodes
		WHERE air_date BETWEEN ? AND ? AND show_id IN (?`+strings.Repeat(",?", len(shows)-1)+`)
		ORDER BY air_date, show_id, season, episode`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UpcomingEpisode
	for rows.Next() {
		var e UpcomingEpisode
		if err := rows.Scan(&e.ShowID, &e.Season, &e.Episode, &e.Title, &e.AirDate); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetAlert sets or clears "Notify me" on an episode (R38).
func (st *Store) SetAlert(ctx context.Context, profileID, showID string, season, episode int, on bool, at time.Time) error {
	var err error
	if on {
		_, err = st.db.ExecContext(ctx, `INSERT INTO release_alerts (profile_id, show_id, season, episode, created_at)
			VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`, profileID, showID, season, episode, ts(at))
	} else {
		_, err = st.db.ExecContext(ctx, `DELETE FROM release_alerts WHERE profile_id = ? AND show_id = ? AND season = ? AND episode = ?`,
			profileID, showID, season, episode)
	}
	return err
}

// Alerts returns a profile's pending alerts as "show|season|episode".
func (st *Store) Alerts(ctx context.Context, profileID string) (map[string]bool, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT show_id, season, episode FROM release_alerts
		WHERE profile_id = ? AND notified_at IS NULL`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var e UpcomingEpisode
		if err := rows.Scan(&e.ShowID, &e.Season, &e.Episode); err != nil {
			return nil, err
		}
		out[alertKey(e.ShowID, e.Season, e.Episode)] = true
	}
	return out, rows.Err()
}

func alertKey(show string, season, episode int) string {
	return show + "|" + strconv.Itoa(season) + "|" + strconv.Itoa(episode)
}

// FireAlerts marks and returns the alerts the new episode items answer:
// profile id → user id to notify.
func (st *Store) FireAlerts(ctx context.Context, itemIDs []string, at time.Time) ([][2]string, error) {
	var out [][2]string
	for _, id := range itemIDs {
		var show string
		var season, episode sql.NullInt64
		err := st.db.QueryRowContext(ctx, `SELECT se.parent_id, se.season_number, e.episode_number FROM items e
			JOIN items se ON se.id = e.parent_id WHERE e.id = ? AND e.kind = 'episode'`, id).Scan(&show, &season, &episode)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return out, err
		}
		rows, err := st.db.QueryContext(ctx, `UPDATE release_alerts SET notified_at = ?
			WHERE show_id = ? AND season = ? AND episode = ? AND notified_at IS NULL
			RETURNING profile_id, (SELECT user_id FROM profiles WHERE id = release_alerts.profile_id)`,
			ts(at), show, season.Int64, episode.Int64)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var p, u string
			if err := rows.Scan(&p, &u); err != nil {
				rows.Close()
				return out, err
			}
			out = append(out, [2]string{p, u})
		}
		rows.Close()
	}
	return out, nil
}

// FetchReleases refreshes the releases of every followed show (R36): the
// latest two seasons TMDB lists, kept from 7 days back to 60 ahead.
func (s *Service) FetchReleases(ctx context.Context) error {
	if s.o.Releases == nil {
		return nil
	}
	src := s.o.Releases()
	if src == nil {
		return nil
	}
	shows, err := s.store.AllFollowed(ctx)
	if err != nil {
		return err
	}
	now := s.o.Now()
	from, to := now.Add(-releasesBack).Format(dateLayout), now.Add(releasesAhead).Format(dateLayout)
	for showID, tmdb := range shows {
		d, err := src.Show(ctx, tmdb, provider.Locale{})
		if err != nil {
			s.o.Log.Warn("party: releases", "show_id", showID, "err", err)
			continue
		}
		var numbers []int
		for _, sn := range d.Seasons {
			if sn.Number > 0 {
				numbers = append(numbers, sn.Number)
			}
		}
		slices.Sort(numbers)
		if len(numbers) > 2 {
			numbers = numbers[len(numbers)-2:]
		}
		var eps []UpcomingEpisode
		for _, n := range numbers {
			season, err := src.Season(ctx, tmdb, n, provider.Locale{})
			if err != nil {
				continue
			}
			for _, e := range season.Episodes {
				if e.AirDate >= from && e.AirDate <= to {
					eps = append(eps, UpcomingEpisode{ShowID: showID, Season: e.Season, Episode: e.Number, Title: e.Name, AirDate: e.AirDate})
				}
			}
		}
		if err := s.store.ReplaceReleases(ctx, showID, eps, now); err != nil {
			return err
		}
	}
	s.o.Log.Info("party: releases refreshed", "shows", len(shows))
	return nil
}

// ItemsCreated is a library hook: new episodes fire "Notify me" alerts (R38).
func (s *Service) ItemsCreated(_ string, itemIDs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fired, err := s.store.FireAlerts(ctx, itemIDs, s.o.Now())
	if err != nil {
		s.o.Log.Error("party: release alerts", "err", err)
	}
	s.mu.Lock()
	hub := s.hub
	s.mu.Unlock()
	if hub == nil {
		return
	}
	for _, f := range fired {
		hub.SendToUser(f[1], "notify", map[string]any{"kind": "episode.available", "profileId": f[0]})
	}
}

// UpcomingEpisodeView is a release as Upcoming shows it.
type UpcomingEpisodeView struct {
	UpcomingEpisode
	ShowTitle string
	// BackdropID is the show's selected backdrop.
	BackdropID string
	Status     string
	ItemID     string
	Alert      bool
}

// UpcomingView is the Upcoming tab (R37).
type UpcomingView struct {
	Invitations []Party
	Episodes    []UpcomingEpisodeView
	Parties     []Party
}

// Upcoming assembles the tab for the caller's profile between two dates.
func (s *Service) Upcoming(ctx context.Context, p auth.Principal, from, to time.Time, kind string) (UpcomingView, error) {
	var out UpcomingView
	if p.IsGuest() {
		return out, errNotAllowed
	}
	if p.ProfileID == "" {
		return out, errProfileRequired
	}
	parties, err := s.store.ForUser(ctx, p.UserID)
	if err != nil {
		return out, err
	}
	if kind != "episodes" {
		for _, pt := range parties {
			if ans, _ := s.answerOf(ctx, pt.ID, p.UserID); ans == AnswerPending && pt.OrganizerUserID != p.UserID {
				out.Invitations = append(out.Invitations, pt)
			}
			when := pt.CreatedAt
			if pt.ScheduledAt != nil {
				when = *pt.ScheduledAt
			}
			if pt.Status == StatusLive || (!when.Before(from) && !when.After(to)) {
				out.Parties = append(out.Parties, pt)
			}
		}
	}
	if kind == "parties" {
		return out, nil
	}
	shows, err := s.store.Followed(ctx, p.ProfileID)
	if err != nil {
		return out, err
	}
	rel, err := s.store.Releases(ctx, shows, from.Format(dateLayout), to.Format(dateLayout))
	if err != nil {
		return out, err
	}
	alerts, err := s.store.Alerts(ctx, p.ProfileID)
	if err != nil {
		return out, err
	}
	today := s.o.Now().Format(dateLayout)
	shown := map[string]Item{}
	for _, e := range rel {
		v := UpcomingEpisodeView{UpcomingEpisode: e, Alert: alerts[alertKey(e.ShowID, e.Season, e.Episode)]}
		if _, ok := shown[e.ShowID]; !ok {
			shown[e.ShowID], _ = s.store.Item(ctx, e.ShowID)
		}
		v.ShowTitle, v.BackdropID = shown[e.ShowID].Title, shown[e.ShowID].BackdropID
		v.ItemID, _ = s.store.EpisodeOf(ctx, e.ShowID, e.Season, e.Episode)
		switch {
		case v.ItemID != "":
			v.Status = ReleaseOnServer
		case e.AirDate <= today:
			v.Status = ReleasePending
		default:
			v.Status = ReleaseUpcoming
		}
		out.Episodes = append(out.Episodes, v)
	}
	return out, nil
}

func (s *Service) answerOf(ctx context.Context, partyID, userID string) (string, error) {
	inv, err := s.store.Invites(ctx, partyID)
	if err != nil {
		return "", err
	}
	for _, in := range inv {
		if in.UserID == userID {
			return in.Answer, nil
		}
	}
	return "", nil
}

// Follow follows or unfollows a show for the caller's profile.
func (s *Service) Follow(ctx context.Context, p auth.Principal, showID string, on bool) error {
	if p.ProfileID == "" {
		return errProfileRequired
	}
	it, err := s.visibleShow(ctx, p, showID)
	if err != nil {
		return err
	}
	return s.store.SetFollow(ctx, p.ProfileID, it.ID, on, s.o.Now())
}

// Following reports whether the caller's profile follows a show (explicitly
// or because it started it).
func (s *Service) Following(ctx context.Context, p auth.Principal, showID string) (explicit, followed bool, err error) {
	if p.ProfileID == "" {
		return false, false, errProfileRequired
	}
	if _, err := s.visibleShow(ctx, p, showID); err != nil {
		return false, false, err
	}
	explicit, err = s.store.Follows(ctx, p.ProfileID, showID)
	if err != nil {
		return false, false, err
	}
	shows, err := s.store.Followed(ctx, p.ProfileID)
	return explicit, slices.Contains(shows, showID), err
}

// Alert sets "Notify me" on an episode of a show the caller can see.
func (s *Service) Alert(ctx context.Context, p auth.Principal, showID string, season, episode int, on bool) error {
	if p.ProfileID == "" {
		return errProfileRequired
	}
	if _, err := s.visibleShow(ctx, p, showID); err != nil {
		return err
	}
	return s.store.SetAlert(ctx, p.ProfileID, showID, season, episode, on, s.o.Now())
}

func (s *Service) visibleShow(ctx context.Context, p auth.Principal, showID string) (Item, error) {
	it, err := s.store.Item(ctx, showID)
	if err != nil || it.Kind != "show" {
		return Item{}, apiNotFoundItem
	}
	lim, err := s.o.Accounts.AccessLimit(ctx, p)
	if err != nil {
		return Item{}, err
	}
	if !lim.Allows(it.LibraryID, it.AgeRating) {
		return Item{}, apiNotFoundItem
	}
	return it, nil
}
