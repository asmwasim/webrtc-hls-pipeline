CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE sessions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id        UUID NOT NULL,
    teacher_id       UUID NOT NULL,
    title            TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'live', 'ended')),
    started_at       TIMESTAMPTZ,
    ended_at         TIMESTAMPTZ,
    hls_playlist_url TEXT DEFAULT '',
    mp4_url          TEXT DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_tenant_id ON sessions (tenant_id);
CREATE INDEX idx_sessions_status ON sessions (status);
CREATE INDEX idx_sessions_teacher_id ON sessions (teacher_id);
