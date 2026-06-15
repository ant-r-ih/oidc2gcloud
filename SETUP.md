# oidc2gcloud Setup Guide

This guide explains how to configure Authentik OIDC provider and GCP Workload Identity Federation to enable research unit members to access GCP CLI.

## Prerequisites

- GCP project created
- Authentik installation with admin access
- gcloud CLI installed

## 1. GCP Workload Identity Federation Setup

### 1.1 Gather Basic Information

```bash
# Project ID
PROJECT_ID="my-project"

# Get project number
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project Number: $PROJECT_NUMBER"
# Example: 123456789012
```

### 1.2 Enable Required APIs

```bash
gcloud services enable iam.googleapis.com --project=$PROJECT_ID
gcloud services enable iamcredentials.googleapis.com --project=$PROJECT_ID
gcloud services enable sts.googleapis.com --project=$PROJECT_ID
```

### 1.3 Create Workload Identity Pool

```bash
POOL_ID="authentik-pool"

gcloud iam workload-identity-pools create $POOL_ID \
  --project=$PROJECT_ID \
  --location=global \
  --display-name="Authentik OIDC Pool"
```

### 1.4 Create OIDC Provider

**Important:** The Authentik Issuer URL must include `/application/o/{slug}/`.

```bash
PROVIDER_ID="authentik-provider"
ISSUER_URL="https://idp.example.com/application/o/gcp-workload/"
CLIENT_ID="YOUR_CLIENT_ID"

gcloud iam workload-identity-pools providers create-oidc $PROVIDER_ID \
  --project=$PROJECT_ID \
  --location=global \
  --workload-identity-pool=$POOL_ID \
  --issuer-uri="$ISSUER_URL" \
  --allowed-audiences="$CLIENT_ID" \
  --attribute-mapping="google.subject=assertion.email,attribute.email=assertion.email" \
  --attribute-condition="assertion.email.contains('@example.com')"
```

**Attribute Mapping Notes:**
- `google.subject=assertion.email` - User email recorded in GCP audit logs
- `attribute.email=assertion.email` - Enable filtering by email attribute
- `attribute-condition` - Restrict to specific domain (optional)

### 1.5 Create Service Account

```bash
SA_NAME="unit-base"
SA_EMAIL="$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"

gcloud iam service-accounts create $SA_NAME \
  --project=$PROJECT_ID \
  --display-name="Research Unit Base Service Account"
```

### 1.6 Grant Permissions to Service Account

```bash
# Project-level permissions (e.g., Editor)
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$SA_EMAIL" \
  --role="roles/editor"

# Add other roles as needed
# gcloud projects add-iam-policy-binding $PROJECT_ID \
#   --member="serviceAccount:$SA_EMAIL" \
#   --role="roles/compute.instanceAdmin.v1"
```

### 1.7 Allow Workload Identity Pool to Access Service Account

**Important:** Two roles are required.

```bash
# Role 1: workloadIdentityUser (basic)
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/*"

# Role 2: serviceAccountTokenCreator (required for token generation)
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.serviceAccountTokenCreator" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/*"
```

**To restrict to specific email addresses:**

```bash
# Filter by email attribute instead of wildcard (*)
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/attribute.email/*"
```

### 1.8 Verify Configuration

```bash
# Check Workload Identity Pool
gcloud iam workload-identity-pools describe $POOL_ID \
  --project=$PROJECT_ID \
  --location=global

# Check OIDC Provider
gcloud iam workload-identity-pools providers describe $PROVIDER_ID \
  --project=$PROJECT_ID \
  --location=global \
  --workload-identity-pool=$POOL_ID

# Check Service Account IAM Policy
gcloud iam service-accounts get-iam-policy $SA_EMAIL --project=$PROJECT_ID
```

---

## 2. Authentik OIDC Provider Setup

### 2.1 Create Application

In Authentik Admin UI:

1. **Applications → Create**
2. Basic info:
   - Name: `GCP Workload Identity`
   - Slug: `gcp-workload`

### 2.2 Create OAuth2/OIDC Provider

1. **Providers → Create → OAuth2/OpenID Provider**

2. Basic settings:
   - Name: `GCP OIDC Provider`
   - Authorization flow: `default-authentication-flow (implicit consent)`
   - Client type: `Confidential`
   - Client ID: Auto-generated (e.g., `YOUR_CLIENT_ID`)
   - Client Secret: Auto-generated (save this)
   - Redirect URIs: `http://localhost:8085/callback`

3. **Important: Signing Key Configuration**
   - Signing Key: **Select RSA key** (HS256 not supported!)
   - GCP Workload Identity only supports RS256

4. Scopes configuration:
   - Scopes: `openid`, `email`, `profile`
   - Sub Mode: `Based on the User's Email` (recommended)
   - Or `Based on the User's UPN`

5. **Add Scope Mappings:**
   - Go to `Scope Mappings` tab and select:
     - ✅ `authentik default OAuth Mapping: OpenID 'email'`
     - ✅ `authentik default OAuth Mapping: OpenID 'openid'`
     - ✅ `authentik default OAuth Mapping: OpenID 'profile'`

### 2.3 Link Provider to Application

1. **Applications → GCP Workload Identity → Edit**
2. Provider: Select `GCP OIDC Provider`
3. Launch URL: (leave empty, CLI-only)

### 2.4 Record Configuration Values

Save the following information:

```bash
Issuer URL: https://idp.example.com/application/o/gcp-workload/
Client ID: YOUR_CLIENT_ID
Client Secret: [saved value]
Redirect URI: http://localhost:8085/callback
```

**Verification:**
- Access `https://{authentik-domain}/application/o/{slug}/.well-known/openid-configuration`
- Verify ID Token includes email claim

---

## 3. oidc2gcloud CLI Setup

### 3.1 Build

```bash
cd ~/git/00ANT/AUTH/oidc2gcloud
go build -o oidc2gcloud ./cmd/oidc2gcloud

# Copy to system path
sudo cp oidc2gcloud /usr/local/bin/
# or
mkdir -p ~/bin
cp oidc2gcloud ~/bin/
export PATH="$HOME/bin:$PATH"
```

### 3.2 Configure Profile

```bash
# Interactive configuration
oidc2gcloud configure --profile my-profile

# Or manually create ~/.oidc2gcloud
cat > ~/.oidc2gcloud << 'EOF'
[my-profile]
provider = oidc
issuer = https://idp.example.com/application/o/gcp-workload/
client_id = YOUR_CLIENT_ID
client_secret = YOUR_CLIENT_SECRET_HERE
redirect_uri = http://localhost:8085/callback
scopes = openid,email,profile
gcp_project_number = 123456789012
gcp_pool_id = authentik-pool
gcp_provider_id = authentik-provider
gcp_service_account = unit-base@my-project.iam.gserviceaccount.com
gcp_project = my-project
EOF

chmod 600 ~/.oidc2gcloud
```

### 3.3 Shell Alias Setup (Recommended)

```bash
# Add to ~/.zshrc or ~/.bashrc
cat >> ~/.zshrc << 'EOF'

# oidc2gcloud alias (auto-refresh)
alias gcloud='oidc2gcloud exec --profile my-profile -- gcloud'
EOF

source ~/.zshrc
```

---

## 4. Verification

### 4.1 Initial Login

```bash
oidc2gcloud login --profile my-profile
```

1. Browser opens automatically
2. Log in to Authentik
3. Return to CLI and tokens are saved

### 4.2 Test gcloud Commands

```bash
# Using exec (auto-refresh)
oidc2gcloud exec --profile my-profile -- gcloud projects list

# Or using alias
gcloud compute instances list --project=my-project
gcloud storage buckets list --project=my-project
```

### 4.3 Audit Log Inspection

GCP audit logs track user operations.

#### Web UI Inspection

1. **Open GCP Console → Logging → Logs Explorer**
   - URL: https://console.cloud.google.com/logs/query

2. **Enter query**

   Show all operations via oidc2gcloud:
   ```
   protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"
   ```

3. **Expand log details**
   - `protoPayload.authenticationInfo.principalEmail` → Service Account (unit-base)
   - `protoPayload.authenticationInfo.serviceAccountDelegationInfo` → **Actual user**

   Example:
   ```json
   {
     "principalEmail": "unit-base@my-project.iam.gserviceaccount.com",
     "serviceAccountDelegationInfo": [
       {
         "firstPartyPrincipal": {
           "principalSubject": "principal://iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/authentik-pool/subject/user@example.com"
         }
       }
     ]
   }
   ```

#### Search Specific User Operations

```
protoPayload.authenticationInfo.serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject:"user@example.com"
```

#### CLI Inspection

```bash
# Get recent audit logs
gcloud logging read \
  'protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"' \
  --limit 10 \
  --format json \
  --project my-project

# Search specific user operations
gcloud logging read \
  'protoPayload.authenticationInfo.serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject:"user@example.com"' \
  --limit 10 \
  --format json \
  --project my-project

# Search specific operation (e.g., compute instances create)
gcloud logging read \
  'protoPayload.methodName="v1.compute.instances.insert"
   AND protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"' \
  --limit 10 \
  --format json \
  --project my-project
```

#### Python Script for Log Analysis

```python
from google.cloud import logging

client = logging.Client(project="my-project")

# Get operations via oidc2gcloud
filter_str = '''
protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"
'''

for entry in client.list_entries(filter_=filter_str, max_results=10):
    auth_info = entry.payload.get('authenticationInfo', {})
    delegation_info = auth_info.get('serviceAccountDelegationInfo', [])
    
    if delegation_info:
        principal_subject = delegation_info[0].get('firstPartyPrincipal', {}).get('principalSubject', '')
        # Extract email from "principal://.../subject/user@example.com"
        user_email = principal_subject.split('subject/')[-1] if 'subject/' in principal_subject else 'unknown'
    else:
        user_email = 'unknown'
    
    method = entry.payload.get('methodName', 'unknown')
    resource = entry.payload.get('resourceName', 'unknown')
    
    print(f"User: {user_email}")
    print(f"Method: {method}")
    print(f"Resource: {resource}")
    print(f"Time: {entry.timestamp}")
    print("---")
```

#### Audit Log Notes

- **Service Account Email**: `unit-base@my-project.iam.gserviceaccount.com` always displayed
- **Actual User**: Recorded in `serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject`
- **Retention Period**: Default 30 days (Admin Activity), 400 days (Data Access)
- **Log Enablement**: Already enabled (automatically recorded with Workload Identity Federation)

---

## 5. Troubleshooting

### Error: "HMAC algorithm is not supported"

**Cause:** HS256 key selected in Authentik Provider

**Solution:**
1. Authentik Admin → Providers → [Your Provider] → Edit
2. Signing Key: **Select RSA key**
3. Save

---

### Error: "ID Token missing email claim"

**Cause:** Scopes not properly passed or Scope Mappings not selected

**Solution:**
1. Check Authentik Provider Scope Mappings
2. Select `authentik default OAuth Mapping: OpenID 'email'`
3. Verify oidc2gcloud scopes: `openid,email,profile` (comma-separated)

---

### Error: "Permission 'iam.serviceAccounts.getAccessToken' denied"

**Cause:** Service Account missing `roles/iam.serviceAccountTokenCreator`

**Solution:**
```bash
gcloud iam service-accounts add-iam-policy-binding \
  unit-base@my-project.iam.gserviceaccount.com \
  --role="roles/iam.serviceAccountTokenCreator" \
  --member="principalSet://iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/authentik-pool/*"
```

---

### Error: "IAM Service Account Credentials API has not been used"

**Cause:** API not enabled

**Solution:**
```bash
gcloud services enable iamcredentials.googleapis.com --project=my-project
```

---

### Browser doesn't open

**Solution:**
1. Manually copy the displayed URL
2. Open in browser
3. Or set `OIDC2GCLOUD_BROWSER_CMD` environment variable for custom command

---

### Token expired

**Symptom:** `gcloud` commands fail with `You do not currently have an active account selected` error

**Solution:**
- Use `oidc2gcloud exec` (auto-refresh)
- Or manually run `oidc2gcloud login --profile my-profile`

---

## 6. Member Distribution Guide

### 6.1 Quick Setup Guide

Setup instructions to distribute to research unit members:

```markdown
# GCP CLI Access Setup

## 1. Install oidc2gcloud

Obtain the `oidc2gcloud` binary from administrator and run:

```bash
mkdir -p ~/bin
mv oidc2gcloud ~/bin/
chmod +x ~/bin/oidc2gcloud
```

## 2. Copy Configuration File

Place the `.oidc2gcloud` file received from administrator:

```bash
cp .oidc2gcloud ~/.oidc2gcloud
chmod 600 ~/.oidc2gcloud
```

## 3. Shell Configuration

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="$HOME/bin:$PATH"
alias gcloud='oidc2gcloud exec --profile my-profile -- gcloud'
source ~/.zshrc
```

## 4. Initial Login

```bash
oidc2gcloud login --profile my-profile
```

Browser will open. Log in to Authentik.

## 5. Usage

```bash
# Use gcloud commands normally
gcloud compute instances list --project=my-project
gcloud storage buckets list --project=my-project
```

Token is valid for 1 hour. Auto-refresh on expiry.
```

---

## 7. Security Considerations

- **Client Secret Management:** Set `.oidc2gcloud` file permissions to 600
- **Token Validity:** 1 hour (GCP default)
- **Audit Logs:** All operations recorded with user email address
- **Access Scope:** Controlled by Service Account permissions (Editor, Viewer, etc.)
- **OnePassword Integration:** Never store credentials in plain text

---

## 8. References

- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Authentik OAuth2/OIDC Provider](https://docs.goauthentik.io/docs/providers/oauth2/)
- [gcloud CLI Reference](https://cloud.google.com/sdk/gcloud/reference)

---

## Appendix A: OnePassword Integration (Optional)

Automate browser authentication.

### Prerequisites

- OnePassword CLI installed (`brew install --cask 1password-cli`)
- Credentials stored in 1Password

### Setup Steps

#### 1. Store Credentials in 1Password

Create item in 1Password:
- Item Name: "Authentik Credentials"
- username: your.email@example.com
- password: your-password

#### 2. Create Environment File

```bash
# Get OnePassword reference path
op item get "Authentik Credentials" --format json

# Create ~/.oidc2gcloud.env
cat > ~/.oidc2gcloud.env << 'EOF'
OIDC_USERNAME=op://Private/YOUR_ITEM_ID/username
OIDC_PASSWORD=op://Private/YOUR_ITEM_ID/password
EOF

chmod 600 ~/.oidc2gcloud.env
```

#### 3. Add to Shell Configuration

```bash
# Add to ~/.zshrc or ~/.bashrc
cat >> ~/.zshrc << 'EOF'

# oidc2gcloud with OnePassword
export OIDC2GCLOUD_BROWSER_AUTOFILL=1
export OIDC2GCLOUD_BROWSER_CMD='op run --env-file ~/.oidc2gcloud.env -- open'
EOF

source ~/.zshrc
```

#### 4. Usage

```bash
# Login (browser auto-fills credentials)
oidc2gcloud login --profile my-profile
```

### Security Notes

- Set proper permissions: `chmod 600 ~/.oidc2gcloud.env`
- Never commit `.oidc2gcloud.env` to version control
- Use `op://` references instead of plain text credentials
