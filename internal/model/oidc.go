package model

type RequestNewOIDCCode struct {
	ClientID string `json:"client_id"`
	Nonce    string `json:"nonce"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"` // Обычно "Bearer"
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type IDTokenClaims struct {
	Iss      string `json:"iss"`
	Sub      string `json:"sub"`
	Aud      string `json:"aud"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
	AuthTime int64  `json:"auth_time,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
}

type GetOIDCSession struct {
	Code      string `json:"code" validate:"required,min=17"`
	ClientIP  string `json:"ip"`
	UserAgent string `json:"agent"`
	ClientID  string `json:"client_id"`
}

type RefreshS2S struct {
	RefreshKey string `json:"refreshKey"`
	ClientIP   string `json:"ip"`
	UserAgent  string `json:"agent"`
}

type RegisterPartner struct {
	HomepageURL  string `json:"home_page_url"`
	ReturnURI    string `json:"return_uri"`
	ClientID     string `json:"client_id"`
	CompanyName  string `json:"companyName"`
	FullName     string `json:"fullName"`
	CompanyEmail string `json:"companyEmail"`
}
