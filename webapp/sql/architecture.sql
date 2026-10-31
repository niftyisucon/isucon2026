-- Idempotent migration; no initial image export or rewrite is required.
ALTER TABLE image_objects ADD COLUMN IF NOT EXISTS storage_key text;
CREATE OR REPLACE FUNCTION isureal_state_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM pg_notify('isureal_state','changed');
 RETURN NULL;
END;
$$;
CREATE OR REPLACE TRIGGER isureal_notification_changed AFTER INSERT OR UPDATE OR DELETE ON notifications
 FOR EACH STATEMENT EXECUTE FUNCTION isureal_state_changed();
CREATE OR REPLACE TRIGGER isureal_window_changed AFTER UPDATE ON posting_windows
 FOR EACH STATEMENT EXECUTE FUNCTION isureal_state_changed();
CREATE OR REPLACE TRIGGER isureal_user_changed AFTER UPDATE OF region,status OR DELETE ON users
 FOR EACH STATEMENT EXECUTE FUNCTION isureal_state_changed();
CREATE TABLE IF NOT EXISTS isureal_runtime_state (
 id boolean PRIMARY KEY DEFAULT true CHECK (id),
 generation uuid NOT NULL
);
INSERT INTO isureal_runtime_state(id,generation) VALUES(true,gen_random_uuid())
 ON CONFLICT(id) DO UPDATE SET generation=EXCLUDED.generation;
-- reset.sql suppresses row triggers while restoring data. Notify explicitly;
-- subscribers from before initialize must never receive restored/old results.
SELECT pg_notify('isureal_state','reset');
