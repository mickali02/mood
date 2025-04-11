-- migrations/000001_create_moods_table.down.sql
DROP TRIGGER IF EXISTS update_moods_updated_at ON moods;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP INDEX IF EXISTS idx_moods_created_at;
DROP TABLE IF EXISTS moods;