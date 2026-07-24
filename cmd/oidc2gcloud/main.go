package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/your-org/oidc2gcloud/pkg/cfg"
	"github.com/your-org/oidc2gcloud/pkg/gcp"
	"github.com/your-org/oidc2gcloud/pkg/provider"
)

var (
	app     = kingpin.New("oidc2gcloud", "Authenticate to GCP using OIDC providers")
	version = "1.0.0"

	// Global flags
	profileName = app.Flag("profile", "Configuration profile to use").Short('p').Default("default").String()
	configFile  = app.Flag("config", "Configuration file path").Default(cfg.DefaultConfigPath).String()

	// Commands
	loginCmd     = app.Command("login", "Authenticate with OIDC provider and obtain GCP credentials")
	configureCmd = app.Command("configure", "Configure a new profile")
	listCmd      = app.Command("list", "List configured profiles")
	envCmd       = app.Command("env", "Print environment variables to set")
	execCmd      = app.Command("exec", "Execute command with auto-refreshed credentials")
	execArgs     = execCmd.Arg("command", "Command and arguments to execute").Required().Strings()
	statusCmd    = app.Command("status", "Show token status and validity")
	statusVerify = statusCmd.Flag("verify", "Verify token by calling GCP API").Bool()
	versionCmd   = app.Command("version", "Print version information")
)

func main() {
	app.Version(version)
	app.Interspersed(false) // Allow commands to have their own flags
	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch command {
	case loginCmd.FullCommand():
		if err := runLogin(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case configureCmd.FullCommand():
		if err := runConfigure(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case listCmd.FullCommand():
		if err := runList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case envCmd.FullCommand():
		if err := runEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case execCmd.FullCommand():
		if err := runExec(*execArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case statusCmd.FullCommand():
		if err := runStatus(*statusVerify); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case versionCmd.FullCommand():
		fmt.Printf("oidc2gcloud version %s\n", version)
	}
}

func runLogin() error {
	// Load configuration
	configLoader := cfg.NewConfigLoader(*configFile)
	profile, err := configLoader.LoadProfile(*profileName)
	if err != nil {
		return errors.Wrap(err, "failed to load profile")
	}

	fmt.Printf("Authenticating with profile: %s\n", profile.Name)
	fmt.Printf("Provider: %s\n", profile.Provider)
	fmt.Printf("Issuer: %s\n\n", profile.Issuer)

	// Perform OIDC authentication
	oidcProvider := provider.NewOIDCProvider(profile)
	oidcToken, err := oidcProvider.Authenticate()
	if err != nil {
		return errors.Wrap(err, "authentication failed")
	}

	// Exchange OIDC token for GCP access token (saml2aws style)
	gcpManager := gcp.NewWorkloadIdentityManager(profile)
	gcpAccessToken, err := gcpManager.ExchangeTokenForGCPAccess(oidcToken)
	if err != nil {
		return errors.Wrap(err, "failed to exchange token for GCP access")
	}

	// Save GCP access token
	credFile, err := gcpManager.SaveAccessToken(gcpAccessToken)
	if err != nil {
		return errors.Wrap(err, "failed to save access token")
	}

	// Print usage instructions
	gcpManager.PrintUsageInstructions(credFile)

	return nil
}

func runConfigure() error {
	fmt.Println("Configure a new OIDC2GCloud profile")
	fmt.Println("=====================================")
	fmt.Println()

	profile := &cfg.Profile{
		Name:     *profileName,
		Provider: "oidc",
	}

	// Collect configuration
	fmt.Print("OIDC Issuer URL: ")
	fmt.Scanln(&profile.Issuer)

	fmt.Print("Client ID: ")
	fmt.Scanln(&profile.ClientID)

	fmt.Print("Client Secret: ")
	fmt.Scanln(&profile.ClientSecret)

	fmt.Print("Redirect URI (default: http://localhost:8085/callback): ")
	var redirectURI string
	fmt.Scanln(&redirectURI)
	if redirectURI == "" {
		profile.RedirectURI = "http://localhost:8085/callback"
	} else {
		profile.RedirectURI = redirectURI
	}

	fmt.Print("Scopes (default: openid,email,profile): ")
	var scopes string
	fmt.Scanln(&scopes)
	if scopes == "" {
		profile.Scopes = "openid,email,profile"
	} else {
		profile.Scopes = scopes
	}

	fmt.Println("\nGCP Workload Identity Federation Settings:")
	fmt.Print("GCP Project Number: ")
	fmt.Scanln(&profile.GCPProjectNumber)

	fmt.Print("Workload Identity Pool ID: ")
	fmt.Scanln(&profile.GCPPoolID)

	fmt.Print("Provider ID: ")
	fmt.Scanln(&profile.GCPProviderID)

	fmt.Print("Service Account Email: ")
	fmt.Scanln(&profile.GCPServiceAccount)

	fmt.Print("GCP Project ID (optional): ")
	fmt.Scanln(&profile.GCPProject)

	// Save configuration
	configLoader := cfg.NewConfigLoader(*configFile)
	if err := configLoader.SaveProfile(profile); err != nil {
		return errors.Wrap(err, "failed to save profile")
	}

	fmt.Printf("\n✓ Profile '%s' configured successfully\n", profile.Name)
	fmt.Printf("Configuration saved to: %s\n", *configFile)
	fmt.Printf("\nTo authenticate, run: oidc2gcloud login --profile %s\n", profile.Name)

	return nil
}

func runList() error {
	configLoader := cfg.NewConfigLoader(*configFile)
	profiles, err := configLoader.ListProfiles()
	if err != nil {
		return errors.Wrap(err, "failed to list profiles")
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println("Run 'oidc2gcloud configure' to create a new profile.")
		return nil
	}

	fmt.Println("Configured profiles:")
	for _, profileName := range profiles {
		profile, err := configLoader.LoadProfile(profileName)
		if err != nil {
			fmt.Printf("  - %s (error loading)\n", profileName)
			continue
		}
		fmt.Printf("  - %s (Provider: %s, Issuer: %s)\n", profile.Name, profile.Provider, profile.Issuer)
	}

	return nil
}

func runEnv() error {
	configLoader := cfg.NewConfigLoader(*configFile)
	profile, err := configLoader.LoadProfile(*profileName)
	if err != nil {
		return errors.Wrap(err, "failed to load profile")
	}

	gcpManager := gcp.NewWorkloadIdentityManager(profile)

	// Get access token file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	tokenFile := fmt.Sprintf("%s/.config/oidc2gcloud/%s-access-token.txt", homeDir, profile.Name)

	envVars := gcpManager.GetEnvironmentVariables(tokenFile)

	// Print environment variables in shell format
	for key, value := range envVars {
		fmt.Printf("export %s=\"%s\"\n", key, value)
	}

	return nil
}

func runExec(cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return errors.New("command required")
	}

	// Load configuration
	configLoader := cfg.NewConfigLoader(*configFile)
	profile, err := configLoader.LoadProfile(*profileName)
	if err != nil {
		return errors.Wrap(err, "failed to load profile")
	}

	// Check token expiry
	homeDir, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	metadataFile := fmt.Sprintf("%s/.config/oidc2gcloud/%s-metadata.json", homeDir, profile.Name)
	tokenFile := fmt.Sprintf("%s/.config/oidc2gcloud/%s-access-token.txt", homeDir, profile.Name)

	needLogin := false

	// Check if token file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "⚠️  No credentials found. Logging in...\n")
		needLogin = true
	} else {
		// Check token expiry
		if data, err := os.ReadFile(metadataFile); err == nil {
			var metadata map[string]interface{}
			if err := json.Unmarshal(data, &metadata); err == nil {
				if expiryStr, ok := metadata["token_expiry"].(string); ok {
					if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
						if time.Until(expiry) < 0 {
							fmt.Fprintf(os.Stderr, "⚠️  Token expired. Re-authenticating...\n")
							needLogin = true
						} else if time.Until(expiry) < 5*time.Minute {
							fmt.Fprintf(os.Stderr, "ℹ️  Token expires in %s\n", time.Until(expiry).Round(time.Minute))
						}
					}
				}
			}
		}
	}

	// Auto-login if needed
	if needLogin {
		if err := runLogin(); err != nil {
			return errors.Wrap(err, "auto-login failed")
		}
		fmt.Fprintf(os.Stderr, "✓ Re-authentication successful\n\n")
	}

	// Load token and set environment
	gcpManager := gcp.NewWorkloadIdentityManager(profile)
	envVars := gcpManager.GetEnvironmentVariables(tokenFile)

	// Set up environment for subprocess
	env := os.Environ()
	for key, value := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Execute command
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Preserve the exit code from the subprocess
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return errors.Wrap(err, "command execution failed")
	}

	return nil
}

func runStatus(verify bool) error {
	// Load configuration
	configLoader := cfg.NewConfigLoader(*configFile)
	profile, err := configLoader.LoadProfile(*profileName)
	if err != nil {
		return errors.Wrap(err, "failed to load profile")
	}

	homeDir, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	metadataFile := fmt.Sprintf("%s/.config/oidc2gcloud/%s-metadata.json", homeDir, profile.Name)
	tokenFile := fmt.Sprintf("%s/.config/oidc2gcloud/%s-access-token.txt", homeDir, profile.Name)

	fmt.Printf("Profile: %s\n", profile.Name)
	fmt.Printf("Project: %s\n", profile.GCPProject)
	fmt.Printf("Service Account: %s\n", profile.GCPServiceAccount)
	fmt.Println()

	// Check if token file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		fmt.Println("Status: ❌ No credentials found")
		fmt.Printf("Run: oidc2gcloud login --profile %s\n", profile.Name)
		return nil
	}

	// Read metadata
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		fmt.Println("Status: ⚠️  Token file exists but metadata missing")
		fmt.Printf("Run: oidc2gcloud login --profile %s\n", profile.Name)
		return nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		fmt.Println("Status: ⚠️  Invalid metadata format")
		fmt.Printf("Run: oidc2gcloud login --profile %s\n", profile.Name)
		return nil
	}

	// Check expiry
	expiryStr, ok := metadata["token_expiry"].(string)
	if !ok {
		fmt.Println("Status: ⚠️  Missing expiry information")
		return nil
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		fmt.Println("Status: ⚠️  Invalid expiry format")
		return nil
	}

	remaining := time.Until(expiry)

	if remaining < 0 {
		fmt.Println("Status: ❌ Token expired")
		fmt.Printf("Expired: %s ago\n", (-remaining).Round(time.Second))
		fmt.Printf("Run: oidc2gcloud login --profile %s\n", profile.Name)
		return nil
	}

	// Token is valid
	fmt.Println("Status: ✅ Valid")
	fmt.Printf("Expires: %s\n", expiry.Local().Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Remaining: %s\n", remaining.Round(time.Second))

	if remaining < 5*time.Minute {
		fmt.Println("\n⚠️  Token will expire soon. Consider running login.")
	}

	// Level 2: Verify with GCP API
	if verify {
		fmt.Println("\n--- API Verification ---")
		fmt.Println("Calling GCP API to verify token...")

		tokenData, err := os.ReadFile(tokenFile)
		if err != nil {
			return errors.Wrap(err, "failed to read token file")
		}

		// Call GCP API (projects.get as a simple test)
		apiURL := fmt.Sprintf("https://cloudresourcemanager.googleapis.com/v1/projects/%s", profile.GCPProject)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return errors.Wrap(err, "failed to create request")
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(tokenData)))

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("API Call: ❌ Failed (%v)\n", err)
			return nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Println("API Call: ✅ Success")
			fmt.Println("Token is valid and has proper permissions")
		} else {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("API Call: ❌ Failed (HTTP %d)\n", resp.StatusCode)
			fmt.Printf("Response: %s\n", string(body))
		}
	}

	return nil
}

// openBrowser opens the default browser with the given URL
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return errors.New("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}
