package party

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"time"
)

// Votes (R31–R34): proposals, anonymous ballots, a tally with a seeded tie
// draw anyone can check again.

// OpenVote stores a vote's deadlines.
func (s *Store) OpenVote(ctx context.Context, partyID string, proposals, ranking time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO party_votes (party_id, proposals_until, ranking_until) VALUES (?,?,?)`,
		partyID, ts(proposals), ts(ranking))
	return err
}

// Vote is a vote's stored state.
type Vote struct {
	PartyID        string
	ProposalsUntil time.Time
	RankingUntil   time.Time
	Seed           string
	ClosedAt       *time.Time
	WinnerItemID   string
}

// Vote returns a party's vote, or ErrNotFound.
func (s *Store) Vote(ctx context.Context, partyID string) (Vote, error) {
	var v Vote
	var pu, ru string
	var seed, closed, winner sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT party_id, proposals_until, ranking_until, seed, closed_at, winner_item_id
		FROM party_votes WHERE party_id = ?`, partyID).Scan(&v.PartyID, &pu, &ru, &seed, &closed, &winner)
	if errors.Is(err, sql.ErrNoRows) {
		return Vote{}, ErrNotFound
	}
	if err != nil {
		return Vote{}, err
	}
	v.ProposalsUntil, v.RankingUntil, v.ClosedAt = parseTS(pu), parseTS(ru), optTS(closed)
	v.Seed, v.WinnerItemID = seed.String, winner.String
	return v, nil
}

// Proposal is a proposed item and who proposed it.
type Proposal struct {
	ItemID string
	UserID string
}

// Proposals lists a vote's proposals in proposal order.
func (s *Store) Proposals(ctx context.Context, partyID string) ([]Proposal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_id, user_id FROM party_proposals WHERE party_id = ? ORDER BY created_at, item_id`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Proposal
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(&p.ItemID, &p.UserID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// maxProposals is how many items one person proposes (R31).
const maxProposals = 2

// Vote errors.
var (
	ErrVoteClosed     = errors.New("vote_closed")
	ErrTooManyProps   = errors.New("too_many_proposals")
	ErrAlreadyPropose = errors.New("already_proposed")
	ErrBadBallot      = errors.New("bad_ballot")
)

// Propose adds userID's proposal, at most maxProposals each, one per item.
func (s *Store) Propose(ctx context.Context, partyID, userID, itemID string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM party_proposals WHERE party_id = ? AND user_id = ?`,
		partyID, userID).Scan(&n); err != nil {
		return err
	}
	if n >= maxProposals {
		return ErrTooManyProps
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO party_proposals (party_id, item_id, user_id, created_at) VALUES (?,?,?,?)`,
		partyID, itemID, userID, ts(at)); err != nil {
		if isUnique(err) {
			return ErrAlreadyPropose
		}
		return err
	}
	return tx.Commit()
}

// Withdraw removes userID's own proposal.
func (s *Store) Withdraw(ctx context.Context, partyID, userID, itemID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM party_proposals WHERE party_id = ? AND user_id = ? AND item_id = ?`,
		partyID, userID, itemID)
	return err
}

// Ballot is one voter's top 3 and veto.
type Ballot struct {
	UserID string
	Ranks  []string // best first, up to 3
	Veto   string
}

// SaveBallot replaces userID's ballot.
func (s *Store) SaveBallot(ctx context.Context, partyID string, b Ballot, at time.Time) error {
	rank := func(i int) any {
		if i < len(b.Ranks) {
			return b.Ranks[i]
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO party_ballots (party_id, user_id, first_item, second_item, third_item, veto_item, updated_at)
		VALUES (?,?,?,?,?,?,?) ON CONFLICT (party_id, user_id) DO UPDATE SET first_item = excluded.first_item,
		second_item = excluded.second_item, third_item = excluded.third_item, veto_item = excluded.veto_item,
		updated_at = excluded.updated_at`, partyID, b.UserID, rank(0), rank(1), rank(2), nullStr(b.Veto), ts(at))
	return err
}

// Ballots returns every ballot, for the tally only (R32).
func (s *Store) Ballots(ctx context.Context, partyID string) ([]Ballot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, first_item, second_item, third_item, veto_item
		FROM party_ballots WHERE party_id = ? ORDER BY user_id`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ballot
	for rows.Next() {
		var b Ballot
		var r [3]sql.NullString
		var veto sql.NullString
		if err := rows.Scan(&b.UserID, &r[0], &r[1], &r[2], &veto); err != nil {
			return nil, err
		}
		for _, x := range r {
			if x.Valid {
				b.Ranks = append(b.Ranks, x.String)
			}
		}
		b.Veto = veto.String
		out = append(out, b)
	}
	return out, rows.Err()
}

// SetSeed stores the tie-draw seed before it's used (R33).
func (s *Store) SetSeed(ctx context.Context, partyID, seed string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE party_votes SET seed = ? WHERE party_id = ? AND seed IS NULL`, seed, partyID)
	return err
}

// CloseVote records the winner.
func (s *Store) CloseVote(ctx context.Context, partyID, winner string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE party_votes SET closed_at = ?, winner_item_id = ? WHERE party_id = ?`,
		ts(at), nullStr(winner), partyID)
	return err
}

// Score is an item's result.
type Score struct {
	ItemID string `json:"itemId"`
	Points int    `json:"points"`
	Vetoed bool   `json:"vetoed"`
}

// Result is a tally.
type Result struct {
	Scores []Score `json:"scores"` // best first, vetoed last
	// Tied are the items sharing the top score before the draw.
	Tied   []string `json:"tied,omitempty"`
	Winner string   `json:"winner,omitempty"`
	Seed   string   `json:"seed,omitempty"`
}

// Tally counts ballots: 3, 2, 1 points, vetoed items out (R32); a tie at
// the top is drawn from seed (R33). An empty winner means nothing is left.
func Tally(proposals []Proposal, ballots []Ballot, seed string) Result {
	points := map[string]int{}
	vetoed := map[string]bool{}
	for _, p := range proposals {
		points[p.ItemID] = 0
	}
	for _, b := range ballots {
		for i, id := range b.Ranks {
			if _, ok := points[id]; ok && i < 3 {
				points[id] += 3 - i
			}
		}
		if _, ok := points[b.Veto]; ok {
			vetoed[b.Veto] = true
		}
	}
	var res Result
	for _, p := range proposals {
		res.Scores = append(res.Scores, Score{ItemID: p.ItemID, Points: points[p.ItemID], Vetoed: vetoed[p.ItemID]})
	}
	sort.SliceStable(res.Scores, func(i, j int) bool {
		a, b := res.Scores[i], res.Scores[j]
		if a.Vetoed != b.Vetoed {
			return !a.Vetoed
		}
		return a.Points > b.Points
	})
	if len(res.Scores) == 0 || res.Scores[0].Vetoed || len(ballots) == 0 {
		return res
	}
	top := res.Scores[0].Points
	for _, sc := range res.Scores {
		if !sc.Vetoed && sc.Points == top {
			res.Tied = append(res.Tied, sc.ItemID)
		}
	}
	if len(res.Tied) == 1 {
		res.Winner, res.Tied = res.Tied[0], nil
		return res
	}
	res.Winner, res.Seed = Draw(res.Tied, seed), seed
	return res
}

// Draw picks one of ids from seed, the same way every time.
func Draw(ids []string, seed string) string {
	h := fnv.New64a()
	h.Write([]byte(seed))
	r := rand.New(rand.NewPCG(h.Sum64(), uint64(len(ids))))
	return ids[r.IntN(len(ids))]
}

// NewSeed returns a random seed.
func NewSeed() string {
	b := make([]byte, 16)
	crand.Read(b)
	return hex.EncodeToString(b)
}
