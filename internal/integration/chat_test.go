//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/chat"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
)

func TestChatHistory_Empty(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Chat Session"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id+"/chat/history", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "[]\n", w.Body.String())
}

func TestChatHistory_WithMessages(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Chat Session"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	sessionID, _ := uuid.Parse(created["id"].(string))

	repo := chat.NewRepository(testPool)
	base := time.Now().UTC().Add(-10 * time.Second)
	for i := 0; i < 5; i++ {
		msg := &chat.Message{
			ID:        uuid.New(),
			SessionID: sessionID,
			TenantID:  tenantA,
			UserID:    teacherID,
			Username:  "Teacher",
			Content:   "msg-" + string(rune('A'+i)),
			Type:      "message",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repo.Insert(context.Background(), msg))
	}

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/chat/history", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var messages []map[string]any
	decodeJSON(t, w, &messages)
	assert.Len(t, messages, 5)
	assert.Equal(t, "msg-A", messages[0]["message"])
	assert.Equal(t, "msg-E", messages[4]["message"])
}

func TestChatHistory_Pagination(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Paginated"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	sessionID, _ := uuid.Parse(created["id"].(string))

	repo := chat.NewRepository(testPool)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		msg := &chat.Message{
			ID:        uuid.New(),
			SessionID: sessionID,
			TenantID:  tenantA,
			UserID:    teacherID,
			Username:  "Teacher",
			Content:   "msg-" + string(rune('A'+i)),
			Type:      "message",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repo.Insert(context.Background(), msg))
	}

	before := base.Add(7 * time.Second).Format(time.RFC3339)
	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/chat/history?limit=3&before="+before, nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var messages []map[string]any
	decodeJSON(t, w, &messages)
	assert.Len(t, messages, 3)
	assert.Equal(t, "msg-E", messages[0]["message"])
	assert.Equal(t, "msg-G", messages[2]["message"])
}

func TestChatHistory_LimitClamp(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Clamp"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	sessionID, _ := uuid.Parse(created["id"].(string))

	repo := chat.NewRepository(testPool)
	base := time.Now().UTC().Add(-2 * time.Minute)
	for i := 0; i < 60; i++ {
		msg := &chat.Message{
			ID:        uuid.New(),
			SessionID: sessionID,
			TenantID:  tenantA,
			UserID:    teacherID,
			Username:  "Teacher",
			Content:   "msg",
			Type:      "message",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repo.Insert(context.Background(), msg))
	}

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/chat/history?limit=200", nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var messages []map[string]any
	decodeJSON(t, w, &messages)
	assert.Equal(t, 50, len(messages))
}
