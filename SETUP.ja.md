# oidc2gcloud セットアップガイド

このガイドでは、Authentik OIDC プロバイダーと GCP Workload Identity Federation を使用して、研究ユニットメンバーが GCP CLI にアクセスできるようにする手順を説明します。

## 前提条件

- GCP プロジェクトの作成済み
- Authentik のインストールと管理者権限
- gcloud CLI のインストール済み

## 1. GCP Workload Identity Federation の設定

### 1.1 基本情報の収集

```bash
# プロジェクトID
PROJECT_ID="my-project"

# プロジェクト番号を取得
PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format="value(projectNumber)")
echo "Project Number: $PROJECT_NUMBER"
# 例: 123456789012
```

### 1.2 必要な API を有効化

```bash
gcloud services enable iam.googleapis.com --project=$PROJECT_ID
gcloud services enable iamcredentials.googleapis.com --project=$PROJECT_ID
gcloud services enable sts.googleapis.com --project=$PROJECT_ID
```

### 1.3 Workload Identity Pool の作成

```bash
POOL_ID="authentik-pool"

gcloud iam workload-identity-pools create $POOL_ID \
  --project=$PROJECT_ID \
  --location=global \
  --display-name="Authentik OIDC Pool"
```

### 1.4 OIDC Provider の作成

**重要：** Authentik の Issuer URL は `/application/o/{slug}/` まで含める必要があります。

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

**attribute-mapping のポイント：**
- `google.subject=assertion.email` - GCP 監査ログに email が記録される
- `attribute.email=assertion.email` - email 属性でフィルタリング可能
- `attribute-condition` - 特定ドメインのみ許可（オプション）

### 1.5 Service Account の作成

```bash
SA_NAME="unit-base"
SA_EMAIL="$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"

gcloud iam service-accounts create $SA_NAME \
  --project=$PROJECT_ID \
  --display-name="Research Unit Base Service Account"
```

### 1.6 Service Account に権限を付与

```bash
# プロジェクトレベルの権限（例：Editor）
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$SA_EMAIL" \
  --role="roles/editor"

# 必要に応じて他のロールも追加
# gcloud projects add-iam-policy-binding $PROJECT_ID \
#   --member="serviceAccount:$SA_EMAIL" \
#   --role="roles/compute.instanceAdmin.v1"
```

### 1.7 Workload Identity Pool に Service Account へのアクセスを許可

**重要：** 2つのロールが必要です。

```bash
# ロール1: workloadIdentityUser（基本）
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/*"

# ロール2: serviceAccountTokenCreator（トークン生成に必須）
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.serviceAccountTokenCreator" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/*"
```

**特定のメールアドレスのみ許可する場合：**

```bash
# 全ユーザー（*）ではなく、email 属性でフィルタ
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$PROJECT_ID \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/$PROJECT_NUMBER/locations/global/workloadIdentityPools/$POOL_ID/attribute.email/*"
```

### 1.8 設定の確認

```bash
# Workload Identity Pool の確認
gcloud iam workload-identity-pools describe $POOL_ID \
  --project=$PROJECT_ID \
  --location=global

# OIDC Provider の確認
gcloud iam workload-identity-pools providers describe $PROVIDER_ID \
  --project=$PROJECT_ID \
  --location=global \
  --workload-identity-pool=$POOL_ID

# Service Account IAM ポリシーの確認
gcloud iam service-accounts get-iam-policy $SA_EMAIL --project=$PROJECT_ID
```

---

## 2. Authentik OIDC Provider の設定

### 2.1 Application の作成

Authentik Admin UI で：

1. **Applications → Create**
2. 基本情報：
   - Name: `GCP Workload Identity`
   - Slug: `gcp-workload`

### 2.2 OAuth2/OIDC Provider の作成

1. **Providers → Create → OAuth2/OpenID Provider**

2. 基本設定：
   - Name: `GCP OIDC Provider`
   - Authorization flow: `default-authentication-flow (implicit consent)`
   - Client type: `Confidential`
   - Client ID: 自動生成（例: `YOUR_CLIENT_ID`）
   - Client Secret: 自動生成（保存しておく）
   - Redirect URIs: `http://localhost:8085/callback`

3. **重要：Signing Key の設定**
   - Signing Key: **RSA 鍵を選択**（HS256 ではダメ！）
   - GCP Workload Identity は RS256 のみサポート

4. Scopes の設定：
   - Scopes: `openid`, `email`, `profile`
   - Sub Mode: `Based on the User's Email` (推奨)
   - または `Based on the User's UPN` でも可

5. **Scope Mappings の追加：**
   - `Scope Mappings` タブで以下を選択：
     - ✅ `authentik default OAuth Mapping: OpenID 'email'`
     - ✅ `authentik default OAuth Mapping: OpenID 'openid'`
     - ✅ `authentik default OAuth Mapping: OpenID 'profile'`

### 2.3 Application に Provider を紐付け

1. **Applications → GCP Workload Identity → Edit**
2. Provider: 作成した `GCP OIDC Provider` を選択
3. Launch URL: （空欄でOK、CLI専用）

### 2.4 設定値の確認

以下の情報をメモ：

```bash
Issuer URL: https://idp.example.com/application/o/gcp-workload/
Client ID: YOUR_CLIENT_ID
Client Secret: [保存した値]
Redirect URI: http://localhost:8085/callback
```

**確認方法：**
- Issuer URL: `https://{authentik-domain}/application/o/{slug}/.well-known/openid-configuration` にアクセス
- ID Token に email が含まれるか確認

---

## 3. oidc2gcloud CLI のセットアップ

### 3.1 ビルド

```bash
cd ~/git/00ANT/AUTH/oidc2gcloud
go build -o oidc2gcloud ./cmd/oidc2gcloud

# システムパスにコピー
sudo cp oidc2gcloud /usr/local/bin/
# または
mkdir -p ~/bin
cp oidc2gcloud ~/bin/
export PATH="$HOME/bin:$PATH"
```

### 3.2 プロファイルの設定

```bash
# 対話形式で設定
oidc2gcloud configure --profile my-profile

# または、手動で ~/.oidc2gcloud を作成
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

### 3.3 シェルエイリアスの設定（推奨）

```bash
# ~/.zshrc または ~/.bashrc に追加
cat >> ~/.zshrc << 'EOF'

# oidc2gcloud alias (自動再認証)
alias gcloud='oidc2gcloud exec --profile my-profile -- gcloud'
EOF

source ~/.zshrc
```

---

## 4. 動作確認

### 4.1 初回ログイン

```bash
oidc2gcloud login --profile my-profile
```

1. ブラウザが自動で開く
2. Authentik でログイン
3. CLI に戻ってトークン保存完了

### 4.2 gcloud コマンドのテスト

```bash
# exec 経由（自動再認証）
oidc2gcloud exec --profile my-profile -- gcloud projects list

# または、エイリアス使用
gcloud compute instances list --project=my-project
gcloud storage buckets list --project=my-project
```

### 4.3 監査ログの確認

GCP 監査ログでユーザーの操作を追跡できます。

#### Web UI での確認

1. **GCP Console → Logging → Logs Explorer** を開く
   - URL: https://console.cloud.google.com/logs/query

2. **クエリを入力**

   すべての oidc2gcloud 経由の操作を表示：
   ```
   protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"
   ```

3. **ログの詳細を展開**
   - `protoPayload.authenticationInfo.principalEmail` → Service Account（unit-base）
   - `protoPayload.authenticationInfo.serviceAccountDelegationInfo` → **実際のユーザー**

   例：
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

#### 特定ユーザーの操作を検索

```
protoPayload.authenticationInfo.serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject:"user@example.com"
```

#### CLI での確認

```bash
# 最近の監査ログを取得
gcloud logging read \
  'protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"' \
  --limit 10 \
  --format json \
  --project my-project

# 特定ユーザーの操作を検索
gcloud logging read \
  'protoPayload.authenticationInfo.serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject:"user@example.com"' \
  --limit 10 \
  --format json \
  --project my-project

# 特定の操作（例：compute instances create）を検索
gcloud logging read \
  'protoPayload.methodName="v1.compute.instances.insert"
   AND protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"' \
  --limit 10 \
  --format json \
  --project my-project
```

#### Python スクリプトでログ解析

```python
from google.cloud import logging

client = logging.Client(project="my-project")

# oidc2gcloud 経由の操作を取得
filter_str = '''
protoPayload.authenticationInfo.principalEmail="unit-base@my-project.iam.gserviceaccount.com"
'''

for entry in client.list_entries(filter_=filter_str, max_results=10):
    auth_info = entry.payload.get('authenticationInfo', {})
    delegation_info = auth_info.get('serviceAccountDelegationInfo', [])
    
    if delegation_info:
        principal_subject = delegation_info[0].get('firstPartyPrincipal', {}).get('principalSubject', '')
        # "principal://.../subject/user@example.com" から email を抽出
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

#### 監査ログの注意点

- **Service Account Email**: `unit-base@my-project.iam.gserviceaccount.com` が常に表示される
- **実際のユーザー**: `serviceAccountDelegationInfo.firstPartyPrincipal.principalSubject` に記録
- **保持期間**: デフォルトで 30 日間（Admin Activity）、400 日間（Data Access）
- **ログの有効化**: すでに有効（Workload Identity Federation 使用時は自動記録）

---

## 5. トラブルシューティング

### エラー: "HMAC algorithm is not supported"

**原因：** Authentik Provider で HS256 鍵が選択されている

**解決：**
1. Authentik Admin → Providers → [Your Provider] → Edit
2. Signing Key: **RSA 鍵を選択**
3. Save

---

### エラー: "ID Token missing email claim"

**原因：** Scopes が正しく渡されていない、または Scope Mappings が未選択

**解決：**
1. Authentik Provider の Scope Mappings を確認
2. `authentik default OAuth Mapping: OpenID 'email'` を選択
3. oidc2gcloud の設定で scopes を確認：`openid,email,profile`（カンマ区切り）

---

### エラー: "Permission 'iam.serviceAccounts.getAccessToken' denied"

**原因：** Service Account に `roles/iam.serviceAccountTokenCreator` が付与されていない

**解決：**
```bash
gcloud iam service-accounts add-iam-policy-binding \
  unit-base@my-project.iam.gserviceaccount.com \
  --role="roles/iam.serviceAccountTokenCreator" \
  --member="principalSet://iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/authentik-pool/*"
```

---

### エラー: "IAM Service Account Credentials API has not been used"

**原因：** API が有効化されていない

**解決：**
```bash
gcloud services enable iamcredentials.googleapis.com --project=my-project
```

---

### ブラウザが開かない

**解決：**
1. 表示された URL を手動でコピー
2. ブラウザで開く
3. または、環境変数 `OIDC2GCLOUD_BROWSER_CMD` でカスタムコマンドを指定

---

### トークンが期限切れ

**現象：** `gcloud` コマンドで `You do not currently have an active account selected` エラー

**解決：**
- `oidc2gcloud exec` を使う（自動再認証）
- または手動で `oidc2gcloud login --profile my-profile` を実行

---

## 6. メンバー向け配布資料

### 6.1 簡易セットアップガイド

研究ユニットメンバーに配布する手順書：

```markdown
# GCP CLI アクセス手順

## 1. oidc2gcloud のインストール

管理者から `oidc2gcloud` バイナリを受け取り、以下を実行：

```bash
mkdir -p ~/bin
mv oidc2gcloud ~/bin/
chmod +x ~/bin/oidc2gcloud
```

## 2. 設定ファイルのコピー

管理者から受け取った `.oidc2gcloud` ファイルを配置：

```bash
cp .oidc2gcloud ~/.oidc2gcloud
chmod 600 ~/.oidc2gcloud
```

## 3. シェル設定

```bash
# ~/.zshrc または ~/.bashrc に追加
export PATH="$HOME/bin:$PATH"
alias gcloud='oidc2gcloud exec --profile my-profile -- gcloud'
source ~/.zshrc
```

## 4. 初回ログイン

```bash
oidc2gcloud login --profile my-profile
```

ブラウザが開くので、Authentik でログインしてください。

## 5. 使い方

```bash
# 通常通り gcloud コマンドを使用
gcloud compute instances list --project=my-project
gcloud storage buckets list --project=my-project
```

トークンは1時間有効です。期限切れ時は自動で再認証されます。
```

---

## 7. セキュリティ考慮事項

- **Client Secret の管理：** `.oidc2gcloud` ファイルのパーミッションは 600 に設定
- **トークンの有効期限：** 1時間（GCP デフォルト）
- **監査ログ：** すべての操作がユーザーメールアドレス付きで記録される
- **アクセス範囲：** Service Account の権限で制御（Editor, Viewer など）
- **OnePassword 連携：** 認証情報を平文保存しない

---

## 8. 参考資料

- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Authentik OAuth2/OIDC Provider](https://docs.goauthentik.io/docs/providers/oauth2/)
- [gcloud CLI Reference](https://cloud.google.com/sdk/gcloud/reference)

---

## Appendix A: OnePassword 連携（オプション）

ブラウザ認証を自動化する場合に使用します。

### 前提条件

- OnePassword CLI インストール済み (`brew install --cask 1password-cli`)
- Authentik 認証情報を 1Password に保存済み

### セットアップ手順

#### 1. OnePassword に認証情報を保存

1Password に以下の項目を作成：
- Item Name: "Authentik Credentials"
- username: your.email@example.com
- password: your-password

#### 2. 環境ファイルを作成

```bash
# OnePassword の参照パスを取得
op item get "Authentik Credentials" --format json

# ~/.oidc2gcloud.env を作成
cat > ~/.oidc2gcloud.env << 'EOF'
OIDC_USERNAME=op://Private/YOUR_ITEM_ID/username
OIDC_PASSWORD=op://Private/YOUR_ITEM_ID/password
EOF

chmod 600 ~/.oidc2gcloud.env
```

#### 3. シェル設定に追加

```bash
# ~/.zshrc または ~/.bashrc に追加
cat >> ~/.zshrc << 'EOF'

# oidc2gcloud with OnePassword
export OIDC2GCLOUD_BROWSER_AUTOFILL=1
export OIDC2GCLOUD_BROWSER_CMD='op run --env-file ~/.oidc2gcloud.env -- open'
EOF

source ~/.zshrc
```

#### 4. 使用方法

```bash
# 通常通りログイン（ブラウザで自動入力される）
oidc2gcloud login --profile my-profile
```

### セキュリティ注意事項

- `~/.oidc2gcloud.env` のパーミッションは 600 に設定
- バージョン管理システムに `.oidc2gcloud.env` をコミットしない
- `op://` 参照を使い、平文パスワードは保存しない
