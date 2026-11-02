package party

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gh-prim/plaid/server/internal/access"
	"github.com/gh-prim/plaid/server/internal/accounts"
	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/clientinfo"
	"github.com/gh-prim/plaid/server/internal/realtime"
)

// Accounts is what parties need from F003 and F009.
type Accounts interface {
	Party(ctx context.Context) (accounts.PartySettings, error)
	Access(ctx context.Context, userID string) (accounts.Access, error)
	AccessLimit(ctx context.Context, p auth.Principal) (access.Limit, error)
	Me(ctx context.Context, p auth.Principal) (accounts.Me, error)
	ListUsers(ctx context.Context) ([]accounts.User, error)
	DefaultProfiles(ctx context.Context) (map[string]accounts.Profile, error)
}

// Sender is the hub side the service uses.
type Sender interface {
	SendToUser(userID, typ string, data any)
	Find(match func(*realtime.Conn) bool) []*realtime.Conn
}

// Options configures a Service.
type Options struct {
	Store    *Store
	Accounts Accounts
	Log      *slog.Logger
	Now      func() time.Time
	// Tick is how often rooms run their timers (250 ms).
	Tick time.Duration
	// Releases returns TMDB for the Upcoming tab's air dates, nil without
	// a key (R36).
	Releases func() Releases
}

// Service runs Plaid Party: the stored parties and the live rooms. It is
// the hub's realtime.Handler.
type Service struct {
	o     Options
	store *Store

	mu    sync.Mutex
	hub   Sender
	rooms map[string]*live
	conns map[string]*realtime.Conn
	// names keeps people's names after they left, for summaries.
	names map[string]map[string]string
	// codes slows down guessing party codes (R3), by address and account.
	codes *auth.Backoff
	// lastReleases is when the releases were last fetched (R36).
	lastReleases time.Time
}

// CodeBackoff: 10 wrong codes in 10 minutes wait 15 minutes (R3).
var CodeBackoff = auth.BackoffPolicy{Threshold: 10, Base: 15 * time.Minute, Max: 15 * time.Minute, Forget: 10 * time.Minute}

// codeKey is who a wrong code counts against.
func codeKey(ctx context.Context, p auth.Principal) string {
	if p.UserID != "" {
		return "u:" + p.UserID
	}
	return "ip:" + clientinfo.FromContext(ctx).IP.String()
}

// byCode finds a party by code, counting misses toward the backoff.
func (s *Service) byCode(ctx context.Context, key, code string) (Party, error) {
	if wait := s.codes.Wait(key); wait > 0 {
		return Party{}, apierr.TooManyAttempts(wait)
	}
	pt, err := s.store.ByCode(ctx, code)
	if errors.Is(err, ErrNotFound) {
		s.codes.Fail(key)
		return Party{}, errNotFound
	}
	return pt, err
}

// live is a room and the party it belongs to, behind one lock.
type live struct {
	mu    sync.Mutex
	party Party
	room  *Room
	// blocked are people whose screen got transcode_limit_reached (R26),
	// with the quality they wanted.
	blocked map[string]string
	// nextAt is when a next episode starts (R30), zero when none pending.
	nextAt   time.Time
	nextItem string
	// startBy starts the room's item once screens are ready, or then
	// (lobby, next episode).
	startBy time.Time
}

// Errors of the service, as API errors.
var (
	errNotAllowed      = apierr.New(403, "party_not_allowed", "Plaid Party isn't open to you")
	errOff             = apierr.New(403, "party_disabled", "Plaid Party is turned off on this server")
	errAgeLimit        = apierr.New(403, "party_age_limit", "this title isn't for this profile")
	errNotFound        = apierr.NotFound("no such party")
	errNotStarted      = &realtime.Error{Code: "party_not_started"}
	errProfileRequired = apierr.ProfileRequired()
	apiNotFoundItem    = apierr.NotFound("no such item")
)

// New returns a service. Call SetHub before serving sockets, then Run.
func New(o Options) *Service {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Tick == 0 {
		o.Tick = 250 * time.Millisecond
	}
	return &Service{o: o, store: o.Store, rooms: map[string]*live{}, conns: map[string]*realtime.Conn{},
		names: map[string]map[string]string{}, codes: auth.NewBackoff(CodeBackoff, o.Now)}
}

// Store returns the party tables.
func (s *Service) Store() *Store { return s.store }

// SetHub gives the service the hub it sends through.
func (s *Service) SetHub(h Sender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hub = h
}

// Run ends the rooms a restart lost, then runs room timers until ctx ends.
func (s *Service) Run(ctx context.Context) {
	if err := s.store.EndLive(ctx, s.o.Now()); err != nil {
		s.o.Log.Error("party: ending lost rooms", "err", err)
	}
	t := time.NewTicker(s.o.Tick)
	defer t.Stop()
	sched := time.NewTicker(30 * time.Second)
	defer sched.Stop()
	s.schedule(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		case <-sched.C:
			s.schedule(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	now := s.o.Now()
	for _, l := range s.liveRooms() {
		l.mu.Lock()
		ended := l.room.Tick(now)
		s.tickNext(ctx, l, now)
		l.mu.Unlock()
		if ended {
			s.finish(ctx, l)
		}
	}
}

func (s *Service) liveRooms() []*live {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*live, 0, len(s.rooms))
	for _, l := range s.rooms {
		out = append(out, l)
	}
	return out
}

func (s *Service) roomOf(partyID string) *live {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rooms[partyID]
}

// emitter routes a room's events to connections.
func (s *Service) emitter(partyID string) Emit {
	return func(to Target, typ string, data any) {
		s.mu.Lock()
		l := s.rooms[partyID]
		s.mu.Unlock()
		var ids []string
		switch {
		case to.Conn != "":
			ids = []string{to.Conn}
		case l == nil:
			return
		case to.All:
			ids = l.room.Connections("")
		case to.Person != "":
			ids = l.room.Connections(to.Person)
		case to.Host:
			ids = l.room.HostConnections()
		}
		s.sendConns(ids, typ, data)
	}
}

func (s *Service) sendConns(ids []string, typ string, data any) {
	s.mu.Lock()
	conns := make([]*realtime.Conn, 0, len(ids))
	for _, id := range ids {
		if c := s.conns[id]; c != nil {
			conns = append(conns, c)
		}
	}
	s.mu.Unlock()
	for _, c := range conns {
		c.Send(typ, data)
	}
}

// openRoom creates the live room of p (started now or in the lobby).
func (s *Service) openRoom(ctx context.Context, p Party) (*live, error) {
	set, err := s.o.Accounts.Party(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if l := s.rooms[p.ID]; l != nil {
		return l, nil
	}
	l := &live{party: p, blocked: map[string]string{}}
	l.room = NewRoom(p.ID, p.ItemID, p.ControlMode, set.MaxPeople, s.o.Now(), nil)
	l.room.emit = s.emitter(p.ID)
	if p.OrganizerUserID != "" {
		// The organizer hosts until they hand over (R1).
		l.room.SetHost(PersonID(p.OrganizerUserID, ""))
	}
	s.rooms[p.ID] = l
	s.names[p.ID] = map[string]string{}
	return l, nil
}

// finish stores the end of a room and forgets it.
func (s *Service) finish(ctx context.Context, l *live) {
	l.mu.Lock()
	now := s.o.Now()
	s.mu.Lock()
	names := s.names[l.party.ID]
	s.mu.Unlock()
	sum := l.room.Summarize(now, names)
	p := l.party
	l.mu.Unlock()
	raw, _ := json.Marshal(sum)
	p.Status, p.EndedAt, p.SummaryJSON = StatusEnded, &now, string(raw)
	if err := s.store.Update(context.WithoutCancel(ctx), p); err != nil {
		s.o.Log.Error("party: storing the end", "party_id", p.ID, "err", err)
	}
	s.mu.Lock()
	delete(s.rooms, p.ID)
	delete(s.names, p.ID)
	s.mu.Unlock()
	s.o.Log.Info("party ended", "party_id", p.ID, "people", len(sum.People), "reactions", sum.Reactions)
}

// --- Rights ---------------------------------------------------------------

func (s *Service) right(ctx context.Context, p auth.Principal) (string, error) {
	if p.IsAdmin() {
		return "start", nil
	}
	acc, err := s.o.Accounts.Access(ctx, p.UserID)
	if err != nil {
		return "", err
	}
	return acc.Rights.Party, nil
}

func (s *Service) enabled(ctx context.Context) (accounts.PartySettings, error) {
	set, err := s.o.Accounts.Party(ctx)
	if err != nil {
		return set, err
	}
	if !set.Enabled {
		return set, errOff
	}
	return set, nil
}

// visible returns an item the caller's profile may see.
func (s *Service) visible(ctx context.Context, p auth.Principal, itemID string) (Item, error) {
	it, err := s.store.Item(ctx, itemID)
	if errors.Is(err, ErrNotFound) {
		return Item{}, apierr.NotFound("no such item")
	}
	if err != nil {
		return Item{}, err
	}
	lim, err := s.o.Accounts.AccessLimit(ctx, p)
	if err != nil {
		return Item{}, err
	}
	if !lim.Allows(it.LibraryID, it.AgeRating) {
		return Item{}, apierr.NotFound("no such item")
	}
	if it.Kind != "movie" && it.Kind != "episode" {
		return Item{}, apierr.NotPlayable("only movies and episodes can be watched together")
	}
	return it, nil
}

// --- REST side ------------------------------------------------------------

// NewParty is the input of Create.
type NewParty struct {
	ItemID      string
	ControlMode string
	ScheduledAt *time.Time
	Note        string
	Invite      []string
	// Vote opens a vote instead of an item (R31).
	Vote *VoteDeadlines
	// Future episode (R38): show and numbers when ItemID is empty.
	ShowID          string
	Season, Episode int
}

// VoteDeadlines are a vote's two deadlines.
type VoteDeadlines struct {
	ProposalsUntil, RankingUntil time.Time
}

// Limits of a party's fields (R28).
const (
	maxNote         = 140
	minScheduleLead = 15 * time.Minute
	maxScheduleLead = 60 * 24 * time.Hour
)

// Create creates a party (R1, R2, R28, R31).
func (s *Service) Create(ctx context.Context, p auth.Principal, in NewParty) (Party, error) {
	if p.IsGuest() {
		return Party{}, errNotAllowed
	}
	if _, err := s.enabled(ctx); err != nil {
		return Party{}, err
	}
	right, err := s.right(ctx, p)
	if err != nil {
		return Party{}, err
	}
	if right != "start" {
		return Party{}, errNotAllowed
	}
	now := s.o.Now()
	var probs []apierr.Detail
	if in.ControlMode == "" {
		in.ControlMode = ControlHost
	}
	if in.ControlMode != ControlHost && in.ControlMode != ControlEveryone {
		probs = append(probs, apierr.Detail{Field: "controlMode", Code: "invalid"})
	}
	in.Note = strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(in.Note) > maxNote {
		probs = append(probs, apierr.Detail{Field: "note", Code: "too_long"})
	}
	if in.ScheduledAt != nil {
		lead := in.ScheduledAt.Sub(now)
		if lead < minScheduleLead || lead > maxScheduleLead {
			probs = append(probs, apierr.Detail{Field: "scheduledAt", Code: "out_of_range"})
		}
	}
	if in.Vote != nil {
		v := in.Vote
		if in.ScheduledAt == nil || !now.Before(v.ProposalsUntil) || !v.ProposalsUntil.Before(v.RankingUntil) ||
			v.RankingUntil.After(*in.ScheduledAt) {
			probs = append(probs, apierr.Detail{Field: "vote", Code: "out_of_range"})
		}
	}
	if in.ItemID == "" && in.Vote == nil && in.ShowID == "" {
		probs = append(probs, apierr.Detail{Field: "itemId", Code: "required"})
	}
	if in.ShowID != "" && in.ScheduledAt == nil {
		probs = append(probs, apierr.Detail{Field: "scheduledAt", Code: "required"})
	}
	if len(probs) > 0 {
		return Party{}, apierr.ValidationFailed(probs...)
	}
	if in.ItemID != "" {
		if _, err := s.visible(ctx, p, in.ItemID); err != nil {
			return Party{}, err
		}
	}
	invite, err := s.invitable(ctx, p, in.Invite)
	if err != nil {
		return Party{}, err
	}
	pt := Party{
		ID: NewID(), ItemID: in.ItemID, OrganizerUserID: p.UserID, OrganizerProfileID: p.ProfileID,
		ControlMode: in.ControlMode, ScheduledAt: in.ScheduledAt, Note: in.Note, CreatedAt: now,
		FutureShowID: in.ShowID, FutureSeason: in.Season, FutureEpisode: in.Episode,
	}
	switch {
	case in.Vote != nil:
		pt.Status = StatusVoting
	case in.ScheduledAt != nil:
		pt.Status = StatusScheduled
	default:
		pt.Status = StatusLive
		pt.StartedAt = &now
	}
	for range 10 {
		pt.Code = NewCode()
		err = s.store.Create(ctx, pt, invite)
		if !errors.Is(err, ErrCodeTaken) {
			break
		}
	}
	if err != nil {
		return Party{}, err
	}
	if in.Vote != nil {
		if err := s.store.OpenVote(ctx, pt.ID, in.Vote.ProposalsUntil, in.Vote.RankingUntil); err != nil {
			return Party{}, err
		}
	}
	if pt.Status == StatusLive {
		if _, err := s.openRoom(ctx, pt); err != nil {
			return Party{}, err
		}
	}
	s.notifyInvites(pt, invite, "party.invite")
	s.o.Log.Info("party created", "party_id", pt.ID, "by", p.UserID, "status", pt.Status, "invited", len(invite))
	return pt, nil
}

// invitable keeps the people who may join parties, the caller excluded.
func (s *Service) invitable(ctx context.Context, p auth.Principal, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	people, err := s.People(ctx, p)
	if err != nil {
		return nil, err
	}
	ok := map[string]bool{}
	for _, x := range people {
		ok[x.UserID] = true
	}
	var out []string
	for _, id := range ids {
		if !ok[id] {
			return nil, apierr.ValidationFailed(apierr.Detail{Field: "invite", Code: "not_found"})
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// Invitee is someone the caller can invite (R28): a display name and the
// default profile's picture, never a username.
type Invitee struct {
	UserID      string
	Name        string
	AvatarColor int
	Profile     accounts.Profile
}

// People lists who can be invited: accounts with the join or start right,
// other than the caller, not disabled.
func (s *Service) People(ctx context.Context, p auth.Principal) ([]Invitee, error) {
	if p.IsGuest() {
		return nil, errNotAllowed
	}
	users, err := s.o.Accounts.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	profiles, err := s.o.Accounts.DefaultProfiles(ctx)
	if err != nil {
		return nil, err
	}
	var out []Invitee
	for _, u := range users {
		if u.ID == p.UserID || u.Disabled {
			continue
		}
		if u.Role != auth.RoleAdmin {
			acc, err := s.o.Accounts.Access(ctx, u.ID)
			if err != nil {
				return nil, err
			}
			if acc.Rights.Party == "none" {
				continue
			}
		}
		pr := profiles[u.ID]
		out = append(out, Invitee{UserID: u.ID, Name: u.DisplayName, AvatarColor: pr.AvatarColor, Profile: pr})
	}
	return out, nil
}

// notifyInvites tells invitees over the socket (R41).
func (s *Service) notifyInvites(p Party, users []string, kind string) {
	s.mu.Lock()
	hub := s.hub
	s.mu.Unlock()
	if hub == nil {
		return
	}
	for _, u := range users {
		hub.SendToUser(u, "notify", map[string]any{"kind": kind, "partyId": p.ID, "status": p.Status})
	}
}

// Get returns a party the caller may see: organizer, invited, in its room,
// or a guest of it.
func (s *Service) Get(ctx context.Context, p auth.Principal, id string) (Party, error) {
	pt, err := s.store.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Party{}, errNotFound
	}
	if err != nil {
		return Party{}, err
	}
	if !s.mayRead(ctx, p, pt) {
		return Party{}, errNotFound
	}
	return pt, nil
}

func (s *Service) mayRead(ctx context.Context, p auth.Principal, pt Party) bool {
	switch {
	case p.IsGuest():
		return p.PartyID == pt.ID
	case p.IsAdmin(), pt.OrganizerUserID == p.UserID:
		return true
	}
	if ok, _ := s.store.IsInvited(ctx, pt.ID, p.UserID); ok {
		return true
	}
	if l := s.roomOf(pt.ID); l != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.room.Has(PersonID(p.UserID, ""))
	}
	return false
}

// List returns the caller's parties (R37, R40); admins with all see every
// party not over (R43).
func (s *Service) List(ctx context.Context, p auth.Principal, all bool) ([]Party, error) {
	if p.IsGuest() {
		return nil, errNotAllowed
	}
	if all && p.IsAdmin() {
		return s.store.Active(ctx)
	}
	return s.store.ForUser(ctx, p.UserID)
}

// Invites returns a party's invitations.
func (s *Service) Invites(ctx context.Context, id string) ([]Invite, error) {
	return s.store.Invites(ctx, id)
}

// Patch is a change of a party by its organizer.
type Patch struct {
	ScheduledAt *time.Time
	Note        *string
	ItemID      *string
	ControlMode *string
}

// Update changes a party not started yet (or its mode, live).
func (s *Service) Update(ctx context.Context, p auth.Principal, id string, in Patch) (Party, error) {
	pt, err := s.Get(ctx, p, id)
	if err != nil {
		return Party{}, err
	}
	if pt.OrganizerUserID != p.UserID && !p.IsAdmin() {
		return Party{}, errNotAllowed
	}
	if pt.Status == StatusEnded || pt.Status == StatusCancelled {
		return Party{}, apierr.New(409, "party_ended", "this party is over")
	}
	if in.ScheduledAt != nil {
		lead := in.ScheduledAt.Sub(s.o.Now())
		if pt.Status == StatusLive || lead < 0 || lead > maxScheduleLead {
			return Party{}, apierr.ValidationFailed(apierr.Detail{Field: "scheduledAt", Code: "out_of_range"})
		}
		pt.ScheduledAt = in.ScheduledAt
	}
	if in.Note != nil {
		n := strings.TrimSpace(*in.Note)
		if utf8.RuneCountInString(n) > maxNote {
			return Party{}, apierr.ValidationFailed(apierr.Detail{Field: "note", Code: "too_long"})
		}
		pt.Note = n
	}
	if in.ItemID != nil && pt.Status != StatusLive {
		if _, err := s.visible(ctx, p, *in.ItemID); err != nil {
			return Party{}, err
		}
		pt.ItemID = *in.ItemID
	}
	if in.ControlMode != nil {
		if *in.ControlMode != ControlHost && *in.ControlMode != ControlEveryone {
			return Party{}, apierr.ValidationFailed(apierr.Detail{Field: "controlMode", Code: "invalid"})
		}
		pt.ControlMode = *in.ControlMode
		if l := s.roomOf(pt.ID); l != nil {
			l.mu.Lock()
			l.room.SetMode(s.o.Now(), l.room.Host(), *in.ControlMode)
			l.mu.Unlock()
		}
	}
	if err := s.store.Update(ctx, pt); err != nil {
		return Party{}, err
	}
	return pt, nil
}

// Cancel cancels a party not started; on a live one, the organizer or an
// admin ends it for everyone.
func (s *Service) Cancel(ctx context.Context, p auth.Principal, id string) error {
	pt, err := s.Get(ctx, p, id)
	if err != nil {
		return err
	}
	if pt.OrganizerUserID != p.UserID && !p.IsAdmin() {
		return errNotAllowed
	}
	if l := s.roomOf(pt.ID); l != nil {
		l.mu.Lock()
		l.room.end()
		l.mu.Unlock()
		s.finish(ctx, l)
		return nil
	}
	if pt.Status == StatusEnded || pt.Status == StatusCancelled {
		return nil
	}
	pt.Status = StatusCancelled
	now := s.o.Now()
	pt.EndedAt = &now
	if err := s.store.Update(ctx, pt); err != nil {
		return err
	}
	users, _ := s.inviteeIDs(ctx, pt.ID)
	s.notifyInvites(pt, users, "party.cancelled")
	return nil
}

func (s *Service) inviteeIDs(ctx context.Context, partyID string) ([]string, error) {
	inv, err := s.store.Invites(ctx, partyID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(inv))
	for _, i := range inv {
		out = append(out, i.UserID)
	}
	return out, nil
}

// InviteMore adds invitees, also during a live party (R3).
func (s *Service) InviteMore(ctx context.Context, p auth.Principal, id string, users []string) error {
	pt, err := s.Get(ctx, p, id)
	if err != nil {
		return err
	}
	if p.IsGuest() {
		return errNotAllowed
	}
	host := ""
	if l := s.roomOf(pt.ID); l != nil {
		l.mu.Lock()
		host = l.room.Host()
		l.mu.Unlock()
	}
	if pt.OrganizerUserID != p.UserID && host != PersonID(p.UserID, "") && !p.IsAdmin() {
		return errNotAllowed
	}
	ids, err := s.invitable(ctx, p, users)
	if err != nil {
		return err
	}
	if err := s.store.Invite(ctx, pt.ID, ids, s.o.Now()); err != nil {
		return err
	}
	s.notifyInvites(pt, ids, "party.invite")
	return nil
}

// Answer records the caller's answer (R28).
func (s *Service) Answer(ctx context.Context, p auth.Principal, id, answer string) error {
	if answer != AnswerYes && answer != AnswerMaybe && answer != AnswerNo {
		return apierr.ValidationFailed(apierr.Detail{Field: "answer", Code: "invalid"})
	}
	pt, err := s.Get(ctx, p, id)
	if err != nil {
		return err
	}
	if p.IsGuest() {
		return errNotAllowed
	}
	if err := s.store.Answer(ctx, pt.ID, p.UserID, answer, s.o.Now()); errors.Is(err, ErrNotFound) {
		return apierr.New(409, "not_invited", "you weren't invited to this party")
	} else if err != nil {
		return err
	}
	s.mu.Lock()
	hub := s.hub
	s.mu.Unlock()
	if hub != nil && pt.OrganizerUserID != p.UserID {
		hub.SendToUser(pt.OrganizerUserID, "notify", map[string]any{"kind": "party.answer", "partyId": pt.ID})
	}
	return nil
}

// Preview is what the join page shows before joining (R3, Party14).
type Preview struct {
	Party     Party
	HostName  string
	Item      Item
	Watching  int
	Full      bool
	Guests    bool
	StartedAt *time.Time
}

// LookupCode answers the public join page by code.
func (s *Service) LookupCode(ctx context.Context, code string) (Preview, error) {
	set, err := s.enabled(ctx)
	if err != nil {
		return Preview{}, err
	}
	pt, err := s.byCode(ctx, codeKey(ctx, auth.Principal{}), code)
	if err != nil {
		return Preview{}, err
	}
	out := Preview{Party: pt, Guests: set.Guests, StartedAt: pt.StartedAt}
	if pt.ItemID != "" {
		out.Item, _ = s.store.Item(ctx, pt.ItemID)
	}
	if l := s.roomOf(pt.ID); l != nil {
		l.mu.Lock()
		out.Watching = l.room.Count()
		out.Full = l.room.Count() >= l.room.MaxPeople
		if h := l.room.people[l.room.Host()]; h != nil {
			out.HostName = h.Name
		}
		l.mu.Unlock()
	}
	if out.HostName == "" {
		if users, err := s.o.Accounts.ListUsers(ctx); err == nil {
			for _, u := range users {
				if u.ID == pt.OrganizerUserID {
					out.HostName = u.DisplayName
				}
			}
		}
	}
	return out, nil
}

// maxFirstName is a guest's first name (R5).
const maxFirstName = 32

// JoinAsGuest creates a guest of the party holding code (R5); the token
// exists only in the result.
func (s *Service) JoinAsGuest(ctx context.Context, code, firstName string) (Guest, string, error) {
	set, err := s.enabled(ctx)
	if err != nil {
		return Guest{}, "", err
	}
	if !set.Guests {
		return Guest{}, "", apierr.New(403, "guests_not_allowed", "this server doesn't let people without an account join")
	}
	firstName = strings.TrimSpace(firstName)
	if firstName == "" || utf8.RuneCountInString(firstName) > maxFirstName {
		return Guest{}, "", apierr.ValidationFailed(apierr.Detail{Field: "firstName", Code: "invalid"})
	}
	pt, err := s.byCode(ctx, codeKey(ctx, auth.Principal{}), code)
	if err != nil {
		return Guest{}, "", err
	}
	g, token, err := s.store.AddGuest(ctx, Guest{PartyID: pt.ID, FirstName: firstName,
		AvatarColor: int(firstName[0]) % 8, CreatedAt: s.o.Now()})
	if err != nil {
		return Guest{}, "", err
	}
	s.o.Log.Info("party guest joined", "party_id", pt.ID, "guest_id", g.ID)
	return g, token, nil
}

// Summary returns an ended party's summary (R21).
func (s *Service) Summary(pt Party) (Summary, bool) {
	var sum Summary
	if pt.SummaryJSON == "" || json.Unmarshal([]byte(pt.SummaryJSON), &sum) != nil {
		return sum, false
	}
	return sum, true
}

// RoomInfo is the live state a party read shows.
type RoomInfo struct {
	Count    int
	Max      int
	Playback Playback
	Host     string
}

// Room returns the live room of a party, if any.
func (s *Service) Room(id string) (RoomInfo, bool) {
	l := s.roomOf(id)
	if l == nil {
		return RoomInfo{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return RoomInfo{Count: l.room.Count(), Max: l.room.MaxPeople, Playback: l.room.Playback(), Host: l.room.Host()}, true
}

// RaiseLimit lets an admin host raise a full party's limit from the prompt
// (R4); the server setting changes too.
func (s *Service) RaiseLimit(partyID string, n int) {
	if l := s.roomOf(partyID); l != nil {
		l.mu.Lock()
		l.room.SetMaxPeople(s.o.Now(), n)
		l.mu.Unlock()
	}
}

// --- Playback side (playback.PartyAccess) -----------------------------------

// InParty reports whether p is in the live party on itemID (R6, R12).
func (s *Service) InParty(_ context.Context, p auth.Principal, partyID, itemID string) (string, bool, error) {
	l := s.roomOf(partyID)
	if l == nil {
		return "", false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	who := PersonID(p.UserID, p.GuestID)
	if !l.room.Has(who) || (l.room.ItemID != itemID && l.nextItem != itemID) {
		return "", false, nil
	}
	return l.room.people[who].Name, true, nil
}

// GuestActive reports whether a guest's token still works.
func (s *Service) GuestActive(ctx context.Context, guestID string) (bool, error) {
	return s.store.GuestActive(ctx, guestID)
}

// Position is a playing party's reference position now.
func (s *Service) Position(partyID string) (int64, bool) {
	l := s.roomOf(partyID)
	if l == nil {
		return 0, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	pb := l.room.Playback()
	if pb.State != Playing {
		return pb.PositionMs, true
	}
	return pb.At(s.o.Now().UnixMilli()), true
}

// --- Realtime side (realtime.Handler) ---------------------------------------

// Connected registers a connection for routing.
func (s *Service) Connected(c *realtime.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c.ID] = c
}

// Disconnected keeps the person's seat for a while (R17).
func (s *Service) Disconnected(c *realtime.Conn) {
	s.mu.Lock()
	delete(s.conns, c.ID)
	s.mu.Unlock()
	if id, _ := c.Get("party").(string); id != "" {
		if l := s.roomOf(id); l != nil {
			l.mu.Lock()
			l.room.Leave(s.o.Now(), c.ID, false)
			l.mu.Unlock()
		}
	}
}

// Message handles a client message.
func (s *Service) Message(c *realtime.Conn, env realtime.Envelope) error {
	ctx := context.Background()
	switch env.Type {
	case "party.join":
		return s.join(ctx, c, env.Data)
	case "party.screens":
		c.Send("party.screens", map[string]any{"screens": s.Screens(c.Principal)})
		return nil
	}
	if !strings.HasPrefix(env.Type, "party.") {
		return &realtime.Error{Code: "unknown_type"}
	}
	id, _ := c.Get("party").(string)
	l := s.roomOf(id)
	if l == nil {
		return ErrNotInParty
	}
	var in struct {
		PositionMs *int64 `json:"positionMs"`
		Action     string `json:"action"`
		Text       string `json:"text"`
		Emoji      string `json:"emoji"`
		To         string `json:"to"`
		Who        string `json:"who"`
		On         bool   `json:"on"`
		Mode       string `json:"mode"`
		RequestID  int64  `json:"requestId"`
		Answer     string `json:"answer"`
		Choice     string `json:"choice"`
		First      string `json:"first"`
		Quality    string `json:"quality"`
		Max        int    `json:"max"`
	}
	if len(env.Data) > 0 && json.Unmarshal(env.Data, &in) != nil {
		return realtime.ErrBadMessage
	}
	now := s.o.Now()
	l.mu.Lock()
	me := l.room.PersonOf(c.ID)
	if me == "" && env.Type != "party.queue.leave" {
		l.mu.Unlock()
		return ErrNotInParty
	}
	var err error
	var removed []string
	ended := false
	switch env.Type {
	case "party.leave", "party.queue.leave":
		l.room.Leave(now, c.ID, true)
		c.Set("party", "")
	case "party.report":
		var rep Report
		if json.Unmarshal(env.Data, &rep) != nil {
			err = realtime.ErrBadMessage
			break
		}
		switch rep.State {
		case "blocked": // transcode_limit_reached (R26)
			err = s.blocked(l, me, rep.Quality)
		case "credits": // the end credits start (R30, ADR 0008)
			rep.State = ScreenPlaying
			err = l.room.Report(now, c.ID, rep)
			s.itemEnded(ctx, l, now)
		default:
			delete(l.blocked, me)
			err = l.room.Report(now, c.ID, rep)
			if rep.State == ScreenEnded {
				s.itemEnded(ctx, l, now)
			}
		}
	case "party.control":
		err = l.room.Control(now, me, in.Action, in.PositionMs)
	case "party.pause.request":
		err = l.room.PauseRequest(now, me)
	case "party.pause.answer":
		err = l.room.PauseAnswer(now, me, in.RequestID, in.Answer)
	case "party.react":
		err = l.room.React(now, me, in.Emoji)
	case "party.chat":
		err = l.room.Chat(now, me, in.Text)
	case "party.host.set":
		err = l.room.HandHost(now, me, in.To)
	case "party.control.grant":
		err = l.room.Grant(now, me, in.To, in.On)
	case "party.mode.set":
		if err = l.room.SetMode(now, me, in.Mode); err == nil {
			l.party.ControlMode = in.Mode
		}
	case "party.remove":
		removed, err = l.room.Remove(now, me, in.Who)
	case "party.end":
		if err = l.room.End(now, me); err == nil {
			ended = true
		}
	case "party.start":
		if me != l.room.Host() {
			err = ErrNotHost
		} else {
			s.startNow(ctx, l, now)
		}
	case "party.resume.without":
		err = l.room.ResumeWithout(now, me)
	case "party.transcode.choose":
		err = s.chooseTranscode(l, me, in.Choice, in.First, in.Quality)
	case "party.next.stop":
		if me != l.room.Host() {
			err = ErrNotHost
		} else {
			l.nextAt, l.nextItem = time.Time{}, ""
			l.room.emit(Target{All: true}, "party.next", map[string]any{"stopped": true})
		}
	default:
		err = &realtime.Error{Code: "unknown_type"}
	}
	party := l.party
	l.mu.Unlock()
	if len(removed) > 0 {
		who := in.Who
		if g, ok := strings.CutPrefix(who, "g:"); ok {
			_ = s.store.Remove(ctx, party.ID, "", g, now)
		} else {
			_ = s.store.Remove(ctx, party.ID, strings.TrimPrefix(who, "u:"), "", now)
		}
		s.mu.Lock()
		for _, cid := range removed {
			if rc := s.conns[cid]; rc != nil {
				rc.Set("party", "")
			}
		}
		s.mu.Unlock()
		s.o.Log.Info("party: person removed", "party_id", party.ID)
	}
	if env.Type == "party.host.set" && err == nil {
		s.o.Log.Info("party: host changed", "party_id", party.ID)
	}
	if ended {
		s.finish(ctx, l)
	}
	return err
}

// join puts a connection in a party (R3, R5, R22).
func (s *Service) join(ctx context.Context, c *realtime.Conn, raw json.RawMessage) error {
	var in struct {
		PartyID string `json:"partyId"`
		Code    string `json:"code"`
		Screen  string `json:"screen"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return realtime.ErrBadMessage
	}
	p := c.Principal
	if _, err := s.enabled(ctx); err != nil {
		return ErrNotAllowed
	}
	var pt Party
	var err error
	byCode := false
	switch {
	case p.IsGuest():
		pt, err = s.store.Get(ctx, p.PartyID)
	case in.Code != "":
		if pt, err = s.byCode(ctx, codeKey(ctx, p), in.Code); err != nil {
			var tooMany *apierr.Error
			if errors.As(err, &tooMany) && tooMany.Status == 429 {
				return realtime.ErrRateLimited
			}
			return &realtime.Error{Code: "party_not_found"}
		}
		byCode = true
	default:
		pt, err = s.store.Get(ctx, in.PartyID)
	}
	if err != nil {
		return ErrNotAllowed
	}
	who := Person{ID: PersonID(p.UserID, p.GuestID), UserID: p.UserID, ProfileID: p.ProfileID, GuestID: p.GuestID}
	if p.IsGuest() {
		g, err := s.store.Guest(ctx, p.GuestID)
		if err != nil {
			return ErrNotAllowed
		}
		who.Name, who.AvatarColor = g.FirstName, g.AvatarColor
	} else {
		right, err := s.right(ctx, p)
		if err != nil || right == "none" || right == "" {
			return ErrNotAllowed
		}
		invited := pt.OrganizerUserID == p.UserID || p.IsAdmin()
		if !invited {
			invited, _ = s.store.IsInvited(ctx, pt.ID, p.UserID)
		}
		inRoom := false
		if l := s.roomOf(pt.ID); l != nil {
			l.mu.Lock()
			inRoom = l.room.Has(who.ID)
			l.mu.Unlock()
		}
		// Invitees join by id; anyone else needs the code (R3).
		if !invited && !byCode && !inRoom {
			return ErrNotAllowed
		}
		if removed, _ := s.store.IsRemoved(ctx, pt.ID, p.UserID); removed {
			return ErrNotAllowed
		}
		me, err := s.o.Accounts.Me(ctx, p)
		if err != nil || me.Active == nil {
			return &realtime.Error{Code: "profile_required"}
		}
		if pt.ItemID != "" {
			it, err := s.store.Item(ctx, pt.ItemID)
			if err != nil {
				return ErrNotAllowed
			}
			lim, err := s.o.Accounts.AccessLimit(ctx, p)
			if err != nil || !lim.AllowsAge(it.AgeRating) {
				return &realtime.Error{Code: "party_age_limit"}
			}
		}
		who.Name, who.AvatarColor = me.Active.Name, me.Active.AvatarColor
		if me.Active.Avatar.Kind != "" && me.Active.Avatar.Kind != accounts.AvatarInitial {
			who.AvatarURL = "/api/v1/avatars/" + me.Active.ID
		}
	}
	l := s.roomOf(pt.ID)
	if l == nil {
		if pt.Status != StatusLive && pt.Status != StatusLobby {
			return errNotStarted
		}
		if l, err = s.openRoom(ctx, pt); err != nil {
			return err
		}
	}
	kind := KindScreen
	switch in.Screen {
	case "", "this":
	case "companion":
		kind = KindCompanion
	default:
		// Watch on one of the account's Apple TVs (R22): it opens the
		// player and joins as the screen; this device is the companion.
		tv := s.tvConn(p, in.Screen)
		if tv == nil {
			return &realtime.Error{Code: "screen_offline"}
		}
		tv.Send("tv.cast", map[string]string{"partyId": pt.ID})
		kind = KindCompanion
	}
	l.mu.Lock()
	_, err = l.room.Join(s.o.Now(), who, c.ID, kind, c.Device)
	l.mu.Unlock()
	if err != nil {
		return err
	}
	c.Set("party", pt.ID)
	if kind == KindScreen && c.Device.Type == "tvos" {
		s.offerCompanion(c, pt.ID)
	}
	s.mu.Lock()
	if s.names[pt.ID] != nil {
		s.names[pt.ID][who.ID] = who.Name
	}
	s.mu.Unlock()
	s.o.Log.Info("party: joined", "party_id", pt.ID, "guest", p.IsGuest(), "by_code", byCode)
	return nil
}

// Screen is an Apple TV of the account with its app open (R22).
type Screen struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

// Screens lists the caller's Apple TVs that are online.
func (s *Service) Screens(p auth.Principal) []Screen {
	if p.UserID == "" {
		return []Screen{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Screen{}
	seen := map[string]bool{}
	for _, c := range s.conns {
		if c.Principal.UserID == p.UserID && c.Device.Type == "tvos" && !seen[c.Principal.SessionID] {
			seen[c.Principal.SessionID] = true
			out = append(out, Screen{SessionID: c.Principal.SessionID, Name: c.Device.Name, Type: c.Device.Type})
		}
	}
	return out
}

func (s *Service) tvConn(p auth.Principal, sessionID string) *realtime.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		if c.Principal.UserID == p.UserID && p.UserID != "" && c.Principal.SessionID == sessionID && c.Device.Type == "tvos" {
			return c
		}
	}
	return nil
}

// offerCompanion asks the same account's iPhone or iPad, app open and not
// in the party, to join as the TV's companion (R23).
func (s *Service) offerCompanion(tv *realtime.Conn, partyID string) {
	s.mu.Lock()
	var phones []*realtime.Conn
	for _, c := range s.conns {
		if c.Principal.UserID == tv.Principal.UserID && c.ID != tv.ID &&
			(c.Device.Type == "ios" || c.Device.Type == "ipados") {
			if id, _ := c.Get("party").(string); id != partyID {
				phones = append(phones, c)
			}
		}
	}
	s.mu.Unlock()
	for _, c := range phones {
		c.Send("party.companion", map[string]string{"partyId": partyID, "screen": tv.Device.Name})
	}
}

// Opens reports whether p is in a live party on itemID (or about to move to
// it): the party opens that title's page and artwork, whatever their
// libraries (R6). A guest's own party only.
func (s *Service) Opens(p auth.Principal, itemID string) bool {
	who := PersonID(p.UserID, p.GuestID)
	for _, l := range s.liveRooms() {
		if p.IsGuest() && l.party.ID != p.PartyID {
			continue
		}
		l.mu.Lock()
		ok := l.room.Has(who) && (l.room.ItemID == itemID || l.nextItem == itemID)
		l.mu.Unlock()
		if ok {
			return true
		}
	}
	return false
}
