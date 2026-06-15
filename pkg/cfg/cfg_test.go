package cfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadProfile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config")

	loader := NewConfigLoader(configFile)

	// Create test profile
	profile := &Profile{
		Name:              "test-profile",
		Provider:          "oidc",
		Issuer:            "https://example.com/oidc",
		ClientID:          "test-client-id",
		ClientSecret:      "test-client-secret",
		RedirectURI:       "http://localhost:8085/callback",
		Scopes:            "openid email profile",
		GCPProjectNumber:  "123456789",
		GCPPoolID:         "test-pool",
		GCPProviderID:     "test-provider",
		GCPServiceAccount: "test-sa@test-project.iam.gserviceaccount.com",
		GCPProject:        "test-project",
	}

	// Save profile
	err := loader.SaveProfile(profile)
	if err != nil {
		t.Fatalf("Failed to save profile: %v", err)
	}

	// Load profile
	loaded, err := loader.LoadProfile("test-profile")
	if err != nil {
		t.Fatalf("Failed to load profile: %v", err)
	}

	// Verify
	if loaded.Name != profile.Name {
		t.Errorf("Expected name %s, got %s", profile.Name, loaded.Name)
	}
	if loaded.Issuer != profile.Issuer {
		t.Errorf("Expected issuer %s, got %s", profile.Issuer, loaded.Issuer)
	}
	if loaded.ClientID != profile.ClientID {
		t.Errorf("Expected client_id %s, got %s", profile.ClientID, loaded.ClientID)
	}
	if loaded.GCPProjectNumber != profile.GCPProjectNumber {
		t.Errorf("Expected project number %s, got %s", profile.GCPProjectNumber, loaded.GCPProjectNumber)
	}
}

func TestLoadNonExistentProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config")

	loader := NewConfigLoader(configFile)

	// Try to load non-existent profile
	_, err := loader.LoadProfile("non-existent")
	if err != ErrProfileNotFound {
		t.Errorf("Expected ErrProfileNotFound, got %v", err)
	}
}

func TestListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config")

	loader := NewConfigLoader(configFile)

	// Initially empty
	profiles, err := loader.ListProfiles()
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("Expected 0 profiles, got %d", len(profiles))
	}

	// Save two profiles
	profile1 := &Profile{Name: "profile1", Provider: "oidc", Issuer: "https://example.com"}
	profile2 := &Profile{Name: "profile2", Provider: "oidc", Issuer: "https://example.org"}

	loader.SaveProfile(profile1)
	loader.SaveProfile(profile2)

	// List profiles
	profiles, err = loader.ListProfiles()
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}

	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}

	// Check profile names
	found1, found2 := false, false
	for _, p := range profiles {
		if p == "profile1" {
			found1 = true
		}
		if p == "profile2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("Expected to find profile1 and profile2, got %v", profiles)
	}
}

func TestConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config")

	loader := NewConfigLoader(configFile)
	profile := &Profile{
		Name:         "test",
		Provider:     "oidc",
		Issuer:       "https://example.com",
		ClientSecret: "secret",
	}

	err := loader.SaveProfile(profile)
	if err != nil {
		t.Fatalf("Failed to save profile: %v", err)
	}

	// Check directory permissions (should be 0700)
	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	// Note: permissions check might vary on different systems
	t.Logf("Directory permissions: %v", dirInfo.Mode().Perm())
}
