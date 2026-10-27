package accounts

import (
	"context"
	"time"
)

// janitorEvery is how often expired rows and lockouts are purged.
const janitorEvery = time.Hour

// retainRevoked keeps revoked and expired sessions this long, so a device
// list or an investigation still sees them for a while.
const retainRevoked = 30 * 24 * time.Hour

// RunJanitor purges expired pairing codes, old sessions and forgotten
// lockouts every hour until ctx is done.
func (s *Service) RunJanitor(ctx context.Context) {
	t := time.NewTicker(janitorEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Purge(ctx); err != nil && ctx.Err() == nil {
				s.logger.Warn("accounts purge failed", "err", err)
			}
		}
	}
}

// Purge does one janitor pass.
func (s *Service) Purge(ctx context.Context) error {
	now := s.now()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pairing_codes WHERE expires_at < ?`,
		ts(now.Add(-pairingGrace))); err != nil {
		return err
	}
	cutoff := ts(now.Add(-retainRevoked))
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ? OR revoked_at < ?`,
		cutoff, cutoff); err != nil {
		return err
	}
	s.mu.Lock()
	for id, pt := range s.pairTokens {
		if now.After(pt.expires) {
			delete(s.pairTokens, id)
		}
	}
	s.mu.Unlock()
	for _, b := range []interface{ Sweep() }{s.logins, s.pins, s.claims, s.pairings} {
		b.Sweep()
	}
	return nil
}
