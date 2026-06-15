package gcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/your-org/oidc2gcloud/pkg/cfg"
	"github.com/your-org/oidc2gcloud/pkg/creds"
)

// WorkloadIdentityManager manages GCP Workload Identity Federation
type WorkloadIdentityManager struct {
	profile *cfg.Profile
}

// GCPAccessToken represents a GCP access token with expiry
type GCPAccessToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int       `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// NewWorkloadIdentityManager creates a new workload identity manager
func NewWorkloadIdentityManager(profile *cfg.Profile) *WorkloadIdentityManager {
	return &WorkloadIdentityManager{
		profile: profile,
	}
}

// ExchangeTokenForGCPAccess exchanges OIDC token for GCP access token (saml2aws style)
func (wim *WorkloadIdentityManager) ExchangeTokenForGCPAccess(oidcToken *creds.OIDCToken) (*GCPAccessToken, error) {
	fmt.Println("Exchanging OIDC token for GCP access token...")

	// Step 1: Exchange OIDC ID Token for GCP federated token
	federatedToken, err := wim.exchangeForFederatedToken(oidcToken.IDToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exchange for federated token")
	}

	// Step 2: Exchange federated token for service account access token
	accessToken, err := wim.impersonateServiceAccount(federatedToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to impersonate service account")
	}

	fmt.Println("✓ GCP access token obtained")
	return accessToken, nil
}

// exchangeForFederatedToken calls GCP STS to get a federated token
func (wim *WorkloadIdentityManager) exchangeForFederatedToken(idToken string) (string, error) {
	stsURL := "https://sts.googleapis.com/v1/token"

	audience := fmt.Sprintf("//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		wim.profile.GCPProjectNumber,
		wim.profile.GCPPoolID,
		wim.profile.GCPProviderID)

	data := map[string]interface{}{
		"audience":           audience,
		"grantType":          "urn:ietf:params:oauth:grant-type:token-exchange",
		"requestedTokenType": "urn:ietf:params:oauth:token-type:access_token",
		"scope":              "https://www.googleapis.com/auth/cloud-platform",
		"subjectTokenType":   "urn:ietf:params:oauth:token-type:jwt",
		"subjectToken":       idToken,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal STS request")
	}

	resp, err := http.Post(stsURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", errors.Wrap(err, "failed to call STS API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read STS response")
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("STS API failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", errors.Wrap(err, "failed to parse STS response")
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", errors.New("no access_token in STS response")
	}

	return accessToken, nil
}

// impersonateServiceAccount uses federated token to get SA access token
func (wim *WorkloadIdentityManager) impersonateServiceAccount(federatedToken string) (*GCPAccessToken, error) {
	impersonateURL := fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
		wim.profile.GCPServiceAccount)

	data := map[string]interface{}{
		"scope": []string{"https://www.googleapis.com/auth/cloud-platform"},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal impersonate request")
	}

	req, err := http.NewRequest("POST", impersonateURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create impersonate request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", federatedToken))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call impersonate API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read impersonate response")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("impersonate API failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireTime  string `json:"expireTime"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.Wrap(err, "failed to parse impersonate response")
	}

	expireTime, err := time.Parse(time.RFC3339, result.ExpireTime)
	if err != nil {
		// Default to 1 hour if parsing fails
		expireTime = time.Now().Add(1 * time.Hour)
	}

	return &GCPAccessToken{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(time.Until(expireTime).Seconds()),
		ExpiresAt:   expireTime,
	}, nil
}

// SaveAccessToken saves the GCP access token (saml2aws style)
func (wim *WorkloadIdentityManager) SaveAccessToken(accessToken *GCPAccessToken) (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", errors.Wrap(err, "failed to get home directory")
	}

	credDir := filepath.Join(homeDir, ".config", "oidc2gcloud")
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return "", errors.Wrap(err, "failed to create credential directory")
	}

	// Save access token as plain text (for CLOUDSDK_AUTH_ACCESS_TOKEN)
	tokenFile := filepath.Join(credDir, fmt.Sprintf("%s-access-token.txt", wim.profile.Name))
	if err := os.WriteFile(tokenFile, []byte(accessToken.AccessToken), 0600); err != nil {
		return "", errors.Wrap(err, "failed to write token file")
	}

	// Also save metadata (expiry time) separately
	metadataFile := filepath.Join(credDir, fmt.Sprintf("%s-metadata.json", wim.profile.Name))
	metadata := map[string]interface{}{
		"token_expiry": accessToken.ExpiresAt.Format(time.RFC3339),
		"profile":      wim.profile.Name,
	}
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal metadata")
	}
	if err := os.WriteFile(metadataFile, metadataJSON, 0600); err != nil {
		return "", errors.Wrap(err, "failed to write metadata file")
	}

	return tokenFile, nil
}

// GetEnvironmentVariables returns the environment variables to set for gcloud
func (wim *WorkloadIdentityManager) GetEnvironmentVariables(tokenFile string) map[string]string {
	env := make(map[string]string)

	// Read the access token from file
	if tokenData, err := os.ReadFile(tokenFile); err == nil {
		env["CLOUDSDK_AUTH_ACCESS_TOKEN"] = string(tokenData)
	}

	if wim.profile.GCPProject != "" {
		env["CLOUDSDK_CORE_PROJECT"] = wim.profile.GCPProject
	}

	return env
}

// PrintUsageInstructions prints usage instructions
func (wim *WorkloadIdentityManager) PrintUsageInstructions(tokenFile string) {
	fmt.Println("\n✓ Authentication successful!")
	fmt.Println("\nTo use gcloud with this authentication:")
	fmt.Println("\nUse eval to set environment variables (recommended):")
	fmt.Printf("  eval $(oidc2gcloud env --profile %s)\n", wim.profile.Name)
	fmt.Println("\nThen run gcloud commands:")
	fmt.Printf("  gcloud compute instances list --project=%s\n", wim.profile.GCPProject)

	// Calculate and display expiry
	homeDir, _ := homedir.Dir()
	metadataFile := filepath.Join(homeDir, ".config", "oidc2gcloud", fmt.Sprintf("%s-metadata.json", wim.profile.Name))
	if data, err := os.ReadFile(metadataFile); err == nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(data, &metadata); err == nil {
			if expiryStr, ok := metadata["token_expiry"].(string); ok {
				if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
					remaining := time.Until(expiry)
					fmt.Printf("\nToken expires in: %s\n", remaining.Round(time.Minute))
					fmt.Printf("Run 'oidc2gcloud login --profile %s' when expired\n", wim.profile.Name)
				}
			}
		}
	}
}
