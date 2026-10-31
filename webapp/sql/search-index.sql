-- Run once, outside a benchmark, after #76 prepared the search columns.
-- Application queries remain correct without this optional acceleration migration.
-- Retry after a timeout: the whole migration is transactional and idempotent.
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='30s';
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;
CREATE INDEX IF NOT EXISTS users_search_trgm ON users
 USING gin(search_username public.gin_trgm_ops, search_display_name public.gin_trgm_ops)
 WITH (fastupdate=off)
 WHERE status='active' AND search_username IS NOT NULL AND search_display_name IS NOT NULL;
CREATE INDEX IF NOT EXISTS users_search_legacy ON users(username,id)
 WHERE status='active' AND (search_username IS NULL OR search_display_name IS NULL);
ANALYZE users;
COMMIT;
