CREATE TABLE chat_messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tenant_id  UUID NOT NULL,
    user_id    UUID NOT NULL,
    username   TEXT NOT NULL,
    message    TEXT NOT NULL,
    type       TEXT NOT NULL DEFAULT 'message' CHECK (type IN ('message', 'hand_raise', 'reaction')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session_id ON chat_messages (session_id);
CREATE INDEX idx_chat_messages_tenant_id ON chat_messages (tenant_id);
CREATE INDEX idx_chat_messages_created_at ON chat_messages (session_id, created_at);
