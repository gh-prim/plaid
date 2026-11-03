package party

import (
	"context"
	"errors"
	"time"

	"github.com/gh-prim/plaid/server/internal/apierr"
	"github.com/gh-prim/plaid/server/internal/auth"
)

// Vote phases.
const (
	PhaseProposals = "proposals"
	PhaseRanking   = "ranking"
	PhaseClosed    = "closed"
)

// VoteView is a vote as the caller sees it: everything but other people's
// ballots (R32).
type VoteView struct {
	Vote      Vote
	Phase     string
	Proposals []ProposalView
	Voters    []Voter
	Mine      Ballot
	Result    *Result
}

// ProposalView is a proposal with its title.
type ProposalView struct {
	Item       Item
	ProposedBy string
	ProposerID string
	Mine       bool
}

// Voter is someone who may vote, and whether they did.
type Voter struct {
	UserID string
	Name   string
	Voted  bool
}

func (v Vote) phase(now time.Time) string {
	switch {
	case v.ClosedAt != nil || !now.Before(v.RankingUntil):
		return PhaseClosed
	case now.Before(v.ProposalsUntil):
		return PhaseProposals
	}
	return PhaseRanking
}

// voters are the organizer and the invitees: accounts, by user id.
func (s *Service) voters(ctx context.Context, pt Party) ([]string, error) {
	ids, err := s.inviteeIDs(ctx, pt.ID)
	if err != nil {
		return nil, err
	}
	return append([]string{pt.OrganizerUserID}, ids...), nil
}

func (s *Service) voteParty(ctx context.Context, p auth.Principal, id string) (Party, Vote, error) {
	if p.IsGuest() {
		return Party{}, Vote{}, errNotAllowed
	}
	pt, err := s.Get(ctx, p, id)
	if err != nil {
		return Party{}, Vote{}, err
	}
	v, err := s.store.Vote(ctx, pt.ID)
	if errors.Is(err, ErrNotFound) {
		return Party{}, Vote{}, apierr.NotFound("this party has no vote")
	}
	return pt, v, err
}

// VoteState returns the caller's view of a vote.
func (s *Service) VoteState(ctx context.Context, p auth.Principal, id string) (VoteView, error) {
	pt, v, err := s.voteParty(ctx, p, id)
	if err != nil {
		return VoteView{}, err
	}
	return s.voteView(ctx, p, pt, v)
}

func (s *Service) voteView(ctx context.Context, p auth.Principal, pt Party, v Vote) (VoteView, error) {
	out := VoteView{Vote: v, Phase: v.phase(s.o.Now())}
	names, err := s.userNames(ctx)
	if err != nil {
		return out, err
	}
	props, err := s.store.Proposals(ctx, pt.ID)
	if err != nil {
		return out, err
	}
	for _, pr := range props {
		it, err := s.store.Item(ctx, pr.ItemID)
		if err != nil {
			continue
		}
		out.Proposals = append(out.Proposals, ProposalView{Item: it, ProposedBy: names[pr.UserID], ProposerID: pr.UserID, Mine: pr.UserID == p.UserID})
	}
	ballots, err := s.store.Ballots(ctx, pt.ID)
	if err != nil {
		return out, err
	}
	voted := map[string]bool{}
	for _, b := range ballots {
		voted[b.UserID] = true
		if b.UserID == p.UserID {
			out.Mine = b
		}
	}
	vs, err := s.voters(ctx, pt)
	if err != nil {
		return out, err
	}
	for _, u := range vs {
		out.Voters = append(out.Voters, Voter{UserID: u, Name: names[u], Voted: voted[u]})
	}
	if out.Phase == PhaseClosed {
		r := Tally(props, ballots, v.Seed)
		out.Result = &r
	}
	return out, nil
}

func (s *Service) userNames(ctx context.Context) (map[string]string, error) {
	users, err := s.o.Accounts.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, u := range users {
		out[u.ID] = u.DisplayName
	}
	return out, nil
}

func (s *Service) isVoter(ctx context.Context, pt Party, userID string) bool {
	vs, err := s.voters(ctx, pt)
	if err != nil {
		return false
	}
	for _, v := range vs {
		if v == userID {
			return true
		}
	}
	return false
}

// visibleToAll checks an item can be seen by every voter (R31): proposals
// are limited to what the whole group may watch.
func (s *Service) visibleToAll(ctx context.Context, pt Party, itemID string) error {
	it, err := s.store.Item(ctx, itemID)
	if err != nil || (it.Kind != "movie" && it.Kind != "episode") {
		return apierr.NotFound("no such item")
	}
	vs, err := s.voters(ctx, pt)
	if err != nil {
		return err
	}
	profiles, err := s.o.Accounts.DefaultProfiles(ctx)
	if err != nil {
		return err
	}
	for _, u := range vs {
		lim, err := s.o.Accounts.AccessLimit(ctx, auth.Principal{UserID: u, ProfileID: profiles[u].ID})
		if err != nil {
			return err
		}
		if !lim.Allows(it.LibraryID, it.AgeRating) {
			return apierr.New(422, "not_for_everyone", "someone in the party can't watch this title")
		}
	}
	return nil
}

// Propose adds a proposal (R31).
func (s *Service) Propose(ctx context.Context, p auth.Principal, id, itemID string) (VoteView, error) {
	pt, v, err := s.voteParty(ctx, p, id)
	if err != nil {
		return VoteView{}, err
	}
	if v.phase(s.o.Now()) != PhaseProposals {
		return VoteView{}, apierr.New(409, "vote_phase", "proposals are closed")
	}
	if !s.isVoter(ctx, pt, p.UserID) {
		return VoteView{}, errNotAllowed
	}
	if err := s.visibleToAll(ctx, pt, itemID); err != nil {
		return VoteView{}, err
	}
	switch err := s.store.Propose(ctx, pt.ID, p.UserID, itemID, s.o.Now()); {
	case errors.Is(err, ErrTooManyProps):
		return VoteView{}, apierr.New(409, "too_many_proposals", "two proposals each")
	case errors.Is(err, ErrAlreadyPropose):
		return VoteView{}, apierr.New(409, "already_proposed", "this title is already proposed")
	case err != nil:
		return VoteView{}, err
	}
	return s.voteView(ctx, p, pt, v)
}

// Withdraw removes the caller's own proposal.
func (s *Service) Withdraw(ctx context.Context, p auth.Principal, id, itemID string) (VoteView, error) {
	pt, v, err := s.voteParty(ctx, p, id)
	if err != nil {
		return VoteView{}, err
	}
	if v.phase(s.o.Now()) != PhaseProposals {
		return VoteView{}, apierr.New(409, "vote_phase", "proposals are closed")
	}
	if err := s.store.Withdraw(ctx, pt.ID, p.UserID, itemID); err != nil {
		return VoteView{}, err
	}
	return s.voteView(ctx, p, pt, v)
}

// Cast saves the caller's ballot (R32).
func (s *Service) Cast(ctx context.Context, p auth.Principal, id string, ranking []string, veto string) (VoteView, error) {
	pt, v, err := s.voteParty(ctx, p, id)
	if err != nil {
		return VoteView{}, err
	}
	if v.phase(s.o.Now()) != PhaseRanking {
		return VoteView{}, apierr.New(409, "vote_phase", "ranking isn't open")
	}
	if !s.isVoter(ctx, pt, p.UserID) {
		return VoteView{}, errNotAllowed
	}
	props, err := s.store.Proposals(ctx, pt.ID)
	if err != nil {
		return VoteView{}, err
	}
	known := map[string]bool{}
	for _, pr := range props {
		known[pr.ItemID] = true
	}
	seen := map[string]bool{}
	if len(ranking) == 0 || len(ranking) > 3 {
		return VoteView{}, apierr.ValidationFailed(apierr.Detail{Field: "ranking", Code: "out_of_range"})
	}
	for _, r := range ranking {
		if !known[r] || seen[r] || r == veto {
			return VoteView{}, apierr.ValidationFailed(apierr.Detail{Field: "ranking", Code: "invalid"})
		}
		seen[r] = true
	}
	if veto != "" && !known[veto] {
		return VoteView{}, apierr.ValidationFailed(apierr.Detail{Field: "veto", Code: "invalid"})
	}
	if err := s.store.SaveBallot(ctx, pt.ID, Ballot{UserID: p.UserID, Ranks: ranking, Veto: veto}, s.o.Now()); err != nil {
		return VoteView{}, err
	}
	return s.voteView(ctx, p, pt, v)
}

// closeVote tallies a vote past its deadline (R33): the seed is stored
// before the draw, the winner becomes the party's title.
func (s *Service) closeVote(ctx context.Context, pt Party) error {
	v, err := s.store.Vote(ctx, pt.ID)
	if err != nil || v.ClosedAt != nil {
		return err
	}
	if v.Seed == "" {
		if err := s.store.SetSeed(ctx, pt.ID, NewSeed()); err != nil {
			return err
		}
		if v, err = s.store.Vote(ctx, pt.ID); err != nil {
			return err
		}
	}
	props, err := s.store.Proposals(ctx, pt.ID)
	if err != nil {
		return err
	}
	ballots, err := s.store.Ballots(ctx, pt.ID)
	if err != nil {
		return err
	}
	r := Tally(props, ballots, v.Seed)
	now := s.o.Now()
	if err := s.store.CloseVote(ctx, pt.ID, r.Winner, now); err != nil {
		return err
	}
	pt.Status = StatusScheduled
	pt.ItemID = r.Winner
	if err := s.store.Update(ctx, pt); err != nil {
		return err
	}
	users, _ := s.voters(ctx, pt)
	kind := "party.vote.closed"
	if r.Winner == "" {
		kind = "party.vote.pick" // the host picks (R33)
	}
	s.notifyInvites(pt, users, kind)
	s.o.Log.Info("party vote closed", "party_id", pt.ID, "winner", r.Winner, "tied", len(r.Tied))
	return nil
}

// StartRanking closes the proposals now (the host's "Rank now"): every
// voter is told the ranking is open.
func (s *Service) StartRanking(ctx context.Context, p auth.Principal, id string) (VoteView, error) {
	pt, v, err := s.voteParty(ctx, p, id)
	if err != nil {
		return VoteView{}, err
	}
	if pt.OrganizerUserID != p.UserID && !p.IsAdmin() {
		return VoteView{}, errNotAllowed
	}
	now := s.o.Now()
	if v.phase(now) != PhaseProposals {
		return VoteView{}, apierr.New(409, "vote_not_proposing", "the proposals are already closed")
	}
	if err := s.store.EndProposals(ctx, pt.ID, now); err != nil {
		return VoteView{}, err
	}
	v.ProposalsUntil = now
	users, _ := s.voters(ctx, pt)
	s.notifyInvites(pt, users, "party.vote.ranking")
	return s.voteView(ctx, p, pt, v)
}

// maxSuggestions caps R34's list.
const maxSuggestions = 12

// Suggestions are titles no voter has watched (R34).
func (s *Service) Suggestions(ctx context.Context, p auth.Principal, id string) ([]Item, error) {
	pt, _, err := s.voteParty(ctx, p, id)
	if err != nil {
		return nil, err
	}
	vs, err := s.voters(ctx, pt)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.Unwatched(ctx, vs, maxSuggestions*3)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, itemID := range ids {
		if len(out) == maxSuggestions {
			break
		}
		if s.visibleToAll(ctx, pt, itemID) != nil {
			continue
		}
		if it, err := s.store.Item(ctx, itemID); err == nil {
			out = append(out, it)
		}
	}
	return out, nil
}
