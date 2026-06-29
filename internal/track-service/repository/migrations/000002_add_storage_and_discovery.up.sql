-- Add storage and discovery columns to tracks
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS storage_url VARCHAR;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS genre VARCHAR DEFAULT 'phonk';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS is_featured BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS is_approved BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS is_rejected BOOLEAN DEFAULT false NOT NULL;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS source VARCHAR DEFAULT 'auto' NOT NULL;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS yt_view_count BIGINT DEFAULT 0;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS fcm_notified BOOLEAN DEFAULT false NOT NULL;

-- Full text search index for SearchTracks
CREATE INDEX IF NOT EXISTS idx_tracks_title_fts 
  ON tracks USING GIN (to_tsvector('english', title || ' ' || artist_name));

-- Genre filter index
CREATE INDEX IF NOT EXISTS idx_tracks_genre ON tracks (genre);

-- Approval status index for admin dashboard
CREATE INDEX IF NOT EXISTS idx_tracks_approval ON tracks (is_approved, is_rejected);