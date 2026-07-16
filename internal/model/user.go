package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthMethod string

const (
	AuthMethodPassword      AuthMethod = "password"
	AuthMethodPasswordEmail AuthMethod = "password_email"
	AuthMethodTOTP          AuthMethod = "totp"
	AuthMethodPasswordTOTP  AuthMethod = "password_totp"
)

type User struct {
	ID                  uuid.UUID  `db:"id"`
	Login               string     `db:"login"`
	Email               string     `db:"email"`
	HashPassword        *string    `db:"hash_password"`
	AuthPreference      AuthMethod `db:"auth_preference"`
	TOTPSecretEncrypted *string    `db:"totp_secret_encrypted"`
	HashTOTPResetCodes  []byte     `db:"hash_totp_reset_codes"`
	EmailVerifiedAt     *time.Time `db:"email_verified_at"`
	TOTPEnabledAt       *time.Time `db:"totp_enabled_at"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"`
}

// RegisterRequest — то, что приходит от пользователя
type RegisterRequest struct {
	Login    string `json:"login" validate:"required,min=3,max=32"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterResponse — то, что мы отдаем при успехе
type RegisterResponse struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	Email       string `json:"email"`
	AccessToken string `json:"access_token"`
}

type RepoRegisterResponse struct {
	UUID         string `json:"UUID"`
	Login        string `json:"login"`
	Email        string `json:"email"`
	HashPassword string `json:"hashedPassword"`
	SessionID    string `json:"sessionID"`
	Code         string `json:"code"`
	CreatedAt    int64  `json:"CreatedAt"`
}

type SessionData struct {
	RefreshToken string `json:"refresh_token"`
	IP           string `json:"ip"`
	UserAgent    string `json:"user_agent"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
}

type HotSwapEmail struct {
	Email string `json:"email" validate:"required,email"`
}

type ConfirmCode struct {
	Code string `json:"code" validate:"required,min=6"`
}

type Login struct {
	Login string `json:"login" validate:"required,min=3,max=32"`
}

type LoginAuthMethodPassword struct {
	Password string `json:"password" validate:"required,min=8"`
}

type LoginAuthMethodPasswordEmail struct {
	Password string `json:"password" validate:"required,min=8"`
	Code     string `json:"code" validate:"required,min=6"`
}

type LoginAuthMethodTOTP struct {
	Code string `json:"code" validate:"required,min=6"`
}

type LoginAuthMethodPasswordTOTP struct {
	Password string `json:"password" validate:"required,min=8"`
	Code     string `json:"code" validate:"required,min=6"`
}

type LoginData struct {
	PasswordHash        string
	AuthMethod          string
	TOTPSecretEncrypted *string
	EmailVerifiedAt     *time.Time
	CurrentCode         string
}

type SSOTokenS2S struct {
	Token     string `json:"token" validate:"required,min=17"`
	ClientIP  string `json:"ip"`
	UserAgent string `json:"agent"`
}

type RefreshS2S struct {
	RefreshKey string `json:"refreshKey"`
	ClientIP   string `json:"ip"`
	UserAgent  string `json:"agent"`
}

type Request2GerAPI struct {
	URL          string `json:"url"`
	CompanyName  string `json:"companyName"` //опционально
	FullName     string `json:"fullName"`
	CompanyEmail string `json:"CompanyEmail"` //опционально
}

type SRequest2GerAPI struct {
	URL          string
	UUID         string //Аккаунт с которого была подана заявка на получение API key
	CompanyName  string
	FullName     string
	CompanyEmail string
}
