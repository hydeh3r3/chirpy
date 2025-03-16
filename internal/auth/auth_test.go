package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	// Test setup
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := time.Hour

	// Test creating a valid token
	t.Run("valid token", func(t *testing.T) {
		// Create token
		token, err := MakeJWT(userID, secret, expiresIn)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		// Validate token
		gotUserID, err := ValidateJWT(token, secret)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}
		if gotUserID != userID {
			t.Errorf("Got user ID %v, want %v", gotUserID, userID)
		}
	})

	// Test expired token
	t.Run("expired token", func(t *testing.T) {
		// Create expired token
		token, err := MakeJWT(userID, secret, -time.Hour)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		// Validate token
		_, err = ValidateJWT(token, secret)
		if err == nil {
			t.Error("Expected error for expired token, got nil")
		}
	})

	// Test wrong secret
	t.Run("wrong secret", func(t *testing.T) {
		// Create token
		token, err := MakeJWT(userID, secret, expiresIn)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		// Validate with wrong secret
		_, err = ValidateJWT(token, "wrong-secret")
		if err == nil {
			t.Error("Expected error for wrong secret, got nil")
		}
	})

	// Test invalid token format
	t.Run("invalid token format", func(t *testing.T) {
		_, err := ValidateJWT("invalid-token", secret)
		if err == nil {
			t.Error("Expected error for invalid token format, got nil")
		}
	})
}

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    string
		wantErr error
	}{
		{
			name:    "no header",
			headers: http.Header{},
			want:    "",
			wantErr: ErrNoAuthHeader,
		},
		{
			name: "valid header",
			headers: http.Header{
				"Authorization": []string{"Bearer test-token"},
			},
			want:    "test-token",
			wantErr: nil,
		},
		{
			name: "invalid format",
			headers: http.Header{
				"Authorization": []string{"test-token"},
			},
			want:    "",
			wantErr: ErrInvalidAuthHeader,
		},
		{
			name: "wrong prefix",
			headers: http.Header{
				"Authorization": []string{"NotBearer test-token"},
			},
			want:    "",
			wantErr: ErrInvalidAuthHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBearerToken(tt.headers)
			if err != tt.wantErr {
				t.Errorf("GetBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
