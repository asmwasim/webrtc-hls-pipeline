//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
)

func TestHLSWatch_LiveSession(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Live"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	sessionRepo := session.NewRepository(testPool)
	sessionID, _ := uuid.Parse(id)
	require.NoError(t, sessionRepo.UpdateStatus(context.Background(), sessionID, "live"))

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id+"/watch", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Contains(t, resp["hls_url"], "/hls/"+id+"/master.m3u8")
}

func TestHLSWatch_EndedSession(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Ended"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)
	sessionID, _ := uuid.Parse(id)

	sessionRepo := session.NewRepository(testPool)
	require.NoError(t, sessionRepo.UpdateStatus(context.Background(), sessionID, "ended"))

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id+"/watch", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Contains(t, resp["hls_url"], "master.m3u8")
}

func TestHLSWatch_WaitingSession(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Waiting"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id+"/watch", nil, token)
	w = serve(req)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "not available")
}

func TestHLSWatch_NotFound(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)
	req := testutil.AuthRequest("GET", "/api/v1/sessions/"+uuid.New().String()+"/watch", nil, token)
	w := serve(req)

	assert.Equal(t, 404, w.Code)
}
