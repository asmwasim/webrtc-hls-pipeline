CREATE TABLE recordings (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id   UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tenant_id    UUID NOT NULL,
    status       TEXT NOT NULL DEFAULT 'processing' CHECK (status IN ('processing', 'ready', 'failed')),
    mp4_url      TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_recordings_session_id ON recordings (session_id);
CREATE INDEX idx_recordings_tenant_id ON recordings (tenant_id);
