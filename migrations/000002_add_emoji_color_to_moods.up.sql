-- migrations/000002_add_emoji_color_to_moods.up.sql

-- Add the new columns with default values
ALTER TABLE moods
ADD COLUMN emoji TEXT NOT NULL DEFAULT '❓', -- Default emoji if not provided
ADD COLUMN color VARCHAR(7) NOT NULL DEFAULT '#cccccc'; -- Default hex color (light grey)

UPDATE moods SET emoji = '😊', color = '#FFD700' WHERE emotion = 'Happy' AND emoji = '❓';
UPDATE moods SET emoji = '😢', color = '#6495ED' WHERE emotion = 'Sad' AND emoji = '❓';
UPDATE moods SET emoji = '😠', color = '#DC143C' WHERE emotion = 'Angry' AND emoji = '❓';
UPDATE moods SET emoji = '😟', color = '#FF8C00' WHERE emotion = 'Anxious' AND emoji = '❓';
UPDATE moods SET emoji = '😌', color = '#90EE90' WHERE emotion = 'Calm' AND emoji = '❓'; -- Adjusted Calm color slightly
UPDATE moods SET emoji = '🤩', color = '#FF69B4' WHERE emotion = 'Excited' AND emoji = '❓';
UPDATE moods SET emoji = '😐', color = '#B0C4DE' WHERE emotion = 'Neutral' AND emoji = '❓'; -- Adjusted Neutral color slightly

