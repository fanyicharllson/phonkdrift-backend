-- 000002_add_profile_to_members.down.sql

ALTER TABLE community_members DROP COLUMN IF EXISTS username;
ALTER TABLE community_members DROP COLUMN IF EXISTS avatar_url;
