# oidc2gcloud

Authenticate to Google Cloud Platform using OIDC providers with Workload Identity Federation.

Inspired by [saml2aws](https://github.com/Versent/saml2aws), this tool provides a seamless CLI experience for accessing GCP resources through external OIDC identity providers.

## Features

- 🔐 **OIDC Authentication** - Works with any OIDC provider (Authentik, Keycloak, Okta, Azure AD, etc.)
- 🚀 **Simple CLI** - saml2aws-style interface for familiar experience
- 🔄 **Auto-refresh** - Automatic token renewal on expiry
- 🔒 **Secure** - Short-lived tokens (1 hour), no long-lived credentials
- 📝 **Audit Friendly** - User identity tracked in GCP audit logs
- 🎯 **Multi-Profile** - Manage multiple OIDC providers and GCP projects

## Installation

### Using Makefile (Recommended)

```bash
git clone git@github.com:ant-r-ih/oidc2gcloud.git
cd oidc2gcloud
make install
```

This will build and install the binary to `/usr/local/bin/oidc2gcloud`.

### Manual Build

```bash
git clone git@github.com:ant-r-ih/oidc2gcloud.git
cd oidc2gcloud

# Build
make build

# Install to custom location
mkdir -p ~/bin
cp oidc2gcloud ~/bin/
export PATH="$HOME/bin:$PATH"
```

### From Source

```bash
go install github.com/ant-r-ih/oidc2gcloud/cmd/oidc2gcloud@latest
```

### Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Clean build artifacts
make clean

# See all available commands
make help
```

## Quick Start

### 1. Configure Profile

Create `~/.oidc2gcloud` configuration file:

```ini
[my-profile]
provider = oidc
issuer = https://authentik.example.com/application/o/my-app/
client_id = YOUR_CLIENT_ID
client_secret = YOUR_CLIENT_SECRET
redirect_uri = http://localhost:8085/callback
scopes = openid,email,profile

gcp_project_number = 123456789012
gcp_pool_id = oidc-pool
gcp_provider_id = oidc-provider
gcp_service_account = my-sa@project-id.iam.gserviceaccount.com
gcp_project = project-id
```

Or use interactive configuration:

```bash
oidc2gcloud configure --profile my-profile
```

### 2. Login

```bash
oidc2gcloud login --profile my-profile
```

This will:
1. Open your browser for OIDC authentication
2. Exchange tokens with GCP Workload Identity Federation
3. Save credentials locally (valid for 1 hour)

### 3. Use with gcloud

**Option A: Exec wrapper (recommended)**

```bash
# Auto-refresh on token expiry
oidc2gcloud exec --profile my-profile -- gcloud compute instances list
oidc2gcloud exec --profile my-profile -- gcloud storage buckets list
```

**Option B: Shell alias (most convenient)**

```bash
# Add to ~/.zshrc or ~/.bashrc
alias gcloud='oidc2gcloud exec --profile my-profile -- gcloud'

# Then use gcloud normally
gcloud compute instances list
gcloud projects list
```

**Option C: Environment variables**

```bash
eval $(oidc2gcloud env --profile my-profile)
gcloud compute instances list
```

## Commands

### login

Authenticate with OIDC provider and obtain GCP credentials:

```bash
oidc2gcloud login --profile my-profile
```

### exec

Execute command with auto-refreshed credentials:

```bash
oidc2gcloud exec --profile my-profile -- gcloud compute instances list
oidc2gcloud exec --profile my-profile -- terraform apply
```

### env

Print environment variables for current shell:

```bash
eval $(oidc2gcloud env --profile my-profile)
```

### list

List all configured profiles:

```bash
oidc2gcloud list
```

### configure

Interactively configure a new profile:

```bash
oidc2gcloud configure --profile my-profile
```

### version

Print version information:

```bash
oidc2gcloud version
```

## Configuration

### Profile Format

The `~/.oidc2gcloud` file uses INI format:

```ini
[profile-name]
# OIDC Provider settings
provider = oidc
issuer = https://your-oidc-provider.com/
client_id = YOUR_CLIENT_ID
client_secret = YOUR_CLIENT_SECRET
redirect_uri = http://localhost:8085/callback
scopes = openid,email,profile

# GCP Workload Identity Federation settings
gcp_project_number = 123456789012
gcp_pool_id = workload-pool-id
gcp_provider_id = oidc-provider-id
gcp_service_account = service-account@project-id.iam.gserviceaccount.com
gcp_project = project-id
```

### Multiple Profiles

Manage different OIDC providers or GCP projects:

```ini
[dev]
issuer = https://dev.example.com/
gcp_project = my-dev-project
...

[prod]
issuer = https://prod.example.com/
gcp_project = my-prod-project
...
```

Switch between profiles:

```bash
oidc2gcloud login --profile dev
oidc2gcloud login --profile prod
```

## How It Works

```
┌──────────┐      ┌──────────────┐      ┌─────────────┐
│  User    │──1──▶│ OIDC Provider│◀─2──▶│  Browser    │
└──────────┘      └──────────────┘      └─────────────┘
     │                   │
     │                   │ 3. ID Token
     ▼                   ▼
┌──────────────────────────────────────────────────────┐
│              oidc2gcloud CLI                         │
│  4. Exchange ID Token → GCP Federated Token          │
│  5. Impersonate Service Account → Access Token       │
│  6. Save token (~/.config/oidc2gcloud/)              │
└──────────────────────────────────────────────────────┘
                      │
                      │ 7. CLOUDSDK_AUTH_ACCESS_TOKEN
                      ▼
              ┌───────────────┐
              │  gcloud CLI   │
              │  Terraform    │
              │  kubectl      │
              └───────────────┘
```

1. User runs `oidc2gcloud login`
2. Browser opens for OIDC authentication
3. ID token returned to CLI
4. CLI exchanges ID token for GCP federated token (STS API)
5. CLI impersonates service account to get access token
6. Access token saved locally (valid 1 hour)
7. Environment variable set for gcloud/terraform/kubectl

## Troubleshooting

### Browser doesn't open

Manually copy and paste the URL shown in the terminal.

### Token expired

Use `oidc2gcloud exec` for automatic refresh, or manually run:

```bash
oidc2gcloud login --profile my-profile
```

### "Profile not found"

Create the profile first:

```bash
oidc2gcloud configure --profile my-profile
```

### Permission errors

Verify your GCP Workload Identity Federation setup. See [SETUP.md](SETUP.md) for complete configuration guide.

## Setup Guide

For complete setup instructions including:
- GCP Workload Identity Federation configuration
- OIDC Provider setup (Authentik, Keycloak, etc.)
- Service Account permissions
- Troubleshooting common issues

See **[SETUP.md](SETUP.md)**

## Appendix

### A. OnePassword Integration (Optional)

Automate browser authentication with OnePassword CLI.

#### Prerequisites

- OnePassword CLI installed (`brew install --cask 1password-cli`)
- Credentials stored in 1Password

#### Setup Steps

1. **Store credentials in 1Password**

   Create an item with your OIDC username and password.

2. **Create environment file**

   ```bash
   # ~/.oidc2gcloud.env
   OIDC_USERNAME=op://Private/YOUR_ITEM_ID/username
   OIDC_PASSWORD=op://Private/YOUR_ITEM_ID/password
   ```

   Get the reference path:
   ```bash
   op item get "Your Item Name" --format json
   ```

3. **Configure shell**

   Add to `~/.zshrc` or `~/.bashrc`:

   ```bash
   export OIDC2GCLOUD_BROWSER_AUTOFILL=1
   export OIDC2GCLOUD_BROWSER_CMD='op run --env-file ~/.oidc2gcloud.env -- open'
   ```

4. **Login**

   ```bash
   oidc2gcloud login --profile my-profile
   # Browser opens with credentials auto-filled
   ```

#### Security Note

- Set proper permissions: `chmod 600 ~/.oidc2gcloud.env`
- Never commit `.oidc2gcloud.env` to version control
- Use `op://` references instead of plain text credentials

## Contributing

Contributions welcome! Please open an issue or submit a pull request.

## License

MIT License

## Acknowledgments

- Inspired by [saml2aws](https://github.com/Versent/saml2aws)
- Built for teams using external OIDC providers with GCP Workload Identity Federation
