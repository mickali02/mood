-- File: migrations/000004__add_user_id_to_moods.down.sql
ALTER TABLE moods
DROP CONSTRAINT IF EXISTS moods_user_id_fk;

DROP INDEX IF EXISTS moods_user_id_idx;

ALTER TABLE moods
DROP COLUMN IF EXISTS user_id;