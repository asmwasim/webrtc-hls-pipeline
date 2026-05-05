package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func main() {
	secret := flag.String("secret", "dev-secret-change-in-production", "JWT signing secret")
	role := flag.String("role", "teacher", "User role: teacher or student")
	username := flag.String("username", "", "Username")
	tenantID := flag.String("tenant", "", "Tenant ID (UUID, auto-generated if empty)")
	userID := flag.String("user", "", "User ID (UUID, auto-generated if empty)")
	ttl := flag.Duration("ttl", 24*time.Hour, "Token TTL")
	flag.Parse()

	if *username == "" {
		*username = *role + "-user"
	}
	if *tenantID == "" {
		*tenantID = uuid.New().String()
	}
	if *userID == "" {
		*userID = uuid.New().String()
	}

	claims := jwt.MapClaims{
		"tenant_id": *tenantID,
		"user_id":   *userID,
		"username":  *username,
		"role":      *role,
		"exp":       time.Now().Add(*ttl).Unix(),
		"iat":       time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(*secret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error signing token: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Tenant ID: %s\n", *tenantID)
	fmt.Fprintf(os.Stderr, "User ID:   %s\n", *userID)
	fmt.Fprintf(os.Stderr, "Username:  %s\n", *username)
	fmt.Fprintf(os.Stderr, "Role:      %s\n", *role)
	fmt.Fprintf(os.Stderr, "Expires:   %s\n", time.Now().Add(*ttl).Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Println(tokenStr)
}
