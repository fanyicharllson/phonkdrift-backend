-- 000002_add_profile_to_members.up.sql

ALTER TABLE community_members ADD COLUMN IF NOT EXISTS username VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE community_members ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT '';
