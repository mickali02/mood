-- File: migrations/000004_add_user_id_to_moods.up.sql
ALTER TABLE moods
ADD COLUMN user_id BIGINT NOT NULL; -- Add the column, initially disallow NULLs

-- Add the foreign key constraint AFTER adding the column
ALTER TABLE moods
ADD CONSTRAINT moods_user_id_fk
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS moods_user_id_idx ON moods (user_id);