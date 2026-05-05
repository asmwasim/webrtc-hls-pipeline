package session

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	TeacherID      uuid.UUID  `json:"teacher_id"`
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	HLSPlaylistURL string    `json:"hls_playlist_url,omitempty"`
	MP4URL         string     `json:"mp4_url,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, tenantID, teacherID uuid.UUID, title string) (*Session, error) {
	s := &Session{
		ID:        uuid.New(),
		TenantID:  tenantID,
		TeacherID: teacherID,
		Title:     title,
		Status:    "waiting",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (id, tenant_id, teacher_id, title, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, s.TenantID, s.TeacherID, s.Title, s.Status, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	s := &Session{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, teacher_id, title, status, started_at, ended_at,
		        hls_playlist_url, mp4_url, created_at, updated_at
		 FROM sessions WHERE id = $1`, id,
	).Scan(&s.ID, &s.TenantID, &s.TeacherID, &s.Title, &s.Status,
		&s.StartedAt, &s.EndedAt, &s.HLSPlaylistURL, &s.MP4URL,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) List(ctx context.Context, tenantID uuid.UUID) ([]*Session, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, teacher_id, title, status, started_at, ended_at,
		        hls_playlist_url, mp4_url, created_at, updated_at
		 FROM sessions WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 50`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(&s.ID, &s.TenantID, &s.TeacherID, &s.Title, &s.Status,
			&s.StartedAt, &s.EndedAt, &s.HLSPlaylistURL, &s.MP4URL,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now().UTC()
	var query string

	switch status {
	case "live":
		query = `UPDATE sessions SET status = $2, started_at = $3, updated_at = $3 WHERE id = $1`
	case "ended":
		query = `UPDATE sessions SET status = $2, ended_at = $3, updated_at = $3 WHERE id = $1`
	default:
		query = `UPDATE sessions SET status = $2, updated_at = $3 WHERE id = $1`
	}

	_, err := r.pool.Exec(ctx, query, id, status, now)
	return err
}

func (r *Repository) SetHLSPlaylistURL(ctx context.Context, id uuid.UUID, url string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET hls_playlist_url = $2, updated_at = $3 WHERE id = $1`,
		id, url, time.Now().UTC(),
	)
	return err
}

func (r *Repository) SetMP4URL(ctx context.Context, id uuid.UUID, url string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessions SET mp4_url = $2, updated_at = $3 WHERE id = $1`,
		id, url, time.Now().UTC(),
	)
	return err
}
