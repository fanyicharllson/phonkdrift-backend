-- 000001_init_chat_schema.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Single global community — user_id references auth-service's users table by
-- UUID only (no cross-database FK possible, same convention track-service
-- uses for track_interactions.user_id).
CREATE TABLE IF NOT EXISTS community_members (
    user_id UUID PRIMARY KEY,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- username/avatar_url are denormalized at write-time (via a lookup to
-- auth-service) so reading message history never needs a cross-service call.
CREATE TABLE IF NOT EXISTS chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    username VARCHAR(50) NOT NULL,
    avatar_url TEXT DEFAULT '',
    content TEXT,
    media_url TEXT,
    message_type VARCHAR(20) NOT NULL DEFAULT 'text' CHECK (message_type IN ('text', 'audio')),
    reply_to_id UUID REFERENCES chat_messages(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_reply_to_id ON chat_messages (reply_to_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages (user_id);
