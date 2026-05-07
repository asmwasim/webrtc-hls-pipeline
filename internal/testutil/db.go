package testutil

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CleanDB(ctx context.Context, pool *pgxpool.Pool) {
	pool.Exec(ctx, "TRUNCATE recordings, chat_messages, sessions CASCADE")
}
