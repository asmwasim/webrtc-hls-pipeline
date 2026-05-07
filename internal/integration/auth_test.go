//go:build integration

package integration

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
)

func TestAuth_MissingToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	w := serve(req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization token")
}

func TestAuth_InvalidToken(t *testing.T) {
	req := testutil.AuthRequest("GET", "/api/v1/sessions", nil, "garbage-token")
	w := serve(req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid token")
}

func TestAuth_ExpiredToken(t *testing.T) {
	token := tokens.ExpiredToken()
	req := testutil.AuthRequest("GET", "/api/v1/sessions", nil, token)
	w := serve(req)

	assert.Equal(t, 401, w.Code)
}

func TestAuth_WrongSigningKey(t *testing.T) {
	token := tokens.TokenWithSecret("wrong-secret", tenantA, teacherID)
	req := testutil.AuthRequest("GET", "/api/v1/sessions", nil, token)
	w := serve(req)

	assert.Equal(t, 401, w.Code)
}

func TestAuth_TokenInQueryParam(t *testing.T) {
	clean(t)
	token := tokens.TeacherToken(tenantA, teacherID)
	req := httptest.NewRequest("GET", "/api/v1/sessions?token="+token, nil)
	w := serve(req)

	assert.Equal(t, 200, w.Code)
}
