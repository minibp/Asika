package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestInit(t *testing.T) {
	Init("test-secret", 24*time.Hour)
	if len(jwtSecret) == 0 {
		t.Error("jwtSecret should be set after Init")
	}
	if tokenExpiry != 24*time.Hour {
		t.Errorf("tokenExpiry = %v, want %v", tokenExpiry, 24*time.Hour)
	}
}

func TestHashPasswordAndCheck(t *testing.T) {
	password := "my-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// Verify correct password
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	// Verify wrong password
	if CheckPassword("wrong-password", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateAndValidateJWT(t *testing.T) {
	Init("test-secret", 24*time.Hour)

	token, err := GenerateJWT("testuser", "admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if claims["username"] != "testuser" {
		t.Errorf("username = %v, want testuser", claims["username"])
	}
	if claims["role"] != "admin" {
		t.Errorf("role = %v, want admin", claims["role"])
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	Init("test-secret", 24*time.Hour)

	_, err := ValidateJWT("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	// Use a secret with very short expiry time
	Init("test-secret", -1*time.Hour) // Expiry time in the past

	token, _ := GenerateJWT("testuser", "admin")

	// Since expiry is negative, token's exp should be in the past
	_, err := ValidateJWT(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestBlacklistToken(t *testing.T) {
	Init("test-secret", 24*time.Hour)

	token, _ := GenerateJWT("testuser", "admin")

	// Verify token is valid
	_, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("token should be valid before blacklist: %v", err)
	}

	// Add to blacklist
	BlacklistToken(token)

	// Verify token is now invalid
	_, err = ValidateJWT(token)
	if err == nil {
		t.Error("expected error for blacklisted token")
	}
}

func TestGetUserRole(t *testing.T) {
	claims := jwt.MapClaims{
		"username": "testuser",
		"role":     "admin",
	}

	role := GetUserRole(claims)
	if role != "admin" {
		t.Errorf("GetUserRole = %q, want admin", role)
	}
}

func TestGetUserRole_Empty(t *testing.T) {
	claims := jwt.MapClaims{}

	role := GetUserRole(claims)
	if role != "" {
		t.Errorf("GetUserRole = %q, want empty string", role)
	}
}

func TestGetUsername(t *testing.T) {
	claims := jwt.MapClaims{
		"username": "testuser",
		"role":     "admin",
	}

	username := GetUsername(claims)
	if username != "testuser" {
		t.Errorf("GetUsername = %q, want testuser", username)
	}
}

func TestGetUsername_Empty(t *testing.T) {
	claims := jwt.MapClaims{}

	username := GetUsername(claims)
	if username != "" {
		t.Errorf("GetUsername = %q, want empty string", username)
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		role     string
		required string
		want     bool
	}{
		{"admin", "admin", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"operator", "admin", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
		{"invalid", "viewer", false},
		{"viewer", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.role+"_"+tt.required, func(t *testing.T) {
			got := HasPermission(tt.role, tt.required)
			if got != tt.want {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.required, got, tt.want)
			}
		})
	}
}
