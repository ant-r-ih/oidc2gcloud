package browser

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallbackServer(t *testing.T) {
	// Start callback server
	server, err := NewCallbackServer(8086)
	if err != nil {
		t.Fatalf("Failed to create callback server: %v", err)
	}
	defer server.Shutdown()

	server.Start()

	// Get redirect URI
	redirectURI := server.GetRedirectURI()
	if !strings.Contains(redirectURI, "localhost:8085") {
		t.Errorf("Expected redirect URI to contain localhost:8085, got %s", redirectURI)
	}

	// Simulate callback with authorization code
	go func() {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get("http://localhost:8086/callback?code=test-auth-code&state=test-state")
		if err != nil {
			t.Errorf("Failed to send callback: %v", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Authentication Successful") {
			t.Errorf("Expected success message in response")
		}
	}()

	// Wait for code
	code, err := server.WaitForCode(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive code: %v", err)
	}

	if code != "test-auth-code" {
		t.Errorf("Expected code 'test-auth-code', got '%s'", code)
	}
}

func TestCallbackServerError(t *testing.T) {
	server, err := NewCallbackServer(8087)
	if err != nil {
		t.Fatalf("Failed to create callback server: %v", err)
	}
	defer server.Shutdown()

	server.Start()

	// Simulate callback with error
	go func() {
		time.Sleep(100 * time.Millisecond)
		http.Get("http://localhost:8087/callback?error=access_denied&error_description=User+denied+access")
	}()

	// Wait for code (should fail)
	_, err = server.WaitForCode(2 * time.Second)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Expected error to contain 'access_denied', got: %v", err)
	}
}

func TestCallbackServerTimeout(t *testing.T) {
	server, err := NewCallbackServer(8088)
	if err != nil {
		t.Fatalf("Failed to create callback server: %v", err)
	}
	defer server.Shutdown()

	server.Start()

	// Don't send any callback, just wait for timeout
	_, err = server.WaitForCode(500 * time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestGetRedirectURI(t *testing.T) {
	server, err := NewCallbackServer(8085)
	if err != nil {
		t.Fatalf("Failed to create callback server: %v", err)
	}
	defer server.Shutdown()

	redirectURI := server.GetRedirectURI()

	// Should always return localhost (not 127.0.0.1) for Authentik compatibility
	expected := "http://localhost:8085/callback"
	if redirectURI != expected {
		t.Errorf("Expected redirect URI %s, got %s", expected, redirectURI)
	}
}
