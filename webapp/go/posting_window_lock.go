package isureal

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// These transaction locks replace the users table lock. Window starts take an
// exclusive lock for their region; changes to the notification recipient set
// take shared locks BEFORE any user/window row locks. Unrelated user updates
// (login, profile, relationships, posting) need no membership lock.
// Keys are disjoint from the search migration's (20261031, 35).
func lockPostingRegion(ctx context.Context, tx *sqlx.Tx, region string, shared bool) error {
	key := 0
	switch region {
	case "asia":
		key = 7201
	case "europe":
		key = 7202
	case "america":
		key = 7203
	default:
		return fmt.Errorf("invalid posting region %q", region)
	}
	query := `SELECT pg_advisory_xact_lock(20261031, $1)`
	if shared {
		query = `SELECT pg_advisory_xact_lock_shared(20261031, $1)`
	}
	_, err := tx.ExecContext(ctx, query, key)
	return err
}

// A move/delete cannot safely trust a pre-lock read of the user's region. Take
// all three shared locks in a fixed order before touching that user instead.
// Membership writers remain mutually compatible; no read/retry loop is needed.
func lockAllPostingMemberships(ctx context.Context, tx *sqlx.Tx) error {
	for _, region := range []string{"asia", "europe", "america"} {
		if err := lockPostingRegion(ctx, tx, region, true); err != nil {
			return err
		}
	}
	return nil
}
