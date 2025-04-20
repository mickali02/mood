-- migrations/000002_add_emoji_color_to_moods.down.sql

ALTER TABLE moods
DROP COLUMN IF EXISTS emoji,
DROP COLUMN IF EXISTS color;