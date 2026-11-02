package party

import (
	"context"
	"testing"
	"time"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/auth"
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
	_, s := fixture(t)
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
	_, st := fixture(t)
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
	_, st := fixture(t)
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
