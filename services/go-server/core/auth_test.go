package core

import (
	"testing"
	"time"
)

func TestHashPassword(t *testing.T) {
	password := "test123456"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash should not be empty")
	}
	if hash == password {
		t.Fatal("Hash should not equal plain password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test123456"
	hash, _ := HashPassword(password)

	if !VerifyPassword(password, hash) {
		t.Fatal("VerifyPassword should return true for correct password")
	}

	if VerifyPassword("wrongpassword", hash) {
		t.Fatal("VerifyPassword should return false for wrong password")
	}
}

func TestCreateAndVerifyToken(t *testing.T) {
	testSecret := "test-secret-key-for-unit-test"
	claims := map[string]interface{}{
		"sub":      "admin",
		"admin_id": 1,
		"role":     "admin",
	}

	token, err := CreateAccessToken(claims, testSecret, 60*time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("Token should not be empty")
	}

	parsed, err := VerifyToken(token, testSecret)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if parsed["sub"] != "admin" {
		t.Fatalf("Expected sub=admin, got %v", parsed["sub"])
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	testSecret := "test-secret-key"
	claims := map[string]interface{}{"sub": "admin"}
	token, _ := CreateAccessToken(claims, testSecret, -1*time.Minute)

	_, err := VerifyToken(token, testSecret)
	if err == nil {
		t.Fatal("VerifyToken should fail for expired token")
	}
}
