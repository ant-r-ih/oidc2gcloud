package creds

import (
	"encoding/json"
	"testing"
)

func TestOIDCTokenJSON(t *testing.T) {
	token := &OIDCToken{
		AccessToken:  "test-access-token",
		IDToken:      "test-id-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}

	// Marshal to JSON
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("Failed to marshal token: %v", err)
	}

	// Unmarshal from JSON
	var decoded OIDCToken
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}

	// Verify fields
	if decoded.AccessToken != token.AccessToken {
		t.Errorf("Expected access_token %s, got %s", token.AccessToken, decoded.AccessToken)
	}

	if decoded.IDToken != token.IDToken {
		t.Errorf("Expected id_token %s, got %s", token.IDToken, decoded.IDToken)
	}

	if decoded.TokenType != token.TokenType {
		t.Errorf("Expected token_type %s, got %s", token.TokenType, decoded.TokenType)
	}

	if decoded.ExpiresIn != token.ExpiresIn {
		t.Errorf("Expected expires_in %d, got %d", token.ExpiresIn, decoded.ExpiresIn)
	}
}

func TestOIDCTokenWithoutRefreshToken(t *testing.T) {
	// Test JSON without refresh_token (should use omitempty)
	jsonData := `{
		"access_token": "test-access",
		"id_token": "test-id",
		"token_type": "Bearer",
		"expires_in": 3600
	}`

	var token OIDCToken
	err := json.Unmarshal([]byte(jsonData), &token)
	if err != nil {
		t.Fatalf("Failed to unmarshal token: %v", err)
	}

	if token.RefreshToken != "" {
		t.Errorf("Expected empty refresh_token, got %s", token.RefreshToken)
	}

	if token.AccessToken != "test-access" {
		t.Errorf("Expected access_token 'test-access', got %s", token.AccessToken)
	}
}

func TestLoginDetails(t *testing.T) {
	details := &LoginDetails{
		Username: "user@example.com",
		Password: "secret-password",
		MFAToken: "123456",
		URL:      "https://authentik.example.com",
	}

	// Just verify the struct can be created and accessed
	if details.Username != "user@example.com" {
		t.Errorf("Expected username 'user@example.com', got %s", details.Username)
	}

	if details.URL != "https://authentik.example.com" {
		t.Errorf("Expected URL 'https://authentik.example.com', got %s", details.URL)
	}
}
