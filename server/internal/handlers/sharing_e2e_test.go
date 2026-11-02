package handlers_test

import (
	"image/png"
	"net/http"
	"net/url"
	"slices"
	"testing"
)

// These tests drive F009: groups, exceptions, household-only libraries,
// invitations and avatars.

type groupJSON struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Household  bool     `json:"household"`
	LibraryIDs []string `json:"libraryIds"`
	MemberIDs  []string `json:"memberIds"`
	Rights     struct {
		MaxQuality        string `json:"maxQuality"`
		MaxStreams        *int   `json:"maxStreams"`
		CanCreateProfiles bool   `json:"canCreateProfiles"`
	} `json:"rights"`
}

func (e *libEnv) group(body map[string]any) groupJSON {
	e.t.Helper()
	r := e.do("POST", "/groups", body, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var g groupJSON
	r.json(&g)
	return g
}

func (e *libEnv) libraryNames(token string) []string {
	e.t.Helper()
	r := e.do("GET", "/libraries", nil, bearer(token))
	e.expect(r, http.StatusOK, "")
	var libs []libraryJSON
	r.json(&libs)
	out := []string{}
	for _, l := range libs {
		out = append(out, l.Name)
	}
	slices.Sort(out)
	return out
}

func TestGroupsDecideWhatMembersSee(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	films, kids, home := t.TempDir(), t.TempDir(), t.TempDir()
	touch(t, films, "Dune (2021).mkv")
	touch(t, kids, "Paddington (2014).mkv")
	touch(t, home, "Birthday (2020).mkv")
	fl := e.create("Films", "movies", films)
	kl := e.create("Kids", "movies", kids)
	hl := e.create("Home videos", "movies", home)
	for _, l := range []libraryJSON{fl, kl, hl} {
		e.waitIdle(l.ID)
	}

	// Home videos never leave the household.
	r := e.do("PATCH", "/libraries/"+hl.ID, map[string]any{"householdOnly": true}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")

	friends := e.group(map[string]any{"name": "Close friends", "libraryIds": []string{fl.ID}})
	if friends.Rights.MaxQuality != "1080p" || friends.Rights.MaxStreams == nil || *friends.Rights.MaxStreams != 2 ||
		friends.Rights.CanCreateProfiles {
		t.Errorf("new group rights = %+v, want 1080p, 2 streams, no profiles", friends.Rights)
	}
	e.expect(e.do("POST", "/groups", map[string]any{"name": "Abroad", "libraryIds": []string{hl.ID}}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")

	camille := e.member(e.admin, "Camille")
	// A member in no group sees nothing.
	if got := e.libraryNames(camille.Token); len(got) != 0 {
		t.Errorf("member without a group sees %v", got)
	}
	e.expect(e.do("PATCH", "/users/"+camille.User.ID, map[string]any{"groupIds": []string{friends.ID}}, bearer(e.admin)),
		http.StatusOK, "")
	if got := e.libraryNames(camille.Token); !slices.Equal(got, []string{"Films"}) {
		t.Errorf("Close friends see %v, want [Films]", got)
	}
	e.expect(e.do("GET", "/libraries/"+kl.ID, nil, bearer(camille.Token)), http.StatusNotFound, "not_found")
	e.expect(e.do("GET", "/libraries/"+kl.ID+"/items", nil, bearer(camille.Token)), http.StatusNotFound, "not_found")
	var page itemPage
	e.do("GET", "/libraries/"+kl.ID+"/items", nil, bearer(e.admin)).json(&page)
	paddington := page.Items[0].ID
	e.expect(e.do("GET", "/items/"+paddington, nil, bearer(camille.Token)), http.StatusNotFound, "not_found")
	var found struct {
		Movies []string `json:"movies"`
	}
	e.do("GET", "/search?q=paddington", nil, bearer(camille.Token)).json(&found)
	if len(found.Movies) != 0 {
		t.Errorf("search finds %v in a library that isn't shared", found.Movies)
	}

	// An exception adds Kids for Camille alone.
	e.expect(e.do("PATCH", "/users/"+camille.User.ID, map[string]any{"extraLibraryIds": []string{kl.ID},
		"extraRights": map[string]any{"maxQuality": "2160p"}}, bearer(e.admin)), http.StatusOK, "")
	if got := e.libraryNames(camille.Token); !slices.Equal(got, []string{"Films", "Kids"}) {
		t.Errorf("with the exception Camille sees %v", got)
	}
	e.expect(e.do("GET", "/items/"+paddington, nil, bearer(camille.Token)), http.StatusOK, "")
	e.expect(e.do("PATCH", "/users/"+camille.User.ID, map[string]any{"extraLibraryIds": []string{hl.ID}}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")

	var me struct {
		Access struct {
			LibraryIDs []string `json:"libraryIds"`
			Rights     struct {
				MaxQuality string `json:"maxQuality"`
			} `json:"rights"`
		} `json:"access"`
	}
	e.do("GET", "/me", nil, bearer(camille.Token)).json(&me)
	if me.Access.Rights.MaxQuality != "2160p" || len(me.Access.LibraryIDs) != 2 {
		t.Errorf("GET /me access = %+v", me.Access)
	}
	// A friend can't add profiles unless a group lets them.
	e.expect(e.do("POST", "/me/profiles", map[string]any{"name": "Léo"}, bearer(camille.Token)), http.StatusForbidden, "forbidden")

	// The People list shows how access is built.
	var people []struct {
		ID              string   `json:"id"`
		GroupIDs        []string `json:"groupIds"`
		ExtraLibraryIDs []string `json:"extraLibraryIds"`
		Access          struct {
			LibraryIDs *[]string `json:"libraryIds"`
		} `json:"access"`
	}
	e.do("GET", "/users", nil, bearer(e.admin)).json(&people)
	for _, p := range people {
		switch p.ID {
		case camille.User.ID:
			if !slices.Equal(p.GroupIDs, []string{friends.ID}) || !slices.Equal(p.ExtraLibraryIDs, []string{kl.ID}) {
				t.Errorf("Camille in People: %+v", p)
			}
		default: // the admin sees everything
			if p.Access.LibraryIDs != nil {
				t.Errorf("admin access libraries = %v, want null", *p.Access.LibraryIDs)
			}
		}
	}

	// A household group sees everything, home videos included.
	family := e.group(map[string]any{"name": "Home", "household": true})
	e.expect(e.do("PATCH", "/users/"+camille.User.ID, map[string]any{"groupIds": []string{friends.ID, family.ID}}, bearer(e.admin)),
		http.StatusOK, "")
	if got := e.libraryNames(camille.Token); !slices.Equal(got, []string{"Films", "Home videos", "Kids"}) {
		t.Errorf("household member sees %v", got)
	}
	e.expect(e.do("POST", "/me/profiles", map[string]any{"name": "Léo"}, bearer(camille.Token)), http.StatusCreated, "")

	// Deleting the group takes it away at once.
	e.expect(e.do("DELETE", "/groups/"+family.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	if got := e.libraryNames(camille.Token); !slices.Equal(got, []string{"Films", "Kids"}) {
		t.Errorf("after the household group is deleted Camille sees %v", got)
	}

	// Suspending signs her out everywhere.
	e.expect(e.do("PATCH", "/users/"+camille.User.ID, map[string]any{"disabled": true}, bearer(e.admin)), http.StatusOK, "")
	e.expect(e.do("GET", "/libraries", nil, bearer(camille.Token)), http.StatusUnauthorized, "unauthorized")
}

func TestInvitation(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	films := t.TempDir()
	touch(t, films, "Dune (2021).mkv")
	fl := e.create("Films", "movies", films)
	e.waitIdle(fl.ID)
	friends := e.group(map[string]any{"name": "Close friends", "libraryIds": []string{fl.ID}})

	r := e.do("POST", "/invitations", map[string]any{"name": "Camille", "groupIds": []string{friends.ID}}, bearer(e.admin))
	e.expect(r, http.StatusCreated, "")
	var created struct {
		Token      string `json:"token"`
		Invitation struct {
			ID string `json:"id"`
		} `json:"invitation"`
	}
	r.json(&created)
	var pending []struct {
		ID     string
		Status string
		Token  string
		UserID string `json:"userId"`
	}
	e.do("GET", "/invitations", nil, bearer(e.admin)).json(&pending)
	if len(pending) != 1 || pending[0].ID != created.Invitation.ID || pending[0].Status != "pending" ||
		pending[0].Token != created.Token {
		t.Fatalf("pending = %+v", pending)
	}

	var prev struct {
		ServerName string `json:"serverName"`
		Name       string `json:"name"`
		InvitedBy  string `json:"invitedBy"`
	}
	r = e.do("POST", "/invite/check", map[string]any{"token": created.Token})
	e.expect(r, http.StatusOK, "")
	r.json(&prev)
	if prev.ServerName != "test-server" || prev.Name != "Camille" || prev.InvitedBy != "Ada" {
		t.Errorf("preview = %+v", prev)
	}
	e.expect(e.do("POST", "/invite/check", map[string]any{"token": "nope"}), http.StatusNotFound, "invitation_not_found")

	accept := map[string]any{"token": created.Token, "displayName": "Camille", "username": "camille",
		"password": "camille likes long passwords", "device": laptop}
	e.expect(e.do("POST", "/invite/accept", accept), http.StatusUnprocessableEntity, "validation_failed") // no PIN
	accept["pin"] = "1234"
	r = e.do("POST", "/invite/accept", accept)
	e.expect(r, http.StatusCreated, "")
	var res authResult
	r.json(&res)
	if res.User.Role != "member" || len(res.Profiles) != 1 || !res.Profiles[0].HasPin {
		t.Errorf("accepted = %+v", res)
	}
	if got := e.libraryNames(res.Token); !slices.Equal(got, []string{"Films"}) {
		t.Errorf("invited member sees %v", got)
	}
	// The link works once.
	accept["username"] = "camille2"
	e.expect(e.do("POST", "/invite/accept", accept), http.StatusGone, "invitation_expired")
	pending = nil
	e.do("GET", "/invitations", nil, bearer(e.admin)).json(&pending)
	if len(pending) != 1 || pending[0].Status != "accepted" || pending[0].Token != "" || pending[0].UserID != res.User.ID {
		t.Errorf("a used invitation is still pending, or lost its account: %+v", pending)
	}

	// A cancelled one can't be used.
	e.expect(e.do("POST", "/invitations", map[string]any{"name": "Paul", "groupIds": []string{}}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	r = e.do("POST", "/invitations", map[string]any{"name": "Paul", "groupIds": []string{friends.ID}, "expiresInDays": 1}, bearer(e.admin))
	r.json(&created)
	e.expect(e.do("DELETE", "/invitations/"+created.Invitation.ID, nil, bearer(e.admin)), http.StatusNoContent, "")
	e.expect(e.do("POST", "/invite/check", map[string]any{"token": created.Token}), http.StatusGone, "invitation_expired")
	pending = nil
	e.do("GET", "/invitations", nil, bearer(e.admin)).json(&pending)
	if len(pending) != 1 || pending[0].Status != "accepted" {
		t.Errorf("a cancelled invitation is still listed: %+v", pending)
	}
	// Members can't invite.
	e.expect(e.do("POST", "/invitations", map[string]any{"name": "X", "groupIds": []string{}}, bearer(res.Token)),
		http.StatusForbidden, "forbidden")
}

func TestAvatars(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	var builder struct {
		Parts []struct {
			Name     string   `json:"name"`
			Variants []string `json:"variants"`
		} `json:"parts"`
		Defaults map[string]any `json:"defaults"`
	}
	r := e.do("GET", "/avatar-builder", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&builder)
	if len(builder.Parts) != 10 || builder.Defaults["hair"] == nil {
		t.Fatalf("builder = %+v", builder)
	}

	var me struct {
		Profiles []struct {
			ID     string `json:"id"`
			Avatar struct {
				Kind string  `json:"kind"`
				URL  *string `json:"url"`
			} `json:"avatar"`
		} `json:"profiles"`
	}
	e.do("GET", "/me", nil, bearer(e.admin)).json(&me)
	id := me.Profiles[0].ID
	if me.Profiles[0].Avatar.Kind != "initial" || me.Profiles[0].Avatar.URL != nil {
		t.Errorf("new profile avatar = %+v", me.Profiles[0].Avatar)
	}
	e.expect(e.do("GET", "/avatars/"+id, nil, bearer(e.admin)), http.StatusNotFound, "not_found")

	opts := builder.Defaults
	opts["glasses"] = "variant02"
	r = e.do("PUT", "/me/profiles/"+id+"/avatar", map[string]any{"kind": "builder", "options": opts}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	var pr struct {
		Avatar struct {
			Kind string  `json:"kind"`
			URL  *string `json:"url"`
		} `json:"avatar"`
	}
	r.json(&pr)
	if pr.Avatar.Kind != "builder" || pr.Avatar.URL == nil {
		t.Fatalf("avatar = %+v", pr.Avatar)
	}
	r = e.do("GET", *pr.Avatar.URL+"&size=64", nil, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	img, err := png.Decode(r.Body)
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("avatar png: %v %v", err, img)
	}
	e.expect(e.do("GET", *pr.Avatar.URL, nil), http.StatusUnauthorized, "unauthorized")

	opts["glasses"] = "nope"
	e.expect(e.do("PUT", "/me/profiles/"+id+"/avatar", map[string]any{"kind": "builder", "options": opts}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	opts["glasses"] = nil
	q := url.Values{"size": {"48"}, "color": {"3"}}
	raw := `{"head":"variant01","hair":"variant02","eyes":"variant01","eyebrows":"variant01","mouth":"happy01","nose":"variant01","hairColor":"#5b3a26","skinColor":"#d9a07a"}`
	q.Set("options", raw)
	e.expect(e.do("GET", "/avatar-builder/render?"+q.Encode(), nil, bearer(e.admin)), http.StatusOK, "")

	// Back to the initial.
	r = e.do("PUT", "/me/profiles/"+id+"/avatar", map[string]any{"kind": "initial"}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	pr.Avatar.URL = nil
	r.json(&pr)
	if pr.Avatar.Kind != "initial" || pr.Avatar.URL != nil {
		t.Errorf("back to initial: %+v", pr.Avatar)
	}
}

// inFamily puts a member in the Family group every install starts with,
// so they see every library like members did before F009.
func (e *env) inFamily(admin, userID string) {
	e.t.Helper()
	var groups []groupJSON
	e.do("GET", "/groups", nil, bearer(admin)).json(&groups)
	for _, g := range groups {
		if g.Name == "Family" && g.Household {
			e.expect(e.do("PATCH", "/users/"+userID, map[string]any{"groupIds": []string{g.ID}}, bearer(admin)), http.StatusOK, "")
			return
		}
	}
	e.t.Fatal("no Family group")
}

// Step 21: Plaid Party settings, stored ahead of F010.
func TestPartySettings(t *testing.T) {
	e := newLibEnv(t, libOpts{})
	var s struct {
		Enabled   bool `json:"enabled"`
		MaxPeople int  `json:"maxPeople"`
		Guests    bool `json:"guests"`
	}
	e.do("GET", "/settings/party", nil, bearer(e.admin)).json(&s)
	if !s.Enabled || s.MaxPeople != 8 || s.Guests {
		t.Fatalf("defaults = %+v", s)
	}
	r := e.do("PATCH", "/settings/party", map[string]any{"maxPeople": 4, "guests": true}, bearer(e.admin))
	e.expect(r, http.StatusOK, "")
	r.json(&s)
	if !s.Enabled || s.MaxPeople != 4 || !s.Guests {
		t.Errorf("after = %+v", s)
	}
	e.expect(e.do("PATCH", "/settings/party", map[string]any{"maxPeople": 40}, bearer(e.admin)),
		http.StatusUnprocessableEntity, "validation_failed")
	e.expect(e.do("PATCH", "/settings/party", map[string]any{"enabled": false}, bearer(e.admin)), http.StatusOK, "")
	e.do("GET", "/settings/party", nil, bearer(e.admin)).json(&s)
	if s.Enabled || s.MaxPeople != 4 {
		t.Errorf("stored = %+v", s)
	}
	member := e.member(e.admin, "bob")
	e.expect(e.do("GET", "/settings/party", nil, bearer(member.Token)), http.StatusForbidden, "forbidden")
}
