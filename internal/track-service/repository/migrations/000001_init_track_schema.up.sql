-- 1. TRACKS STORAGE TABLE
CREATE TABLE "tracks" (
    "id" VARCHAR PRIMARY KEY,
    "title" VARCHAR NOT NULL,
    "artist_id" VARCHAR NOT NULL, -- Links to your upcoming artist/creator system
    "artist_name" VARCHAR NOT NULL,
    "duration" VARCHAR NOT NULL, -- Format "MM:SS"
    "thumbnail_url" VARCHAR NOT NULL,
    "youtube_id" VARCHAR UNIQUE NOT NULL,
    "play_count" INTEGER DEFAULT 0 NOT NULL, -- Incremented via telemetry events
    "likes_count" INTEGER DEFAULT 0 NOT NULL,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT (now()) NOT NULL
);

-- 2. RECENTLY PLAYED / TELEMETRY HISTORY TABLE
CREATE TABLE "listening_history" (
    "id" BIGSERIAL PRIMARY KEY,
    "user_id" UUID NOT NULL,
    "track_id" VARCHAR REFERENCES "tracks" ("id") ON DELETE CASCADE NOT NULL,
    "last_position_sec" INTEGER DEFAULT 0 NOT NULL,
    "is_completed" BOOLEAN DEFAULT false NOT NULL,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT (now()) NOT NULL,
    CONSTRAINT "user_track_history_unique" UNIQUE ("user_id", "track_id")
);

-- 3. LIKES & DISLIKES SYSTEM (Two-way sentiment matrix)
CREATE TABLE "track_interactions" (
    "user_id" UUID NOT NULL,
    "track_id" VARCHAR REFERENCES "tracks" ("id") ON DELETE CASCADE NOT NULL,
    "is_liked" BOOLEAN NOT NULL, -- TRUE for Like, FALSE for Dislike
    "interacted_at" TIMESTAMP WITH TIME ZONE DEFAULT (now()) NOT NULL,
    PRIMARY KEY ("user_id", "track_id")
);

-- 4. PLAYLIST CURATION ENGINE
CREATE TABLE "playlists" (
    "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id" UUID NOT NULL,
    "name" VARCHAR NOT NULL,
    "cover_image_url" VARCHAR,
    "is_private" BOOLEAN DEFAULT false NOT NULL,
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT (now()) NOT NULL
);

-- 5. PLAYLIST & TRACK LINK TABLE (Many-to-Many Layout)
CREATE TABLE "playlist_tracks" (
    "playlist_id" UUID REFERENCES "playlists" ("id") ON DELETE CASCADE NOT NULL,
    "track_id" VARCHAR REFERENCES "tracks" ("id") ON DELETE CASCADE NOT NULL,
    "added_at" TIMESTAMP WITH TIME ZONE DEFAULT (now()) NOT NULL,
    PRIMARY KEY ("playlist_id", "track_id")
);

-- --- PRODUCTION SPEEDED INDEXES ---
CREATE INDEX ON "tracks" ("title");
CREATE INDEX ON "tracks" ("play_count" DESC); -- Powers your trending dashboard instantly!
CREATE INDEX ON "listening_history" ("user_id", "updated_at" DESC);