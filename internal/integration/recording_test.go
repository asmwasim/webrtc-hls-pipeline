//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
)

func TestRecording_Found(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Recorded"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	sessionID, _ := uuid.Parse(created["id"].(string))

	recID := uuid.New()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO recordings (id, session_id, tenant_id, status, mp4_url, created_at, completed_at)
		 VALUES ($1, $2, $3, 'ready', '/recordings/test/recording.mp4', $4, $4)`,
		recID, sessionID, tenantA, time.Now().UTC(),
	)
	require.NoError(t, err)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/recording", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Equal(t, "ready", resp["status"])
	assert.Equal(t, "/recordings/test/recording.mp4", resp["mp4_url"])
}

func TestRecording_NotFound(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "No Recording"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id+"/recording", nil, token)
	w = serve(req)

	assert.Equal(t, 404, w.Code)
}

func TestRecording_ProcessingStatus(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Processing"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	sessionID, _ := uuid.Parse(created["id"].(string))

	recID := uuid.New()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO recordings (id, session_id, tenant_id, status, created_at)
		 VALUES ($1, $2, $3, 'processing', $4)`,
		recID, sessionID, tenantA, time.Now().UTC(),
	)
	require.NoError(t, err)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/recording", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Equal(t, "processing", resp["status"])
}
