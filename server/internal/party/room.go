package party

import (
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gh-prim/plaid/server/internal/realtime"
)

// Room timings and limits (spec F010-R4, R7, R11, R15, R17–R19).
const (
	bufferGrace      = 1500 * time.Millisecond // a screen buffering longer puts the room in waiting (R11)
	resumeWithoutMin = 20 * time.Second        // then someone may resume without them (R11)
	pauseDelay       = 5 * time.Minute         // "In 5 min" (R15)
	pauseRequestGap  = time.Minute             // one pause request a minute per person (R15)
	hostGrace        = 2 * time.Minute         // a vanished host keeps the role that long (R17)
	seatGrace        = 2 * time.Minute         // a vanished person keeps their seat that long
	emptyEnd         = 10 * time.Minute        // nobody connected that long ends the party (R7)
	chatHistory      = 200                     // messages a newcomer gets (R18)
	maxChat          = 500                     // characters per message (R18)
)

// Reactions are the six of the design (R19).
var Reactions = []string{"😂", "😱", "❤️", "👏", "🦇", "🍿"}

// Playback states of the room (R8).
const (
	Playing = "playing"
	Paused  = "paused"
	Waiting = "waiting"
)

// Device kinds: a person's screen plays the item; companions show the chat
// and controls (R22).
const (
	KindScreen    = "screen"
	KindCompanion = "companion"
)

// Screen states reported by clients (party.report).
const (
	ScreenLoading   = "loading"
	ScreenReady     = "ready"
	ScreenPlaying   = "playing"
	ScreenPaused    = "paused"
	ScreenBuffering = "buffering"
	ScreenEnded     = "ended"
)

// Errors a client gets back from the room.
var (
	ErrNoControl    = &realtime.Error{Code: "no_control"}
	ErrNotInParty   = &realtime.Error{Code: "not_in_party"}
	ErrPartyEnded   = &realtime.Error{Code: "party_ended"}
	ErrNotHost      = &realtime.Error{Code: "not_host"}
	ErrTooSoon      = &realtime.Error{Code: "too_soon"}
	ErrBadReaction  = &realtime.Error{Code: "bad_reaction"}
	ErrBadText      = &realtime.Error{Code: "bad_text"}
	ErrRateLimited  = realtime.ErrRateLimited
	ErrNotAllowed   = realtime.ErrNotAllowed
	ErrNoSuchPerson = &realtime.Error{Code: "no_such_person"}
)

// Person is who joins: an account's profile, or a guest.
type Person struct {
	// ID is "u:<userID>" or "g:<guestID>".
	ID          string `json:"id"`
	UserID      string `json:"-"`
	ProfileID   string `json:"-"`
	GuestID     string `json:"-"`
	Name        string `json:"name"`
	AvatarColor int    `json:"avatarColor"`
	// AvatarURL is the F009 picture, empty for guests and initials.
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// Guest reports whether p has no account.
func (p Person) Guest() bool { return p.GuestID != "" }

// PersonID builds a Person.ID.
func PersonID(userID, guestID string) string {
	if guestID != "" {
		return "g:" + guestID
	}
	return "u:" + userID
}

// device is one connection of a participant.
type device struct {
	ConnID    string
	Kind      string
	Device    realtime.Device
	State     string
	Quality   string
	Session   string // F006 playback session of a screen
	bufSince  time.Time
	announced bool
}

type participant struct {
	Person
	joinedAt   time.Time
	devices    map[string]*device
	goneSince  time.Time // zero while connected
	canControl bool
	chat       realtime.Limiter
	react      realtime.Limiter
	lastPause  time.Time
}

func (p *participant) connected() bool { return len(p.devices) > 0 }

func (p *participant) screen() *device {
	for _, d := range p.devices {
		if d.Kind == KindScreen {
			return d
		}
	}
	return nil
}

// Playback is the room's reference state (R8): when playing, the position
// at server time t is PositionMs + (t − AtMs) × Rate.
type Playback struct {
	State      string  `json:"state"`
	PositionMs int64   `json:"positionMs"`
	AtMs       int64   `json:"atServerMs"`
	Rate       float64 `json:"rate"`
}

// At returns the position at server time nowMs.
func (pb Playback) At(nowMs int64) int64 {
	if pb.State != Playing {
		return pb.PositionMs
	}
	return pb.PositionMs + int64(float64(nowMs-pb.AtMs)*pb.Rate)
}

// ChatMessage is a line of the chat (R18).
type ChatMessage struct {
	ID         int64  `json:"id"`
	From       string `json:"from,omitempty"`
	Name       string `json:"name,omitempty"`
	Text       string `json:"text,omitempty"`
	PositionMs int64  `json:"positionMs"`
	AtMs       int64  `json:"atServerMs"`
	// System lines name an event (joined, left, quality) with its values.
	System string            `json:"system,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

// Target says who an event goes to.
type Target struct {
	All    bool
	Person string
	Conn   string
	// Hosts sends to the host's connections (and, in everyone mode, to
	// everyone with control).
	Host bool
}

// Emit sends an event; the service routes it to connections.
type Emit func(to Target, typ string, data any)

// Room is a live party in memory. It isn't safe for concurrent use: the
// service holds its lock around every call.
type Room struct {
	PartyID   string
	ItemID    string
	Mode      string
	MaxPeople int
	// Log traces screen states and waits, to debug sync; nil logs nothing.
	Log *slog.Logger

	host     string
	people   map[string]*participant
	order    []string
	queue    []queued
	removed  map[string]bool
	pb       Playback
	waiting  map[string]bool
	waitFrom time.Time

	pauseReqs   int64
	pauseAt     int64 // scheduled pause, server ms; 0 when none
	pendingAsks map[int64]string
	chat        []ChatMessage
	chatSeq     int64

	startedAt  time.Time
	emptySince time.Time
	ended      bool
	reactions  int
	reactByMin map[int64]map[string]int
	pauseAsked int
	everJoined []string
	emit       Emit
}

type queued struct {
	person Person
	conn   string
	kind   string
	dev    realtime.Device
}

// NewRoom opens a room on itemID, paused at startMs.
func NewRoom(partyID, itemID, mode string, maxPeople int, now time.Time, emit Emit) *Room {
	if emit == nil {
		emit = func(Target, string, any) {}
	}
	return &Room{
		PartyID: partyID, ItemID: itemID, Mode: mode, MaxPeople: maxPeople,
		people: map[string]*participant{}, removed: map[string]bool{}, waiting: map[string]bool{},
		pendingAsks: map[int64]string{}, reactByMin: map[int64]map[string]int{},
		pb:        Playback{State: Paused, AtMs: now.UnixMilli(), Rate: 1},
		startedAt: now, emptySince: now, emit: emit,
	}
}

// Host returns the host's person id, "" when nobody holds the role.
func (r *Room) Host() string { return r.host }

// Playback returns the reference state.
func (r *Room) Playback() Playback { return r.pb }

// Ended reports whether the room is over.
func (r *Room) Ended() bool { return r.ended }

// Has reports whether person is in the room (connected or keeping their seat).
func (r *Room) Has(person string) bool { return r.people[person] != nil }

// Removed reports whether person was removed (R16).
func (r *Room) Removed(person string) bool { return r.removed[person] }

// Count is the number of people holding a seat.
func (r *Room) Count() int { return len(r.people) }

// SetHost sets the initial host (the party's starter).
func (r *Room) SetHost(person string) { r.host = person }

// CanControl reports whether person may play, pause and seek (R14).
func (r *Room) CanControl(person string) bool {
	p := r.people[person]
	if p == nil {
		return false
	}
	return r.Mode == ControlEveryone || person == r.host || p.canControl
}

// JoinResult says what happened to a join.
type JoinResult struct {
	Queued   bool
	Position int
}

// Join adds a device of person. A full room queues new people (R4).
func (r *Room) Join(now time.Time, who Person, conn, kind string, dev realtime.Device) (JoinResult, error) {
	if r.ended {
		return JoinResult{}, ErrPartyEnded
	}
	if r.removed[who.ID] {
		return JoinResult{}, ErrNotAllowed
	}
	if kind != KindCompanion {
		kind = KindScreen
	}
	p := r.people[who.ID]
	if p == nil {
		if len(r.people) >= r.MaxPeople {
			for i, q := range r.queue {
				if q.person.ID == who.ID {
					r.queue[i].conn, r.queue[i].kind, r.queue[i].dev = conn, kind, dev
					return JoinResult{Queued: true, Position: i + 1}, nil
				}
			}
			r.queue = append(r.queue, queued{person: who, conn: conn, kind: kind, dev: dev})
			pos := len(r.queue)
			r.emit(Target{Conn: conn}, "party.queued", map[string]int{"position": pos})
			r.emit(Target{Host: true}, "party.queue", r.queueInfo())
			return JoinResult{Queued: true, Position: pos}, nil
		}
		p = &participant{Person: who, joinedAt: now, devices: map[string]*device{},
			chat: realtime.Limiter{Rate: 1, Burst: 5}, react: realtime.Limiter{Rate: 3, Burst: 3}}
		r.people[who.ID] = p
		r.order = append(r.order, who.ID)
		if !slices.Contains(r.everJoined, who.ID) {
			r.everJoined = append(r.everJoined, who.ID)
		}
		if r.host == "" && !who.Guest() {
			r.host = who.ID
		}
	}
	// One screen per person: a new screen turns the old one into a companion.
	if kind == KindScreen {
		for _, d := range p.devices {
			if d.Kind == KindScreen {
				d.Kind = KindCompanion
			}
		}
	}
	p.devices[conn] = &device{ConnID: conn, Kind: kind, Device: dev, State: ScreenLoading}
	p.goneSince = time.Time{}
	r.emptySince = time.Time{}
	r.emit(Target{Conn: conn}, "party.snapshot", r.Snapshot(now, who.ID))
	r.emit(Target{All: true}, "party.presence", r.presence())
	return JoinResult{}, nil
}

// Leave drops a connection. explicit is a party.leave: the person gives up
// their seat at once (a host hands the role to the next person, R17);
// otherwise they keep it for seatGrace.
func (r *Room) Leave(now time.Time, conn string, explicit bool) {
	for i, q := range r.queue {
		if q.conn == conn {
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
			r.emit(Target{Host: true}, "party.queue", r.queueInfo())
			return
		}
	}
	p := r.byConn(conn)
	if p == nil {
		return
	}
	delete(p.devices, conn)
	delete(r.waiting, p.ID)
	if explicit && !p.connected() {
		r.drop(now, p.ID, "left")
	} else if !p.connected() {
		p.goneSince = now
	}
	r.afterLeave(now)
}

func (r *Room) afterLeave(now time.Time) {
	if !r.anyConnected() && r.emptySince.IsZero() {
		r.emptySince = now
	}
	r.resolveWaiting(now)
	r.emit(Target{All: true}, "party.presence", r.presence())
}

// drop takes person out of the room and admits the first queued person.
func (r *Room) drop(now time.Time, person, why string) {
	p := r.people[person]
	if p == nil {
		return
	}
	delete(r.people, person)
	r.order = slices.DeleteFunc(r.order, func(id string) bool { return id == person })
	delete(r.waiting, person)
	r.system(now, why, map[string]string{"name": p.Name})
	if r.host == person {
		r.passHost(now, "")
	}
	r.admit(now)
}

func (r *Room) admit(now time.Time) {
	for len(r.queue) > 0 && len(r.people) < r.MaxPeople {
		q := r.queue[0]
		r.queue = r.queue[1:]
		_, _ = r.Join(now, q.person, q.conn, q.kind, q.dev) // there is a seat: it can only succeed
	}
	for i, q := range r.queue {
		r.emit(Target{Conn: q.conn}, "party.queued", map[string]int{"position": i + 1})
	}
	r.emit(Target{Host: true}, "party.queue", r.queueInfo())
}

// SetMaxPeople changes the limit (an admin host raising it, R4).
func (r *Room) SetMaxPeople(now time.Time, n int) {
	r.MaxPeople = n
	r.admit(now)
}

func (r *Room) queueInfo() map[string]any {
	names := make([]string, 0, len(r.queue))
	for _, q := range r.queue {
		names = append(names, q.person.Name)
	}
	return map[string]any{"waiting": names}
}

// passHost gives the host role to `to`, or to the person with an account who
// joined first and is connected (R17).
func (r *Room) passHost(now time.Time, to string) {
	if to == "" {
		for _, id := range r.order {
			p := r.people[id]
			if p != nil && !p.Guest() && p.connected() && id != r.host {
				to = id
				break
			}
		}
	}
	r.host = to
	name := ""
	if p := r.people[to]; p != nil {
		name = p.Name
	}
	r.emit(Target{All: true}, "party.host.changed", map[string]string{"host": to, "name": name})
	if to != "" {
		r.system(now, "host", map[string]string{"name": name})
	}
}

func (r *Room) byConn(conn string) *participant {
	for _, p := range r.people {
		if _, ok := p.devices[conn]; ok {
			return p
		}
	}
	return nil
}

// PersonOf returns the person behind a connection, "" if none.
func (r *Room) PersonOf(conn string) string {
	if p := r.byConn(conn); p != nil {
		return p.ID
	}
	return ""
}

// ScreenOf returns the F006 session a person's screen plays, "" if none.
func (r *Room) ScreenOf(person string) string {
	if p := r.people[person]; p != nil {
		if s := p.screen(); s != nil {
			return s.Session
		}
	}
	return ""
}

func (r *Room) anyConnected() bool {
	for _, p := range r.people {
		if p.connected() {
			return true
		}
	}
	return false
}

// Report is a screen's playback report (party.report).
type Report struct {
	State      string `json:"state"`
	PositionMs int64  `json:"positionMs"`
	Session    string `json:"playbackSessionId,omitempty"`
	Quality    string `json:"quality,omitempty"`
}

// Report records a screen's state; buffering past the grace puts the room
// in waiting (Tick), ready resumes it (R11).
func (r *Room) Report(now time.Time, conn string, rep Report) error {
	p := r.byConn(conn)
	if p == nil {
		return ErrNotInParty
	}
	d := p.devices[conn]
	if d.Kind != KindScreen {
		return nil
	}
	if rep.Session != "" {
		d.Session = rep.Session
	}
	if rep.Quality != "" {
		d.Quality = rep.Quality
	}
	switch rep.State {
	case ScreenBuffering:
		if d.State != ScreenBuffering {
			d.bufSince = now
		}
	case ScreenLoading, ScreenReady, ScreenPlaying, ScreenPaused, ScreenEnded:
		d.bufSince = time.Time{}
	default:
		return realtime.ErrBadMessage
	}
	prev := d.State
	d.State = rep.State
	if prev != rep.State && r.Log != nil {
		r.Log.Info("party: screen", "party_id", r.PartyID, "name", p.Name, "device", d.Device.Type,
			"from", prev, "state", rep.State, "position_ms", rep.PositionMs, "room", r.pb.State)
	}
	if rep.State != ScreenBuffering && rep.State != ScreenLoading && !d.announced {
		d.announced = true
		r.system(now, "joined", map[string]string{"name": p.Name, "device": d.Device.Type, "quality": d.Quality})
	}
	if rep.State != ScreenBuffering && r.waiting[p.ID] {
		delete(r.waiting, p.ID)
		r.resolveWaiting(now)
	}
	if prev != rep.State {
		r.emit(Target{All: true}, "party.presence", r.presence())
	}
	return nil
}

// Control applies play, pause or seek from person (R13, R14).
func (r *Room) Control(now time.Time, person, action string, positionMs *int64) error {
	if r.ended {
		return ErrPartyEnded
	}
	if !r.CanControl(person) {
		return ErrNoControl
	}
	nowMs := now.UnixMilli()
	pos := r.pb.At(nowMs)
	switch action {
	case "play":
		if positionMs != nil {
			pos = *positionMs
		}
		r.waiting = map[string]bool{}
		r.pb = Playback{State: Playing, PositionMs: pos, AtMs: nowMs, Rate: 1}
	case "pause":
		if positionMs != nil {
			pos = *positionMs
		}
		r.pb = Playback{State: Paused, PositionMs: pos, AtMs: nowMs, Rate: 1}
		r.pauseAt = 0
	case "seek":
		if positionMs == nil || *positionMs < 0 {
			return realtime.ErrBadMessage
		}
		state := r.pb.State
		if state == Waiting {
			state = Playing
		}
		r.waiting = map[string]bool{}
		r.pb = Playback{State: state, PositionMs: *positionMs, AtMs: nowMs, Rate: 1}
	default:
		return realtime.ErrBadMessage
	}
	r.broadcast(action, person)
	return nil
}

func (r *Room) broadcast(action, by string) {
	name := ""
	if p := r.people[by]; p != nil {
		name = p.Name
	}
	r.emit(Target{All: true}, "party.playback", map[string]any{
		"state": r.pb.State, "positionMs": r.pb.PositionMs, "atServerMs": r.pb.AtMs, "rate": r.pb.Rate,
		"action": action, "by": by, "byName": name,
	})
}

// Start plays from startMs for everyone (lobby countdown, next episode).
func (r *Room) Start(now time.Time, startMs int64) {
	r.waiting = map[string]bool{}
	r.pb = Playback{State: Playing, PositionMs: startMs, AtMs: now.UnixMilli(), Rate: 1}
	r.broadcast("play", "")
}

// ChangeItem moves the room to another item, paused at 0 until Start (R30).
func (r *Room) ChangeItem(now time.Time, itemID string) {
	r.ItemID = itemID
	r.waiting = map[string]bool{}
	r.pauseAt = 0
	for _, p := range r.people {
		if s := p.screen(); s != nil {
			s.State, s.Session, s.bufSince = ScreenLoading, "", time.Time{}
		}
	}
	r.pb = Playback{State: Paused, AtMs: now.UnixMilli(), Rate: 1}
	r.emit(Target{All: true}, "party.item", map[string]string{"itemId": itemID})
	r.broadcast("item", "")
}

// Tick runs the room's timers; the service calls it a few times a second.
// It reports whether the room just ended.
func (r *Room) Tick(now time.Time) bool {
	if r.ended {
		return false
	}
	nowMs := now.UnixMilli()
	// Scheduled pause ("In 5 min", R15).
	if r.pauseAt != 0 && nowMs >= r.pauseAt {
		r.pauseAt = 0
		r.pb = Playback{State: Paused, PositionMs: r.pb.At(nowMs), AtMs: nowMs, Rate: 1}
		r.broadcast("pause", "")
	}
	// Screens buffering past the grace put the room in waiting (R11).
	if r.pb.State == Playing {
		var slow []string
		for _, id := range r.order {
			if s := r.people[id].screen(); s != nil && s.State == ScreenBuffering && now.Sub(s.bufSince) >= bufferGrace {
				slow = append(slow, id)
			}
		}
		if len(slow) > 0 {
			if r.Log != nil {
				for _, id := range slow {
					r.Log.Info("party: waiting", "party_id", r.PartyID, "name", r.people[id].Name,
						"buffering_ms", now.Sub(r.people[id].screen().bufSince).Milliseconds(), "position_ms", r.pb.At(nowMs))
				}
			}
			r.pb = Playback{State: Waiting, PositionMs: r.pb.At(nowMs), AtMs: nowMs, Rate: 1}
			r.waitFrom = now
			for _, id := range slow {
				r.waiting[id] = true
			}
			r.emitWaiting(now)
			r.broadcast("wait", "")
		}
	}
	// Seats and host of people gone too long (R17).
	for _, id := range slices.Clone(r.order) {
		p := r.people[id]
		if p == nil || p.connected() || p.goneSince.IsZero() {
			continue
		}
		if id == r.host && now.Sub(p.goneSince) >= hostGrace {
			r.passHost(now, "")
		}
		if now.Sub(p.goneSince) >= seatGrace {
			r.drop(now, id, "left")
			r.emit(Target{All: true}, "party.presence", r.presence())
		}
	}
	// Nobody connected for a while ends the party (R7).
	if !r.emptySince.IsZero() && now.Sub(r.emptySince) >= emptyEnd {
		r.end()
		return true
	}
	return false
}

func (r *Room) emitWaiting(_ time.Time) {
	names := []string{}
	ids := []string{}
	for _, id := range r.order {
		if r.waiting[id] {
			ids = append(ids, id)
			names = append(names, r.people[id].Name)
		}
	}
	r.emit(Target{All: true}, "party.waiting", map[string]any{
		"for": ids, "names": names, "positionMs": r.pb.PositionMs,
		"resumeWithoutAtMs": r.waitFrom.Add(resumeWithoutMin).UnixMilli(),
	})
}

// resolveWaiting resumes the room once nobody it waits for is left.
func (r *Room) resolveWaiting(now time.Time) {
	if r.pb.State != Waiting {
		return
	}
	for id := range r.waiting {
		if r.people[id] == nil || !r.people[id].connected() {
			delete(r.waiting, id)
		}
	}
	if len(r.waiting) > 0 {
		r.emitWaiting(now)
		return
	}
	if r.Log != nil {
		r.Log.Info("party: resumed", "party_id", r.PartyID, "waited_ms", now.Sub(r.waitFrom).Milliseconds())
	}
	r.pb = Playback{State: Playing, PositionMs: r.pb.PositionMs, AtMs: now.UnixMilli(), Rate: 1}
	r.broadcast("resume", "")
}

// ResumeWithout plays again without the people the room waits for, once
// they've been waited for resumeWithoutMin (R11).
func (r *Room) ResumeWithout(now time.Time, person string) error {
	if r.pb.State != Waiting {
		return nil
	}
	if !r.CanControl(person) {
		return ErrNoControl
	}
	if now.Sub(r.waitFrom) < resumeWithoutMin {
		return ErrTooSoon
	}
	r.waiting = map[string]bool{}
	r.resolveWaiting(now)
	return nil
}

// PauseRequest asks the host for a pause (R15). With no host connected, the
// room pauses at once.
func (r *Room) PauseRequest(now time.Time, person string) error {
	p := r.people[person]
	if p == nil {
		return ErrNotInParty
	}
	if !p.lastPause.IsZero() && now.Sub(p.lastPause) < pauseRequestGap {
		return ErrTooSoon
	}
	p.lastPause = now
	r.pauseAsked++
	host := r.people[r.host]
	if host == nil || !host.connected() {
		r.pb = Playback{State: Paused, PositionMs: r.pb.At(now.UnixMilli()), AtMs: now.UnixMilli(), Rate: 1}
		r.broadcast("pause", person)
		return nil
	}
	r.pauseReqs++
	r.pendingAsks[r.pauseReqs] = person
	r.emit(Target{All: true}, "party.pause.requested", map[string]any{
		"requestId": r.pauseReqs, "by": person, "name": p.Name,
	})
	return nil
}

// PauseAnswer is the host's answer: "now" or "in5" (R15).
func (r *Room) PauseAnswer(now time.Time, person string, requestID int64, answer string) error {
	if person != r.host {
		return ErrNotHost
	}
	if _, ok := r.pendingAsks[requestID]; !ok {
		return nil
	}
	delete(r.pendingAsks, requestID)
	nowMs := now.UnixMilli()
	switch answer {
	case "now":
		r.pb = Playback{State: Paused, PositionMs: r.pb.At(nowMs), AtMs: nowMs, Rate: 1}
		r.broadcast("pause", person)
	case "in5":
		r.pauseAt = now.Add(pauseDelay).UnixMilli()
		r.emit(Target{All: true}, "party.pause.scheduled", map[string]int64{"atServerMs": r.pauseAt})
	case "no":
	default:
		return realtime.ErrBadMessage
	}
	r.emit(Target{All: true}, "party.pause.answered", map[string]any{"requestId": requestID, "answer": answer})
	return nil
}

// React sends a reaction (R19).
func (r *Room) React(now time.Time, person, emoji string) error {
	p := r.people[person]
	if p == nil {
		return ErrNotInParty
	}
	if !slices.Contains(Reactions, emoji) {
		return ErrBadReaction
	}
	if !p.react.Allow(now) {
		return ErrRateLimited
	}
	pos := r.pb.At(now.UnixMilli())
	r.reactions++
	minute := pos / 60000
	if r.reactByMin[minute] == nil {
		r.reactByMin[minute] = map[string]int{}
	}
	r.reactByMin[minute][emoji]++
	r.emit(Target{All: true}, "party.reaction", map[string]any{
		"from": person, "name": p.Name, "emoji": emoji, "positionMs": pos,
	})
	return nil
}

// Chat posts a message (R18).
func (r *Room) Chat(now time.Time, person, text string) error {
	p := r.people[person]
	if p == nil {
		return ErrNotInParty
	}
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > maxChat {
		return ErrBadText
	}
	if !p.chat.Allow(now) {
		return ErrRateLimited
	}
	r.post(now, ChatMessage{From: person, Name: p.Name, Text: text})
	return nil
}

func (r *Room) system(now time.Time, kind string, values map[string]string) {
	r.post(now, ChatMessage{System: kind, Values: values})
}

func (r *Room) post(now time.Time, m ChatMessage) {
	r.chatSeq++
	m.ID = r.chatSeq
	m.AtMs = now.UnixMilli()
	m.PositionMs = r.pb.At(m.AtMs)
	r.chat = append(r.chat, m)
	if len(r.chat) > chatHistory {
		r.chat = r.chat[len(r.chat)-chatHistory:]
	}
	r.emit(Target{All: true}, "party.chat", m)
}

// SetMode changes the control mode (host only, R14).
func (r *Room) SetMode(_ time.Time, person, mode string) error {
	if person != r.host {
		return ErrNotHost
	}
	if mode != ControlHost && mode != ControlEveryone {
		return realtime.ErrBadMessage
	}
	r.Mode = mode
	r.emit(Target{All: true}, "party.mode", map[string]string{"mode": mode})
	r.emit(Target{All: true}, "party.presence", r.presence())
	return nil
}

// Grant gives or takes control from one person (R16).
func (r *Room) Grant(_ time.Time, person, to string, on bool) error {
	if person != r.host {
		return ErrNotHost
	}
	p := r.people[to]
	if p == nil {
		return ErrNoSuchPerson
	}
	p.canControl = on
	r.emit(Target{All: true}, "party.presence", r.presence())
	return nil
}

// HandHost passes the host role to someone with an account (R16, R17).
func (r *Room) HandHost(now time.Time, person, to string) error {
	if person != r.host {
		return ErrNotHost
	}
	p := r.people[to]
	if p == nil || p.Guest() {
		return ErrNoSuchPerson
	}
	r.passHost(now, to)
	return nil
}

// Remove takes someone out for good (R16); it returns their connections so
// the service can tell them.
func (r *Room) Remove(now time.Time, person, who string) ([]string, error) {
	if person != r.host {
		return nil, ErrNotHost
	}
	p := r.people[who]
	if p == nil || who == person {
		return nil, ErrNoSuchPerson
	}
	conns := make([]string, 0, len(p.devices))
	for c := range p.devices {
		conns = append(conns, c)
	}
	r.removed[who] = true
	for _, c := range conns {
		r.emit(Target{Conn: c}, "party.removed", nil)
	}
	r.drop(now, who, "removed")
	r.afterLeave(now)
	return conns, nil
}

// End closes the room for everyone (host's "End for everyone", R17).
func (r *Room) End(_ time.Time, person string) error {
	if person != r.host {
		return ErrNotHost
	}
	r.end()
	return nil
}

func (r *Room) end() {
	if r.ended {
		return
	}
	r.ended = true
	r.emit(Target{All: true}, "party.ended", nil)
}

// Summary is the end screen (R21).
type Summary struct {
	People        []string `json:"people"`
	TogetherMs    int64    `json:"togetherMs"`
	Reactions     int      `json:"reactions"`
	PauseRequests int      `json:"pauseRequests"`
	PeakMinuteMs  int64    `json:"peakPositionMs,omitempty"`
	PeakEmoji     string   `json:"peakEmoji,omitempty"`
}

// Summarize builds the end summary; names maps person ids to names for
// people who already left.
func (r *Room) Summarize(now time.Time, names map[string]string) Summary {
	s := Summary{People: []string{}, TogetherMs: now.Sub(r.startedAt).Milliseconds(), Reactions: r.reactions, PauseRequests: r.pauseAsked}
	for _, id := range r.everJoined {
		if p := r.people[id]; p != nil {
			s.People = append(s.People, p.Name)
		} else if n := names[id]; n != "" {
			s.People = append(s.People, n)
		}
	}
	best := -1
	for minute, byEmoji := range r.reactByMin {
		total := 0
		top, topN := "", 0
		for _, e := range Reactions {
			n := byEmoji[e]
			total += n
			if n > topN {
				top, topN = e, n
			}
		}
		if total > best || (total == best && minute*60000 < s.PeakMinuteMs) {
			best, s.PeakMinuteMs, s.PeakEmoji = total, minute*60000, top
		}
	}
	return s
}

// PresenceEntry is one person in party.presence (R20).
type PresenceEntry struct {
	Person
	Host       bool   `json:"host"`
	Guest      bool   `json:"guest"`
	CanControl bool   `json:"canControl"`
	State      string `json:"state"` // here, loading, waiting, away
	Screen     string `json:"screen,omitempty"`
	Quality    string `json:"quality,omitempty"`
	Companion  bool   `json:"companion"`
}

func (r *Room) presence() map[string]any {
	out := make([]PresenceEntry, 0, len(r.people))
	for _, id := range r.order {
		p := r.people[id]
		e := PresenceEntry{Person: p.Person, Host: id == r.host, Guest: p.Guest(), CanControl: r.CanControl(id), State: "here"}
		if s := p.screen(); s != nil {
			e.Screen, e.Quality = s.Device.Type, s.Quality
			if s.State == ScreenLoading {
				e.State = "loading"
			}
		}
		for _, d := range p.devices {
			if d.Kind == KindCompanion {
				e.Companion = true
			}
		}
		if r.waiting[id] {
			e.State = "waiting"
		}
		if !p.connected() {
			e.State = "away"
		}
		out = append(out, e)
	}
	return map[string]any{"people": out, "count": len(out), "max": r.MaxPeople, "queue": len(r.queue)}
}

// Snapshot is everything a joining client needs (party.snapshot).
func (r *Room) Snapshot(_ time.Time, me string) map[string]any {
	// Lists are never null: clients decode them as arrays.
	chat := append([]ChatMessage{}, r.chat...)
	asks := make([]map[string]any, 0, len(r.pendingAsks))
	ids := make([]int64, 0, len(r.pendingAsks))
	for id := range r.pendingAsks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		p := r.people[r.pendingAsks[id]]
		if p != nil {
			asks = append(asks, map[string]any{"requestId": id, "by": p.ID, "name": p.Name})
		}
	}
	out := map[string]any{
		"partyId": r.PartyID, "itemId": r.ItemID, "mode": r.Mode, "host": r.host, "me": me,
		"playback": r.pb, "presence": r.presence(), "chat": chat, "pauseRequests": asks,
		"reactions": Reactions,
	}
	if r.pauseAt != 0 {
		out["pauseAtServerMs"] = r.pauseAt
	}
	if r.pb.State == Waiting {
		out["waitingFor"] = r.waitingIDs()
	}
	return out
}

func (r *Room) waitingIDs() []string {
	ids := []string{}
	for _, id := range r.order {
		if r.waiting[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// Screens lists every person's screen state, for the lobby and next
// episode ("3 of 4 ready").
func (r *Room) Screens() (ready, total int) {
	for _, p := range r.people {
		if s := p.screen(); s != nil && p.connected() {
			total++
			if s.State != ScreenLoading && s.State != ScreenBuffering {
				ready++
			}
		}
	}
	return ready, total
}

// Connections returns every connection of the room, or of one person.
func (r *Room) Connections(person string) []string {
	var out []string
	for id, p := range r.people {
		if person != "" && id != person {
			continue
		}
		for c := range p.devices {
			out = append(out, c)
		}
	}
	return out
}

// HostConnections are the connections a Target{Host: true} reaches.
func (r *Room) HostConnections() []string {
	return r.Connections(r.host)
}

// QueuedConnections are the connections waiting in the queue.
func (r *Room) QueuedConnections() []string {
	out := make([]string, 0, len(r.queue))
	for _, q := range r.queue {
		out = append(out, q.conn)
	}
	return out
}
