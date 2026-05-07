package testutil

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenFactory struct {
	secret []byte
}

func NewTokenFactory(secret string) *TokenFactory {
	return &TokenFactory{secret: []byte(secret)}
}

func (f *TokenFactory) TeacherToken(tenantID, userID uuid.UUID) string {
	return f.makeToken("teacher", "Teacher", tenantID, userID, time.Hour)
}

func (f *TokenFactory) StudentToken(tenantID, userID uuid.UUID) string {
	return f.makeToken("student", "Student", tenantID, userID, time.Hour)
}

func (f *TokenFactory) ExpiredToken() string {
	return f.makeToken("teacher", "Expired", uuid.New(), uuid.New(), -time.Hour)
}

func (f *TokenFactory) TokenWithSecret(secret string, tenantID, userID uuid.UUID) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"user_id":   userID.String(),
		"username":  "WrongKey",
		"role":      "teacher",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return token
}

func (f *TokenFactory) makeToken(role, username string, tenantID, userID uuid.UUID, ttl time.Duration) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"user_id":   userID.String(),
		"username":  username,
		"role":      role,
		"exp":       time.Now().Add(ttl).Unix(),
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(f.secret)
	return token
}
