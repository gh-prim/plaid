package party

import (
	"context"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/metadata/provider"
	"github.com/gh-prim/plaid/server/internal/realtime"
)

// fakeAccounts lets everyone start parties and see everything.
type fakeAccounts struct{}

func (fakeAccounts) Party(context.Context) (accounts.PartySettings, error) {
	return accounts.PartySettings{Enabled: true, MaxPeople: 8}, nil
}
func (fakeAccounts) Access(context.Context, string) (accounts.Access, error) {
	return accounts.Access{Rights: accounts.Rights{Party: "start"}}, nil
}
func (fakeAccounts) AccessLimit(context.Context, auth.Principal) (access.Limit, error) {
	return access.Limit{}, nil
}
func (fakeAccounts) Me(_ context.Context, p auth.Principal) (accounts.Me, error) {
	return accounts.Me{Active: &accounts.Profile{ID: "p-" + p.UserID, Name: p.UserID}}, nil
}
func (fakeAccounts) ListUsers(context.Context) ([]accounts.User, error) {
	return []accounts.User{{ID: "alex", DisplayName: "Alex"}, {ID: "mia", DisplayName: "Mia"}, {ID: "sam", DisplayName: "Sam"}}, nil
}
func (fakeAccounts) DefaultProfiles(context.Context) (map[string]accounts.Profile, error) {
	return map[string]accounts.Profile{"alex": {ID: "p-alex"}, "mia": {ID: "p-mia"}, "sam": {ID: "p-sam"}}, nil
}

// showFixture adds a show with two seasons to the fixture database.
func showFixture(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	now := ts(t0)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO libraries (id, name, type, created_at, updated_at) VALUES ('shows','Séries','shows',?,?)`, now, now)
	exec(`INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES ('cam','shows','show','Caminandes',?,?)`, now, now)
	for _, se := range []int{1, 2} {
		sid := "cam-s" + string(rune('0'+se))
		exec(`INSERT INTO items (id, library_id, kind, parent_id, season_number, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
			sid, "shows", "season", "cam", se, now, now)
		for ep := 1; ep <= 2; ep++ {
			exec(`INSERT INTO items (id, library_id, kind, parent_id, title, episode_number, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
				sid+"e"+string(rune('0'+ep)), "shows", "episode", sid, "Episode", ep, now, now)
		}
	}
}

func TestNextEpisode(t *testing.T) {
	s := fixture(t)
	showFixture(t, s)
	ctx := context.Background()
	for cur, want := range map[string]string{"cam-s1e1": "cam-s1e2", "cam-s1e2": "cam-s2e1", "cam-s2e2": "", "nosferatu": ""} {
		got, err := s.NextEpisode(ctx, cur)
		if err != nil || got != want {
			t.Errorf("NextEpisode(%s) = %q, %v; want %q", cur, got, err, want)
		}
	}
}

// TestNextEpisodeTogether covers R30: at the credits the room announces
// the next episode, moves to it after 10 s and starts it once screens
// loaded it.
func TestNextEpisodeTogether(t *testing.T) {
	st := fixture(t)
	showFixture(t, st)
	now := t0
	svc := New(Options{Store: st, Accounts: fakeAccounts{}, Now: func() time.Time { return now }})
	ctx := context.Background()
	pt, err := svc.Create(ctx, auth.Principal{UserID: "alex", ProfileID: "p-alex"}, NewParty{ItemID: "cam-s1e1"})
	if err != nil {
		t.Fatal(err)
	}
	l := svc.roomOf(pt.ID)
	var sent []string
	l.room.emit = func(_ Target, typ string, _ any) { sent = append(sent, typ) }
	l.room.Join(now, alex, "c-alex", KindScreen, realtime.Device{})
	l.room.Report(now, "c-alex", Report{State: ScreenPlaying})
	l.room.Control(now, alex.ID, "play", ptr(0))

	l.mu.Lock()
	svc.itemEnded(ctx, l, now)
	l.mu.Unlock()
	if l.nextItem != "cam-s1e2" || !contains(sent, "party.next") {
		t.Fatalf("next = %q, sent %v", l.nextItem, sent)
	}
	now = now.Add(9 * time.Second)
	svc.tickNext(ctx, l, now)
	if l.room.ItemID != "cam-s1e1" {
		t.Fatal("moved before the countdown")
	}
	now = now.Add(time.Second)
	svc.tickNext(ctx, l, now)
	if l.room.ItemID != "cam-s1e2" || l.room.Playback().State != Paused {
		t.Fatalf("after the countdown: %s %+v", l.room.ItemID, l.room.Playback())
	}
	stored, _ := st.Get(ctx, pt.ID)
	if stored.ItemID != "cam-s1e2" {
		t.Errorf("stored item = %s", stored.ItemID)
	}
	// The screen loads the new episode: it starts for everyone at 0.
	l.room.Report(now, "c-alex", Report{State: ScreenReady})
	now = now.Add(time.Second)
	svc.tickNext(ctx, l, now)
	if pb := l.room.Playback(); pb.State != Playing || pb.PositionMs != 0 {
		t.Fatalf("next episode didn't start: %+v", pb)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestTally(t *testing.T) {
	props := []Proposal{{ItemID: "metropolis"}, {ItemID: "nosferatu"}, {ItemID: "sintel"}, {ItemID: "voyage"}}
	ballots := []Ballot{
		{UserID: "a", Ranks: []string{"metropolis", "nosferatu", "sintel"}, Veto: "voyage"},
		{UserID: "b", Ranks: []string{"nosferatu", "metropolis", "sintel"}},
		{UserID: "c", Ranks: []string{"metropolis", "sintel", "voyage"}},
	}
	r := Tally(props, ballots, "seed")
	if r.Winner != "metropolis" || r.Scores[0].Points != 8 || r.Scores[1].ItemID != "nosferatu" || r.Scores[1].Points != 5 {
		t.Errorf("tally = %+v", r)
	}
	last := r.Scores[len(r.Scores)-1]
	if last.ItemID != "voyage" || !last.Vetoed {
		t.Errorf("veto not last: %+v", r.Scores)
	}

	// A tie is drawn from the seed: the same seed gives the same winner.
	tie := []Ballot{{UserID: "a", Ranks: []string{"metropolis", "nosferatu"}}, {UserID: "b", Ranks: []string{"nosferatu", "metropolis"}}}
	r1 := Tally(props[:2], tie, "abc")
	r2 := Tally(props[:2], tie, "abc")
	if len(r1.Tied) != 2 || r1.Winner == "" || r1.Winner != r2.Winner || r1.Seed != "abc" || Draw(r1.Tied, "abc") != r1.Winner {
		t.Errorf("tie = %+v / %+v", r1, r2)
	}
	// Every proposal vetoed, or no ballot: the host picks.
	if r := Tally(props[:1], []Ballot{{UserID: "a", Ranks: []string{}, Veto: "metropolis"}}, ""); r.Winner != "" {
		t.Errorf("vetoed winner = %+v", r)
	}
	if r := Tally(props, nil, ""); r.Winner != "" {
		t.Errorf("winner without ballots = %+v", r)
	}
}

// TestVoteLifecycle covers R31–R33: phases, two proposals each, ballots
// kept to their voter, and the close picking the winner.
func TestVoteLifecycle(t *testing.T) {
	st := fixture(t)
	ctx := context.Background()
	now := t0
	for _, id := range []string{"metropolis", "sintel"} {
		st.db.ExecContext(ctx, `INSERT INTO items (id, library_id, kind, title, created_at, updated_at) VALUES (?,'lib','movie',?,?,?)`,
			id, id, ts(t0), ts(t0))
	}
	svc := New(Options{Store: st, Accounts: fakeAccounts{}, Now: func() time.Time { return now }})
	alexP := auth.Principal{UserID: "alex", ProfileID: "p-alex"}
	miaP := auth.Principal{UserID: "mia", ProfileID: "p-mia"}
	sched := now.Add(5 * 24 * time.Hour)
	pt, err := svc.Create(ctx, alexP, NewParty{ScheduledAt: &sched, Invite: []string{"mia"},
		Vote: &VoteDeadlines{ProposalsUntil: now.Add(48 * time.Hour), RankingUntil: now.Add(72 * time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	if pt.Status != StatusVoting {
		t.Fatalf("status = %s", pt.Status)
	}
	for _, id := range []string{"metropolis", "sintel"} {
		if _, err := svc.Propose(ctx, alexP, pt.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Propose(ctx, alexP, pt.ID, "nosferatu"); err == nil {
		t.Error("third proposal accepted")
	}
	if _, err := svc.Propose(ctx, miaP, pt.ID, "sintel"); err == nil {
		t.Error("same title proposed twice")
	}
	v, err := svc.Propose(ctx, miaP, pt.ID, "nosferatu")
	if err != nil || len(v.Proposals) != 3 || v.Phase != PhaseProposals {
		t.Fatalf("proposals = %+v, %v", v, err)
	}
	if _, err := svc.Cast(ctx, miaP, pt.ID, []string{"nosferatu"}, ""); err == nil {
		t.Error("ballot during proposals")
	}

	now = now.Add(49 * time.Hour)
	if _, err := svc.Cast(ctx, miaP, pt.ID, []string{"nosferatu", "nosferatu"}, ""); err == nil {
		t.Error("duplicate ranking accepted")
	}
	if _, err := svc.Cast(ctx, miaP, pt.ID, []string{"nosferatu", "metropolis"}, "sintel"); err != nil {
		t.Fatal(err)
	}
	v, err = svc.Cast(ctx, alexP, pt.ID, []string{"metropolis", "sintel", "nosferatu"}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Alex sees who voted and his own ballot, never Mia's.
	if len(v.Mine.Ranks) != 3 || v.Mine.Ranks[0] != "metropolis" || v.Result != nil {
		t.Errorf("alex's view = %+v", v)
	}
	for _, voter := range v.Voters {
		if !voter.Voted {
			t.Errorf("voter %s not marked voted", voter.Name)
		}
	}

	now = now.Add(24 * time.Hour)
	if err := svc.closeVote(ctx, pt); err != nil {
		t.Fatal(err)
	}
	got, _ := st.Get(ctx, pt.ID)
	// metropolis 3+2 = 5, nosferatu 3+1 = 4, sintel vetoed.
	if got.ItemID != "metropolis" || got.Status != StatusScheduled {
		t.Errorf("after close: item %q status %s", got.ItemID, got.Status)
	}
	vote, _ := st.Vote(ctx, pt.ID)
	if vote.Seed == "" || vote.ClosedAt == nil {
		t.Errorf("vote = %+v", vote)
	}
}

// TestScheduler covers R29 and the catch-up after a restart: the lobby and
// reminder 15 minutes before, the start at the time, and a party nobody
// started cancelled 3 hours later.
func TestScheduler(t *testing.T) {
	st := fixture(t)
	ctx := context.Background()
	now := t0
	svc := New(Options{Store: st, Accounts: fakeAccounts{}, Now: func() time.Time { return now }})
	alexP := auth.Principal{UserID: "alex", ProfileID: "p-alex"}
	at := now.Add(time.Hour)
	pt, err := svc.Create(ctx, alexP, NewParty{ItemID: "nosferatu", ScheduledAt: &at, Invite: []string{"mia", "sam"}})
	if err != nil {
		t.Fatal(err)
	}
	st.Answer(ctx, pt.ID, "mia", AnswerYes, now)

	now = at.Add(-20 * time.Minute)
	svc.schedule(ctx)
	if got, _ := st.Get(ctx, pt.ID); got.Status != StatusScheduled || svc.roomOf(pt.ID) != nil {
		t.Fatalf("early: %s", got.Status)
	}
	now = at.Add(-15 * time.Minute)
	svc.schedule(ctx)
	got, _ := st.Get(ctx, pt.ID)
	l := svc.roomOf(pt.ID)
	if got.Status != StatusLobby || l == nil {
		t.Fatalf("lobby: %s, room %v", got.Status, l != nil)
	}
	inv, _ := st.Invites(ctx, pt.ID)
	for _, in := range inv {
		// Every invite is marked so nobody is reminded twice; only yes and
		// maybe got the notification.
		if in.RemindedAt == nil {
			t.Errorf("%s not marked reminded", in.UserID)
		}
	}
	l.room.Join(now, alex, "c-alex", KindScreen, realtime.Device{})
	l.room.Report(now, "c-alex", Report{State: ScreenReady})

	now = at
	svc.schedule(ctx)
	svc.tick(ctx)
	if pb := l.room.Playback(); pb.State != Playing || pb.PositionMs != 0 {
		t.Fatalf("not started at the time: %+v", pb)
	}
	if got, _ := st.Get(ctx, pt.ID); got.Status != StatusLive || got.StartedAt == nil {
		t.Errorf("stored = %+v", got)
	}

	// A party the server slept through is cancelled once 3 hours passed.
	at2 := now.Add(time.Hour)
	stale, _ := svc.Create(ctx, alexP, NewParty{ItemID: "nosferatu", ScheduledAt: &at2})
	now = at2.Add(3 * time.Hour)
	svc.schedule(ctx)
	if got, _ := st.Get(ctx, stale.ID); got.Status != StatusCancelled {
		t.Errorf("stale party = %s", got.Status)
	}
}

// fakeReleases is TMDB for Caminandes: two seasons, season 2 airing.
type fakeReleases struct{}

func (fakeReleases) Show(context.Context, int, provider.Locale) (*provider.Details, error) {
	return &provider.Details{Seasons: []provider.SeasonRef{{Number: 0}, {Number: 1}, {Number: 2}}}, nil
}

func (fakeReleases) Season(_ context.Context, _, n int, _ provider.Locale) (*provider.Season, error) {
	day := func(d int) string { return t0.AddDate(0, 0, d).Format("2006-01-02") }
	if n == 1 {
		return &provider.Season{Number: 1, Episodes: []provider.Episode{{Season: 1, Number: 1, AirDate: day(-400)}}}, nil
	}
	return &provider.Season{Number: 2, Episodes: []provider.Episode{
		{Season: 2, Number: 1, Name: "Llamigos", AirDate: day(-2)},
		{Season: 2, Number: 3, Name: "Le Phare", AirDate: day(-1)},
		{Season: 2, Number: 4, Name: "Les Lunes", AirDate: day(3)},
		{Season: 2, Number: 5, AirDate: day(90)},
	}}, nil
}

// TestUpcoming covers R35–R38: a started show is followed, its releases
// come from TMDB with their status, and "Notify me" fires when the file
// lands.
func TestUpcoming(t *testing.T) {
	st := fixture(t)
	showFixture(t, st)
	ctx := context.Background()
	now := t0
	for _, q := range []string{
		`INSERT INTO item_metadata (item_id, tmdb_id, title, updated_at) VALUES ('cam', 4242, 'Caminandes', '` + ts(t0) + `')`,
		`INSERT INTO watch_state (profile_id, item_id, position_ms, updated_at) VALUES ('p-mia','cam-s1e1',60000,'` + ts(t0) + `')`,
	} {
		if _, err := st.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	svc := New(Options{Store: st, Accounts: fakeAccounts{}, Now: func() time.Time { return now },
		Releases: func() Releases { return fakeReleases{} }})
	if err := svc.FetchReleases(ctx); err != nil {
		t.Fatal(err)
	}
	mia := auth.Principal{UserID: "mia", ProfileID: "p-mia"}
	v, err := svc.Upcoming(ctx, mia, now.AddDate(0, 0, -7), now.AddDate(0, 0, 60), "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Episodes) != 3 {
		t.Fatalf("episodes = %+v", v.Episodes)
	}
	want := map[int]string{1: ReleaseOnServer, 3: ReleasePending, 4: ReleaseUpcoming}
	for _, e := range v.Episodes {
		if e.Status != want[e.Episode] || e.ShowTitle != "Caminandes" {
			t.Errorf("S2E%d = %s %q", e.Episode, e.Status, e.ShowTitle)
		}
	}
	// Sam never started it: nothing, until he follows it.
	sam := auth.Principal{UserID: "sam", ProfileID: "p-sam"}
	if v, _ := svc.Upcoming(ctx, sam, now, now.AddDate(0, 0, 60), "episodes"); len(v.Episodes) != 0 {
		t.Errorf("sam sees %+v", v.Episodes)
	}
	if err := svc.Follow(ctx, sam, "cam", true); err != nil {
		t.Fatal(err)
	}
	if v, _ := svc.Upcoming(ctx, sam, now, now.AddDate(0, 0, 60), "episodes"); len(v.Episodes) != 1 {
		t.Errorf("after follow, sam sees %+v", v.Episodes)
	}

	// Notify me on S2E4, then the scan adds it.
	if err := svc.Alert(ctx, mia, "cam", 2, 4, true); err != nil {
		t.Fatal(err)
	}
	st.db.ExecContext(ctx, `INSERT INTO items (id, library_id, kind, parent_id, episode_number, created_at, updated_at)
		VALUES ('cam-s2e4','shows','episode','cam-s2',4,?,?)`, ts(t0), ts(t0))
	fired, err := st.FireAlerts(ctx, []string{"cam-s2e4", "nosferatu"}, now)
	if err != nil || len(fired) != 1 || fired[0] != [2]string{"p-mia", "mia"} {
		t.Fatalf("fired = %v, %v", fired, err)
	}
	if again, _ := st.FireAlerts(ctx, []string{"cam-s2e4"}, now); len(again) != 0 {
		t.Error("alert fired twice")
	}
}
