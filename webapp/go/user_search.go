package isureal

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"

	"github.com/jmoiron/sqlx"
)

type searchCandidate struct {
	ID          string `db:"id" json:"id"`
	Username    string `db:"username" json:"username"`
	DisplayName string `db:"display_name" json:"display_name"`
}

// Limit only known matches. Unprepared legacy rows must not consume the 51
// slots: an arbitrary number of those rows may fail the Go predicate. The union
// is disjoint; applying the same ordering after it preserves cursor semantics.
// Inline the two branches so the prepared branch can use substring indexes and
// the legacy branch can use its small partial index instead of copying all users.
const searchCandidatesPrefix = `WITH candidates AS NOT MATERIALIZED (
 SELECT id, username, display_name, search_username, search_display_name FROM users
 WHERE status='active' AND id<>$1 AND ($2='' OR (username,id)>($2,$3))
 AND id NOT IN (SELECT CASE WHEN blocker_id=$1 THEN blocked_id ELSE blocker_id END
   FROM block_relations WHERE blocker_id=$1 OR blocked_id=$1)
), matches AS (
 (SELECT id, username, display_name FROM candidates
  WHERE search_username IS NOT NULL AND search_display_name IS NOT NULL
    AND (`

const searchCandidatesSuffix = `)
  ORDER BY username,id LIMIT 51)
 UNION ALL
 SELECT id, username, display_name FROM candidates
 WHERE search_username IS NULL OR search_display_name IS NULL
)
SELECT id, username, display_name FROM matches ORDER BY username,id`

const searchCandidatesSQL = searchCandidatesPrefix + `search_username LIKE $4 ESCAPE E'\\' OR search_display_name LIKE $4 ESCAPE E'\\'` + searchCandidatesSuffix
const searchCandidatesScanSQL = searchCandidatesPrefix + `strpos(search_username,$4)>0 OR strpos(search_display_name,$4)>0` + searchCandidatesSuffix

var searchPatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// The input is already NFC/case-folded. LIKE is only an indexable spelling of
// literal substring search; user-supplied wildcard/escape characters stay literal.
func searchSubstringPattern(query string) string {
	return "%" + searchPatternEscaper.Replace(query) + "%"
}

func searchCandidatesQuery(query string) (string, string) {
	// Unanchored patterns without a word trigram cannot narrow a GIN scan.
	// Keep literal strpos for short/symbol-only queries; all queries are supported.
	wordRun := 0
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			wordRun++
			if wordRun >= 3 {
				return searchCandidatesSQL, searchSubstringPattern(query)
			}
		} else {
			wordRun = 0
		}
	}
	return searchCandidatesScanSQL, query
}

// Called before serving requests and after reset. Existing rows are prepared in
// batches; concurrent profile edits cannot be overwritten by an older snapshot.
// NULL rows stay searchable during a rolling deployment via the Go fallback.
func prepareUserSearch(ctx context.Context, db *sqlx.DB) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize catalog changes across simultaneous region process starts.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(20261031,35)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE users
 ADD COLUMN IF NOT EXISTS search_username text,
 ADD COLUMN IF NOT EXISTS search_display_name text`); err != nil {
		return err
	}
	// Old binaries can still update names during a rollout or rollback. Clear
	// only keys they did not also update, so those rows use the exact fallback.
	if _, err := tx.ExecContext(ctx, `CREATE OR REPLACE FUNCTION invalidate_user_search() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN
  IF NEW.username IS DISTINCT FROM OLD.username AND NEW.search_username IS NOT DISTINCT FROM OLD.search_username THEN
   NEW.search_username := NULL;
  END IF;
  IF NEW.display_name IS DISTINCT FROM OLD.display_name AND NEW.search_display_name IS NOT DISTINCT FROM OLD.search_display_name THEN
   NEW.search_display_name := NULL;
  END IF;
  RETURN NEW;
 END $$;
 CREATE OR REPLACE TRIGGER invalidate_user_search BEFORE UPDATE OF username,display_name ON users
 FOR EACH ROW EXECUTE FUNCTION invalidate_user_search()`); err != nil {
		return err
	}
	var rows []searchCandidate
	// Recompute all source names, including after a previous binary's reset
	// (which disables triggers). Only changed keys are written back.
	if err := tx.SelectContext(ctx, &rows, `SELECT id,username,display_name FROM users`); err != nil {
		return err
	}
	type prepared struct {
		searchCandidate
		SearchUsername string `json:"search_username"`
		SearchDisplay  string `json:"search_display_name"`
	}
	for start := 0; start < len(rows); start += 256 {
		batch := make([]prepared, 0, min(256, len(rows)-start))
		for _, row := range rows[start:min(start+256, len(rows))] {
			batch = append(batch, prepared{row, normalizeSearchText(row.Username), normalizeSearchText(row.DisplayName)})
		}
		data, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE users u
 SET search_username=v.search_username, search_display_name=v.search_display_name
 FROM jsonb_to_recordset($1::jsonb) AS v(id text, username text, display_name text, search_username text, search_display_name text)
 WHERE u.id=v.id AND u.username=v.username AND u.display_name=v.display_name
 AND (u.search_username IS DISTINCT FROM v.search_username OR u.search_display_name IS DISTINCT FROM v.search_display_name)`, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
