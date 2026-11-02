package party

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The scheduler runs parties that wait for a time (R29, R33, R38): it
// opens the lobby and reminds invitees 15 minutes before, starts the title
// at the time, cancels a party nobody started 3 hours later, and closes
// votes at their deadline. It works from the stored times, so ticks missed
// while the server was down run once at the next one.

const (
	reminderLead = 15 * time.Minute
	staleAfter   = 3 * time.Hour
)

// schedule runs one pass.
func (s *Service) schedule(ctx context.Context) {
	parties, err := s.store.Active(ctx)
	if err != nil {
		s.o.Log.Error("party scheduler", "err", err)
		return
	}
	now := s.o.Now()
	for _, pt := range parties {
		if err := s.scheduleOne(ctx, pt, now); err != nil {
			s.o.Log.Error("party scheduler", "party_id", pt.ID, "err", err)
		}
	}
	if err := s.store.PurgeSummaries(ctx, now.Add(-30*24*time.Hour)); err != nil {
		s.o.Log.Error("party scheduler: purge", "err", err)
	}
}

func (s *Service) scheduleOne(ctx context.Context, pt Party, now time.Time) error {
	if pt.Status == StatusVoting {
		v, err := s.store.Vote(ctx, pt.ID)
		if err != nil {
			return err
		}
		if !now.Before(v.RankingUntil) {
			if err := s.closeVote(ctx, pt); err != nil {
				return err
			}
			if pt, err = s.store.Get(ctx, pt.ID); err != nil {
				return err
			}
		}
	}
	if pt.ScheduledAt == nil || pt.Status == StatusLive {
		return nil
	}
	at := *pt.ScheduledAt
	if pt.StartedAt == nil && !now.Before(at.Add(staleAfter)) {
		pt.Status, pt.EndedAt = StatusCancelled, &now
		if err := s.store.Update(ctx, pt); err != nil {
			return err
		}
		if l := s.roomOf(pt.ID); l != nil {
			l.mu.Lock()
			l.room.end()
			l.mu.Unlock()
			s.mu.Lock()
			delete(s.rooms, pt.ID)
			s.mu.Unlock()
		}
		s.o.Log.Info("party cancelled: nobody started it", "party_id", pt.ID)
		return nil
	}
	if now.Before(at.Add(-reminderLead)) {
		return nil
	}
	// A party planned on an episode before its file: resolve it now (R38).
	if pt.ItemID == "" && pt.FutureShowID != "" {
		id, err := s.store.EpisodeOf(ctx, pt.FutureShowID, pt.FutureSeason, pt.FutureEpisode)
		if err != nil {
			return err
		}
		if id == "" {
			s.notifyInvites(pt, []string{pt.OrganizerUserID}, "party.file.missing")
		} else {
			pt.ItemID = id
		}
	}
	if pt.Status == StatusScheduled {
		pt.Status = StatusLobby
		if err := s.store.Update(ctx, pt); err != nil {
			return err
		}
		if err := s.remind(ctx, pt, now); err != nil {
			return err
		}
	}
	if pt.ItemID == "" || pt.Status == StatusVoting {
		return nil
	}
	l, err := s.openRoom(ctx, pt)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.party = pt
	l.room.ItemID = pt.ItemID
	if !now.Before(at) && l.startBy.IsZero() && pt.StartedAt == nil {
		// At the time, the title starts for the screens that are ready;
		// the others join in sync when they arrive (R29).
		l.startBy = now
		pt.Status, pt.StartedAt = StatusLive, &now
		l.party = pt
	}
	l.mu.Unlock()
	if pt.Status == StatusLive {
		if err := s.store.Update(ctx, pt); err != nil {
			return err
		}
		s.o.Log.Info("party started at its time", "party_id", pt.ID)
	}
	return nil
}

// remind tells invitees who said yes or maybe, and the organizer, that the
// party opens (R29). iOS also schedules its own local reminder (R41).
func (s *Service) remind(ctx context.Context, pt Party, now time.Time) error {
	invites, err := s.store.Invites(ctx, pt.ID)
	if err != nil {
		return err
	}
	users := []string{pt.OrganizerUserID}
	for _, in := range invites {
		if in.RemindedAt == nil && (in.Answer == AnswerYes || in.Answer == AnswerMaybe) {
			users = append(users, in.UserID)
		}
	}
	if err := s.store.MarkReminded(ctx, pt.ID, now); err != nil {
		return err
	}
	s.notifyInvites(pt, users, "party.reminder")
	return nil
}

// StartNow starts a lobby before its time ("Start without waiting", R29).
func (s *Service) startNow(ctx context.Context, l *live, now time.Time) {
	l.startBy = now
	if l.party.StartedAt == nil {
		l.party.Status, l.party.StartedAt = StatusLive, &now
		if err := s.store.Update(context.WithoutCancel(ctx), l.party); err != nil {
			s.o.Log.Error("party: storing the start", "party_id", l.party.ID, "err", err)
		}
	}
}

// EpisodeOf returns the item of a show's episode, "" until it's scanned.
func (st *Store) EpisodeOf(ctx context.Context, showID string, season, episode int) (string, error) {
	var id string
	err := st.db.QueryRowContext(ctx, `SELECT e.id FROM items e JOIN items se ON se.id = e.parent_id
		WHERE se.parent_id = ? AND se.season_number = ? AND e.episode_number = ? AND e.hidden = 0`,
		showID, season, episode).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
