package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type partyJSON struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Link        string `json:"link"`
	Status      string `json:"status"`
	ControlMode string `json:"controlMode"`
	Item        *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"item"`
	Invites []struct {
		Answer string `json:"answer"`
		Person struct {
			UserID string `json:"userId"`
			Name   string `json:"name"`
		} `json:"person"`
	} `json:"invites"`
	MyAnswer *string `json:"myAnswer"`
	Live     *struct {
		Count int    `json:"count"`
		State string `json:"state"`
	} `json:"live"`
}

// wsClient is a socket of the e2e server.
type wsClient struct {
	t  *testing.T
	ws *websocket.Conn
	n  int
}

func (e *playEnv) socket(srv *httptest.Server, token string) *wsClient {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + token}}})
	if err != nil {
		e.t.Fatalf("dial: %v", err)
	}
	e.t.Cleanup(func() { ws.CloseNow() })
	return &wsClient{t: e.t.(*testing.T), ws: ws}
}

type envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

// send sends a message with an id and returns the messages up to its ack
// or error.
func (c *wsClient) send(typ string, data any) (got []envelope, err string) {
	c.t.Helper()
	c.n++
	id := typ + "#" + string(rune('0'+c.n%10))
	raw, _ := json.Marshal(map[string]any{"type": typ, "id": id, "data": data})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := c.ws.Write(ctx, websocket.MessageText, raw); e != nil {
		c.t.Fatal(e)
	}
	for {
		_, b, e := c.ws.Read(ctx)
		if e != nil {
			c.t.Fatalf("read after %s: %v", typ, e)
		}
		var env envelope
		json.Unmarshal(b, &env)
		if env.ID == id && env.Type != "error" { // an ack, or a reply like sync.pong
			return append(got, env), ""
		}
		if env.ID == id && env.Type == "error" {
			var x struct{ Code string }
			json.Unmarshal(env.Data, &x)
			return got, x.Code
		}
		got = append(got, env)
	}
}

func has(got []envelope, typ string) bool {
	for _, g := range got {
		if g.Type == typ {
			return true
		}
	}
	return false
}

// TestPartyFlow covers F010-R1–R6: who may start, invitees joining by id
// without a code, others by code, guests limited to the party's title, and
// a party opening its title outside a person's libraries.
func TestPartyFlow(t *testing.T) {
	e := newPlayEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)

	// Bob only sees the shows; he may join parties, not start them.
	var libs []libraryJSON
	e.do("GET", "/libraries", nil, bearer(e.admin)).json(&libs)
	showsLib := ""
	for _, l := range libs {
		if l.Type == "shows" {
			showsLib = l.ID
		}
	}
	g := e.group(map[string]any{"name": "Shows", "libraryIds": []string{showsLib}}) // new groups may join parties
	bob := e.member(e.admin, "bob")
	e.expect(e.do("PATCH", "/users/"+bob.User.ID, map[string]any{"groupIds": []string{g.ID}}, bearer(e.admin)), http.StatusOK, "")
	carol := e.member(e.admin, "carol")
	e.expect(e.do("PATCH", "/users/"+carol.User.ID, map[string]any{"groupIds": []string{g.ID}}, bearer(e.admin)), http.StatusOK, "")
	bobTok := e.login("bob", "bob has a long password").Token
	carolTok := e.login("carol", "carol has a long password").Token

	e.expect(e.do("POST", "/parties", map[string]any{"itemId": e.movie}, bearer(bobTok)), http.StatusForbidden, "party_not_allowed")

	// The admin starts a live party on the movie and invites Bob.
	r := e.do("POST", "/parties", map[string]any{"itemId": e.movie, "invite": []string{bob.User.ID}}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var pt partyJSON
	r.json(&pt)
	if pt.Status != "live" || len(pt.Code) != 7 || pt.Code[3] != '-' || !strings.HasSuffix(pt.Link, "/p/"+strings.ReplaceAll(pt.Code, "-", "")) ||
		pt.Item == nil || pt.Item.Title != "Paddington" || len(pt.Invites) != 1 {
		t.Fatalf("party = %+v", pt)
	}

	host := e.socket(srv, e.admin)
	got, code := host.send("party.join", map[string]any{"partyId": pt.ID})
	if code != "" || !has(got, "party.snapshot") {
		t.Fatalf("host join: %q %v", code, got)
	}

	// Bob was invited: he joins by id, no code (R3).
	bs := e.socket(srv, bobTok)
	if got, code := bs.send("party.join", map[string]any{"partyId": pt.ID}); code != "" || !has(got, "party.snapshot") {
		t.Fatalf("invitee join by id: %q", code)
	}
	// The party opens the movie for him although his libraries don't hold
	// it (R6), but nothing else.
	e.expect(e.do("POST", "/items/"+e.movie+"/playback", map[string]any{"profile": external}, bearer(bobTok)), http.StatusNotFound, "")
	s := e.start(bobTok, e.movie, map[string]any{"profile": external, "partyId": pt.ID})
	if s.ID == "" {
		t.Fatal("no party session for bob")
	}
	e.expect(e.do("POST", "/items/"+e.dune+"/playback", map[string]any{"profile": external, "partyId": pt.ID}, bearer(bobTok)),
		http.StatusNotFound, "")

	// Carol wasn't invited: the id isn't enough, the code is.
	cs := e.socket(srv, carolTok)
	if _, code := cs.send("party.join", map[string]any{"partyId": pt.ID}); code != "not_allowed" {
		t.Fatalf("uninvited join by id: %q", code)
	}
	if _, code := cs.send("party.join", map[string]any{"code": strings.ToLower(pt.Code)}); code != "" {
		t.Fatalf("join by code: %q", code)
	}
	e.do("GET", "/parties/"+pt.ID, nil, bearer(e.admin)).json(&pt)
	if pt.Live == nil || pt.Live.Count != 3 {
		t.Errorf("live = %+v", pt.Live)
	}

	// Host-only control: Bob can't pause, he asks.
	if _, code := bs.send("party.control", map[string]any{"action": "pause"}); code != "no_control" {
		t.Errorf("bob paused: %q", code)
	}
	if _, code := bs.send("party.pause.request", nil); code != "" {
		t.Errorf("pause request: %q", code)
	}

	// Guests: off by default.
	e.expect(e.do("GET", "/party-codes/"+pt.Code, nil), http.StatusOK, "")
	e.expect(e.do("POST", "/party-codes/"+pt.Code+"/guest", map[string]any{"firstName": "Camille"}), http.StatusForbidden, "guests_not_allowed")
	e.expect(e.do("PATCH", "/settings/party", map[string]any{"guests": true}, bearer(e.admin)), http.StatusOK, "")
	r = e.do("POST", "/party-codes/"+pt.Code+"/guest", map[string]any{"firstName": "Camille"})
	e.expect(r, http.StatusCreated, "")
	var guest struct {
		Token   string `json:"token"`
		PartyID string `json:"partyId"`
	}
	r.json(&guest)
	if !strings.HasPrefix(guest.Token, "g_") || guest.PartyID != pt.ID {
		t.Fatalf("guest = %+v", guest)
	}
	e.expect(e.do("GET", "/parties/"+pt.ID, nil, bearer(guest.Token)), http.StatusOK, "")
	for _, path := range []string{"/libraries", "/me", "/items/" + e.movie, "/parties"} {
		e.expect(e.do("GET", path, nil, bearer(guest.Token)), http.StatusUnauthorized, "")
	}
	gs := e.socket(srv, guest.Token)
	if got, code := gs.send("party.join", map[string]any{}); code != "" || !has(got, "party.snapshot") {
		t.Fatalf("guest join: %q", code)
	}
	if s := e.start(guest.Token, e.movie, map[string]any{"profile": external}); s.ID == "" {
		t.Fatal("guest can't play the party's movie")
	}
	e.expect(e.do("POST", "/items/"+e.dune+"/playback", map[string]any{"profile": external}, bearer(guest.Token)),
		http.StatusNotFound, "")

	// Unknown codes count toward a backoff per address (R3).
	for range 10 {
		e.do("GET", "/party-codes/ZZZZZZ", nil)
	}
	e.expect(e.do("GET", "/party-codes/ZZZZZZ", nil), http.StatusTooManyRequests, "")

	// The host ends the party: the guest's token dies with it.
	if _, code := host.send("party.end", nil); code != "" {
		t.Fatalf("end: %q", code)
	}
	e.expect(e.do("GET", "/parties/"+pt.ID, nil, bearer(guest.Token)), http.StatusUnauthorized, "")
	e.do("GET", "/parties/"+pt.ID, nil, bearer(e.admin)).json(&pt)
	if pt.Status != "ended" {
		t.Errorf("status after end = %s", pt.Status)
	}
}

// TestScheduledPartyAnswers covers R28: scheduling, invitations and answers.
func TestScheduledPartyAnswers(t *testing.T) {
	e := newPlayEnv(t)
	bob := e.member(e.admin, "bob")
	friends := e.group(map[string]any{"name": "Friends", "libraryIds": []string{}})
	e.expect(e.do("PATCH", "/users/"+bob.User.ID, map[string]any{"groupIds": []string{friends.ID}}, bearer(e.admin)), http.StatusOK, "")
	when := e.clock.now().Add(24 * time.Hour)
	e.expect(e.do("POST", "/parties", map[string]any{"itemId": e.movie, "scheduledAt": e.clock.now().Add(5 * time.Minute)},
		bearer(e.admin)), http.StatusUnprocessableEntity, "validation_failed")
	r := e.do("POST", "/parties", map[string]any{"itemId": e.movie, "scheduledAt": when, "note": "Popcorn",
		"invite": []string{bob.User.ID}}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var pt partyJSON
	r.json(&pt)
	if pt.Status != "scheduled" || pt.Live != nil {
		t.Fatalf("party = %+v", pt)
	}
	bobTok := e.login("bob", "bob has a long password").Token
	e.expect(e.do("PUT", "/parties/"+pt.ID+"/answer", map[string]any{"answer": "maybe"}, bearer(bobTok)), http.StatusNoContent, "")
	var mine []partyJSON
	e.do("GET", "/parties", nil, bearer(bobTok)).json(&mine)
	if len(mine) != 1 || mine[0].MyAnswer == nil || *mine[0].MyAnswer != "maybe" {
		t.Fatalf("bob's parties = %+v", mine)
	}
	var people []struct {
		UserID string `json:"userId"`
	}
	e.do("GET", "/parties/people", nil, bearer(e.admin)).json(&people)
	if len(people) != 1 || people[0].UserID != bob.User.ID {
		t.Errorf("people = %+v", people)
	}
	e.expect(e.do("DELETE", "/parties/"+pt.ID, nil, bearer(bobTok)), http.StatusForbidden, "")
	e.expect(e.do("DELETE", "/parties/"+pt.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	e.do("GET", "/parties", nil, bearer(bobTok)).json(&mine)
	if len(mine) != 0 {
		t.Errorf("cancelled party still listed: %+v", mine)
	}
}

// next reads until a message of type typ.
func (c *wsClient) next(typ string) envelope {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, b, err := c.ws.Read(ctx)
		if err != nil {
			c.t.Fatalf("waiting for %s: %v", typ, err)
		}
		var env envelope
		json.Unmarshal(b, &env)
		if env.Type == typ {
			return env
		}
	}
}

func (e *playEnv) loginOn(device map[string]any) authResult {
	e.t.Helper()
	r := e.do("POST", "/auth/login", map[string]any{"username": "Ada", "password": adminPassword, "device": device})
	e.expect(r, http.StatusOK, "")
	var res authResult
	r.json(&res)
	return res
}

// TestPartyOnTheTV covers R22 and R23: the phone sends the party to an
// online Apple TV of the same account and becomes its companion; a party
// started on the TV offers the phone to join as companion.
func TestPartyOnTheTV(t *testing.T) {
	e := newPlayEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	tvLogin := e.loginOn(tv)
	phoneLogin := e.loginOn(map[string]any{"name": "iPhone", "type": "ios"})
	tvSock := e.socket(srv, tvLogin.Token)
	phone := e.socket(srv, phoneLogin.Token)

	got, _ := phone.send("party.screens", nil)
	var screens struct {
		Screens []struct{ SessionID, Name string } `json:"screens"`
	}
	for _, g := range got {
		if g.Type == "party.screens" {
			json.Unmarshal(g.Data, &screens)
		}
	}
	if len(screens.Screens) != 1 || screens.Screens[0].SessionID != tvLogin.Session.ID {
		t.Fatalf("screens = %+v", screens)
	}

	var pt partyJSON
	e.do("POST", "/parties", map[string]any{"itemId": e.movie}, bearer(phoneLogin.Token)).json(&pt)
	if _, code := phone.send("party.join", map[string]any{"partyId": pt.ID, "screen": tvLogin.Session.ID}); code != "" {
		t.Fatalf("join with the TV: %q", code)
	}
	var cast struct{ PartyID string }
	json.Unmarshal(tvSock.next("tv.cast").Data, &cast)
	if cast.PartyID != pt.ID {
		t.Fatalf("tv.cast = %+v", cast)
	}
	if _, code := tvSock.send("party.join", map[string]any{"partyId": pt.ID}); code != "" {
		t.Fatalf("tv join: %q", code)
	}
	var pres struct {
		People []struct {
			Screen    string `json:"screen"`
			Companion bool   `json:"companion"`
		} `json:"people"`
	}
	json.Unmarshal(phone.next("party.presence").Data, &pres)
	if len(pres.People) != 1 || pres.People[0].Screen != "tvos" || !pres.People[0].Companion {
		t.Errorf("presence = %+v", pres)
	}
	if _, code := phone.send("party.join", map[string]any{"partyId": pt.ID, "screen": "nope"}); code != "screen_offline" {
		t.Errorf("unknown screen: %q", code)
	}

	// A party started on the TV offers the phone to join (R23).
	phone.send("party.leave", nil)
	var other partyJSON
	e.do("POST", "/parties", map[string]any{"itemId": e.dune}, bearer(tvLogin.Token)).json(&other)
	tvSock.send("party.leave", nil)
	if _, code := tvSock.send("party.join", map[string]any{"partyId": other.ID}); code != "" {
		t.Fatalf("tv join: %q", code)
	}
	var offer struct{ PartyID string }
	json.Unmarshal(phone.next("party.companion").Data, &offer)
	if offer.PartyID != other.ID {
		t.Errorf("companion offer = %+v", offer)
	}
}
