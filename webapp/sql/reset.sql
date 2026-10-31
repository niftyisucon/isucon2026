SET session_replication_role = replica;

BEGIN;

TRUNCATE ip_bans, login_sessions, administrator_sessions, friendships, friend_requests, follows, block_relations;

DELETE FROM notifications WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DELETE FROM comments WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DELETE FROM realmojis WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DELETE FROM posts WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DELETE FROM posting_windows WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DO $$
DECLARE partition_name text;
BEGIN
  FOR partition_name IN
    SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
    WHERE i.inhparent = 'image_objects'::regclass
      AND c.relname > 'image_objects_' || to_char((SELECT created_at FROM initial_data_boundary) AT TIME ZONE 'UTC', 'YYYYMM')
  LOOP
    EXECUTE format('TRUNCATE %I', partition_name);
  END LOOP;
END $$;
DELETE FROM image_objects WHERE created_at > (SELECT created_at FROM initial_data_boundary);
DELETE FROM users WHERE created_at > (SELECT created_at FROM initial_data_boundary);

UPDATE users u
SET display_name = i.display_name, bio = i.bio, profile_image_id = i.profile_image_id, region = i.region,
    account_type = i.account_type, status = i.status, deleted_at = i.deleted_at, updated_at = i.updated_at
FROM initial_users i
WHERE u.id = i.id
  AND (u.display_name, u.bio, u.profile_image_id, u.region, u.account_type, u.status, u.deleted_at, u.updated_at)
      IS DISTINCT FROM (i.display_name, i.bio, i.profile_image_id, i.region, i.account_type, i.status, i.deleted_at, i.updated_at);

INSERT INTO friend_requests (id, requester_id, recipient_id, status, created_at, updated_at)
SELECT id, requester_id, recipient_id, status, created_at, updated_at FROM initial_friend_requests;
INSERT INTO friendships (id, user_low_id, user_high_id, source_request_id, created_at, updated_at)
SELECT id, user_low_id, user_high_id, source_request_id, created_at, updated_at FROM initial_friendships;
INSERT INTO follows (id, follower_id, followee_id, created_at, updated_at)
SELECT id, follower_id, followee_id, created_at, updated_at FROM initial_follows;
INSERT INTO block_relations (id, blocker_id, blocked_id, created_at)
SELECT id, blocker_id, blocked_id, created_at FROM initial_block_relations;

COMMIT;

-- 競技中に追加したインデックス。既存環境にも初期化時に作成する（2回目以降は何もしない）。
CREATE INDEX IF NOT EXISTS realmojis_image_id ON realmojis(image_id);
CREATE INDEX IF NOT EXISTS posts_image_id ON posts(image_id);
CREATE INDEX IF NOT EXISTS friendships_user_high_id ON friendships(user_high_id);

-- 画像 id から created_at を引く対応表（Issue #44）。image_objects は created_at で分割されているため、
-- 画像取得で created_at を与えて実行時に1パーティションへ絞る。初期データ分は初回だけ作成する。
-- 183万行をランダム順で B-tree に入れると遅いので、索引なしで一括投入してから一意索引を作る。
CREATE TABLE IF NOT EXISTS image_locations (
  id varchar(36) NOT NULL,
  created_at timestamptz NOT NULL
);
INSERT INTO image_locations (id, created_at)
SELECT id, created_at FROM image_objects WHERE NOT EXISTS (SELECT 1 FROM image_locations LIMIT 1);
CREATE UNIQUE INDEX IF NOT EXISTS image_locations_id ON image_locations(id);
CREATE OR REPLACE FUNCTION image_locations_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO image_locations (id, created_at) VALUES (NEW.id, NEW.created_at) ON CONFLICT DO NOTHING;
  RETURN NULL;
END;
$$;
CREATE OR REPLACE TRIGGER image_locations_insert AFTER INSERT ON image_objects
FOR EACH ROW EXECUTE FUNCTION image_locations_insert();
