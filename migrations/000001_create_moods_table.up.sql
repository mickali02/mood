-- migrations/000001_create_moods_table.up.sql
CREATE TABLE IF NOT EXISTS moods (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- To track edits
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    emotion TEXT NOT NULL  -- Stores the selected emotion (e.g., "Happy", "Sad")
);

-- Add an index for potentially faster sorting/lookup by creation time
CREATE INDEX IF NOT EXISTS idx_moods_created_at ON moods(created_at);

-- Function to automatically update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW(); -- Set updated_at to the current time on update
   RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to execute the function before any update operation on the moods table
CREATE TRIGGER update_moods_updated_at
BEFORE UPDATE ON moods
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

