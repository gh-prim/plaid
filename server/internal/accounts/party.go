package accounts

import (
	"context"
	"strconv"

	"github.com/gh-prim/plaid/server/internal/auth"
	"github.com/gh-prim/plaid/server/internal/settings"
)

// Plaid Party settings (step 21, stored ahead of F010, like the groups'
// party right; F010-R42).
const (
	partyEnabled   = "party.enabled"
	partyMaxPeople = "party.max_people"
	partyGuests    = "party.guests"
	partyUpload    = "party.upload_mbps"

	defaultPartyPeople = 8
	minPartyPeople     = 2
	maxPartyPeople     = 20
	maxUploadMbps      = 10000
)

// PartySettings is how Plaid Party works on this server.
type PartySettings struct {
	Enabled bool
	// MaxPeople per party, the host included.
	MaxPeople int
	// Guests can join a party from its link without an account.
	Guests bool
	// UploadMbps is the home connection's upload speed, 0 when unknown
	// (F010-R27).
	UploadMbps int
}

// PartyUpdate holds the fields to change; nil keeps a value.
type PartyUpdate struct {
	Enabled   *bool
	MaxPeople *int
	Guests    *bool
	// UploadMbps 0 forgets the value.
	UploadMbps *int
}

// Party returns the Plaid Party settings, defaults included.
func (s *Service) Party(ctx context.Context) (PartySettings, error) {
	st := settings.New(s.db)
	out := PartySettings{Enabled: true, MaxPeople: defaultPartyPeople}
	for key, set := range map[string]func(string){
		partyEnabled: func(v string) { out.Enabled = v == "1" },
		partyGuests:  func(v string) { out.Guests = v == "1" },
		partyUpload: func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				out.UploadMbps = n
			}
		},
		partyMaxPeople: func(v string) {
			if n, err := strconv.Atoi(v); err == nil {
				out.MaxPeople = n
			}
		},
	} {
		v, ok, err := st.Get(ctx, key)
		if err != nil {
			return out, err
		}
		if ok {
			set(v)
		}
	}
	return out, nil
}

// SetParty changes the Plaid Party settings.
func (s *Service) SetParty(ctx context.Context, actor auth.Principal, u PartyUpdate) (PartySettings, error) {
	var p problems
	if u.MaxPeople != nil && (*u.MaxPeople < minPartyPeople || *u.MaxPeople > maxPartyPeople) {
		p.add("maxPeople", "out_of_range")
	}
	if u.UploadMbps != nil && (*u.UploadMbps < 0 || *u.UploadMbps > maxUploadMbps) {
		p.add("uploadMbps", "out_of_range")
	}
	if err := p.err(); err != nil {
		return PartySettings{}, err
	}
	st := settings.New(s.db)
	flag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	if u.Enabled != nil {
		if err := st.Set(ctx, partyEnabled, flag(*u.Enabled)); err != nil {
			return PartySettings{}, err
		}
	}
	if u.Guests != nil {
		if err := st.Set(ctx, partyGuests, flag(*u.Guests)); err != nil {
			return PartySettings{}, err
		}
	}
	if u.MaxPeople != nil {
		if err := st.Set(ctx, partyMaxPeople, strconv.Itoa(*u.MaxPeople)); err != nil {
			return PartySettings{}, err
		}
	}
	if u.UploadMbps != nil {
		if err := st.Set(ctx, partyUpload, strconv.Itoa(*u.UploadMbps)); err != nil {
			return PartySettings{}, err
		}
	}
	s.logger.Info("party settings changed", "by", actor.UserID)
	return s.Party(ctx)
}
