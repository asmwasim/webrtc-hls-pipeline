//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
)

func TestSession_Create(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)
	body := testutil.JSONBody(map[string]string{"title": "Test Lecture"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)

	assert.Equal(t, 201, w.Code)

	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Equal(t, "Test Lecture", resp["title"])
	assert.Equal(t, "waiting", resp["status"])
	assert.Equal(t, tenantA.String(), resp["tenant_id"])
	assert.Equal(t, teacherID.String(), resp["teacher_id"])

	_, err := uuid.Parse(resp["id"].(string))
	require.NoError(t, err)
}

func TestSession_Create_MissingTitle(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)
	body := testutil.JSONBody(map[string]string{})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "title")
}

func TestSession_Create_StudentForbidden(t *testing.T) {
	clean(t)
	token := tokens.StudentToken(tenantA, studentID)
	body := testutil.JSONBody(map[string]string{"title": "Forbidden"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)

	assert.Equal(t, 403, w.Code)
}

func TestSession_Get(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "Get Me"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id, nil, token)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]any
	decodeJSON(t, w, &resp)
	assert.Equal(t, "Get Me", resp["title"])
	assert.Equal(t, id, resp["id"])
}

func TestSession_Get_NotFound(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)
	req := testutil.AuthRequest("GET", "/api/v1/sessions/"+uuid.New().String(), nil, token)
	w := serve(req)

	assert.Equal(t, 404, w.Code)
}

func TestSession_List(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	for _, title := range []string{"First", "Second", "Third"} {
		body := testutil.JSONBody(map[string]string{"title": title})
		req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
		w := serve(req)
		require.Equal(t, 201, w.Code)
	}

	req := testutil.AuthRequest("GET", "/api/v1/sessions", nil, token)
	w := serve(req)

	assert.Equal(t, 200, w.Code)
	var sessions []map[string]any
	decodeJSON(t, w, &sessions)
	assert.Len(t, sessions, 3)
	assert.Equal(t, "Third", sessions[0]["title"])
}

func TestSession_List_TenantIsolation(t *testing.T) {
	clean(t)

	tokenA := tokens.TeacherToken(tenantA, teacherID)
	body := testutil.JSONBody(map[string]string{"title": "Tenant A Session"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, tokenA)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	tokenB := tokens.TeacherToken(tenantB, uuid.New())
	req = testutil.AuthRequest("GET", "/api/v1/sessions", nil, tokenB)
	w = serve(req)

	assert.Equal(t, 200, w.Code)
	var sessions []map[string]any
	decodeJSON(t, w, &sessions)
	assert.Empty(t, sessions)
}

func TestSession_End(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "End Me"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, token)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	req = testutil.AuthRequest("POST", "/api/v1/sessions/"+id+"/end", nil, token)
	w = serve(req)
	assert.Equal(t, 200, w.Code)

	req = testutil.AuthRequest("GET", "/api/v1/sessions/"+id, nil, token)
	w = serve(req)
	var got map[string]any
	decodeJSON(t, w, &got)
	assert.Equal(t, "ended", got["status"])
}

func TestSession_End_StudentForbidden(t *testing.T) {
	clean(t)
	teacherToken := tokens.TeacherToken(tenantA, teacherID)

	body := testutil.JSONBody(map[string]string{"title": "No End"})
	req := testutil.AuthRequest("POST", "/api/v1/sessions", body, teacherToken)
	w := serve(req)
	require.Equal(t, 201, w.Code)

	var created map[string]any
	decodeJSON(t, w, &created)
	id := created["id"].(string)

	studentToken := tokens.StudentToken(tenantA, studentID)
	req = testutil.AuthRequest("POST", "/api/v1/sessions/"+id+"/end", nil, studentToken)
	w = serve(req)
	assert.Equal(t, 403, w.Code)
}
