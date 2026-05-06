package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := flag.String("db", "postgres://postgres:postgres@localhost:5432/streaming?sslmode=disable", "Database URL")
	secret := flag.String("secret", "dev-secret-change-in-production", "JWT signing secret")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	tenantID := uuid.New()
	teacherID := uuid.New()
	studentID := uuid.New()

	sessionID := uuid.New()
	now := time.Now().UTC()

	_, err = pool.Exec(ctx,
		`INSERT INTO sessions (id, tenant_id, teacher_id, title, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT DO NOTHING`,
		sessionID, tenantID, teacherID, "Demo: Introduction to Go", "waiting", now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to insert session: %v\n", err)
		os.Exit(1)
	}

	teacherToken := mintToken(*secret, tenantID, teacherID, "Ms. Smith", "teacher")
	studentToken := mintToken(*secret, tenantID, studentID, "Alex", "student")

	fmt.Fprintln(os.Stderr, "=== Seed Data ===")
	fmt.Fprintf(os.Stderr, "Tenant ID:    %s\n", tenantID)
	fmt.Fprintf(os.Stderr, "Session ID:   %s\n", sessionID)
	fmt.Fprintf(os.Stderr, "Session:      Demo: Introduction to Go (waiting)\n")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "Teacher:      Ms. Smith (%s)\n", teacherID)
	fmt.Fprintf(os.Stderr, "Teacher Token:\n%s\n", teacherToken)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "Student:      Alex (%s)\n", studentID)
	fmt.Fprintf(os.Stderr, "Student Token:\n%s\n", studentToken)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Paste a token into the app at http://localhost:5173 to get started.")
}

func mintToken(secret string, tenantID, userID uuid.UUID, username, role string) string {
	claims := jwt.MapClaims{
		"tenant_id": tenantID.String(),
		"user_id":   userID.String(),
		"username":  username,
		"role":      role,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}
