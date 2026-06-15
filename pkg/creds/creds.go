package creds

// LoginDetails represents the login credentials and configuration
type LoginDetails struct {
	Username string
	Password string
	MFAToken string
	URL      string
}

// OIDCToken represents the tokens returned from OIDC provider
type OIDCToken struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}
