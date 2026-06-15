package cfg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	ini "gopkg.in/ini.v1"
)

const (
	// DefaultConfigPath the default oidc2gcloud configuration path
	DefaultConfigPath = "~/.oidc2gcloud"
)

var (
	// ErrProfileNotFound returned if the profile is not found in the configuration file
	ErrProfileNotFound = errors.New("profile not found, run configure to set it up")
)

// Profile represents an OIDC provider configuration
type Profile struct {
	Name       string `ini:"name"`
	Provider   string `ini:"provider"`
	Issuer     string `ini:"issuer"`
	ClientID   string `ini:"client_id"`
	ClientSecret string `ini:"client_secret"`
	RedirectURI string `ini:"redirect_uri"`
	Scopes     string `ini:"scopes"`

	// GCP Workload Identity Federation settings
	GCPProjectNumber  string `ini:"gcp_project_number"`
	GCPPoolID         string `ini:"gcp_pool_id"`
	GCPProviderID     string `ini:"gcp_provider_id"`
	GCPServiceAccount string `ini:"gcp_service_account"`
	GCPProject        string `ini:"gcp_project,omitempty"`
}

func (p Profile) String() string {
	return fmt.Sprintf(`profile {
  Name: %s
  Provider: %s
  Issuer: %s
  ClientID: %s
  RedirectURI: %s
  Scopes: %s
  GCP ProjectNumber: %s
  GCP PoolID: %s
  GCP ProviderID: %s
  GCP ServiceAccount: %s
}`, p.Name, p.Provider, p.Issuer, p.ClientID, p.RedirectURI, p.Scopes,
		p.GCPProjectNumber, p.GCPPoolID, p.GCPProviderID, p.GCPServiceAccount)
}

// ConfigLoader loads configuration from file
type ConfigLoader struct {
	Filename string
}

// NewConfigLoader creates a new config loader
func NewConfigLoader(filename string) *ConfigLoader {
	return &ConfigLoader{
		Filename: filename,
	}
}

// LoadProfile loads a profile from the configuration file
func (cl *ConfigLoader) LoadProfile(profileName string) (*Profile, error) {
	configPath, err := homedir.Expand(cl.Filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to expand config path")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, ErrProfileNotFound
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load config file")
	}

	section, err := cfg.GetSection(profileName)
	if err != nil {
		return nil, ErrProfileNotFound
	}

	profile := &Profile{Name: profileName}
	if err := section.MapTo(profile); err != nil {
		return nil, errors.Wrap(err, "failed to parse profile")
	}

	return profile, nil
}

// SaveProfile saves a profile to the configuration file
func (cl *ConfigLoader) SaveProfile(profile *Profile) error {
	configPath, err := homedir.Expand(cl.Filename)
	if err != nil {
		return errors.Wrap(err, "failed to expand config path")
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	var cfg *ini.File
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(configPath)
		if err != nil {
			return errors.Wrap(err, "failed to load config file")
		}
	}

	section, err := cfg.NewSection(profile.Name)
	if err != nil {
		// Section already exists, get it
		section, err = cfg.GetSection(profile.Name)
		if err != nil {
			return errors.Wrap(err, "failed to get section")
		}
	}

	if err := section.ReflectFrom(profile); err != nil {
		return errors.Wrap(err, "failed to save profile")
	}

	if err := cfg.SaveTo(configPath); err != nil {
		return errors.Wrap(err, "failed to save config file")
	}

	return nil
}

// ListProfiles lists all profiles in the configuration file
func (cl *ConfigLoader) ListProfiles() ([]string, error) {
	configPath, err := homedir.Expand(cl.Filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to expand config path")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load config file")
	}

	var profiles []string
	for _, section := range cfg.Sections() {
		if section.Name() != "DEFAULT" {
			profiles = append(profiles, section.Name())
		}
	}

	return profiles, nil
}
