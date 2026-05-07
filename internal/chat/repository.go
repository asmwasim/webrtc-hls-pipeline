package chat

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, msg *Message) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO chat_messages (id, session_id, tenant_id, user_id, username, message, type, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		msg.ID, msg.SessionID, msg.TenantID, msg.UserID, msg.Username, msg.Content, msg.Type, msg.CreatedAt,
	)
	return err
}

func (r *Repository) GetHistory(ctx context.Context, sessionID uuid.UUID, before time.Time, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, session_id, tenant_id, user_id, username, message, type, created_at
		 FROM chat_messages
		 WHERE session_id = $1 AND created_at < $2
		 ORDER BY created_at DESC
		 LIMIT $3`,
		sessionID, before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.TenantID, &m.UserID, &m.Username, &m.Content, &m.Type, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
