# Data Models

## User

Structure from `internal/model/user.go`:

```go
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
    EventID             uuid.UUID  `db:"event_id"`
}
```

| Field | Type | Purpose |
|-------|------|---------|
| `ID` | UUID | Primary key |
| `Login` | string | Username (3-32 chars) |
| `Email` | string | Email (verified via code) |
| `HashPassword` | *string | bcrypt hash (nil if not set) |
| `AuthPreference` | AuthMethod | Preferred authentication method |
| `TOTPSecretEncrypted` | *string | Encrypted TOTP secret |
| `HashTOTPResetCodes` | []byte | TOTP recovery code hashes |
| `EmailVerifiedAt` | *time.Time | Email verification timestamp |
| `TOTPEnabledAt` | *time.Time | TOTP activation timestamp |
| `EventID` | UUID | Kafka event ID for outbox pattern |

## AuthMethod

Enum type for authentication methods:

```go
type AuthMethod string

const (
    AuthMethodPassword      AuthMethod = "password"
    AuthMethodPasswordEmail AuthMethod = "password_email"
    AuthMethodTOTP          AuthMethod = "totp"
    AuthMethodPasswordTOTP  AuthMethod = "password_totp"
)
```

## DTOs (Data Transfer Objects)

### RegisterRequest

```go
type RegisterRequest struct {
    Login    string `json:"login" validate:"required,min=3,max=32"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
```

### RegisterResponse

```go
type RegisterResponse struct {
    ID          string `json:"id"`
    Login       string `json:"login"`
    Email       string `json:"email"`
    AccessToken string `json:"access_token"`
}
```

### RepoRegisterResponse

DTO for passing data between service → repository layers:

```go
type RepoRegisterResponse struct {
    UUID         string `json:"UUID"`
    Login        string `json:"login"`
    Email        string `json:"email"`
    EventID      string `json:"eventId"`
    HashPassword string `json:"hashedPassword"`
    SessionID    string `json:"sessionID"`
    Code         string `json:"code"`
    CreatedAt    int64  `json:"CreatedAt"`
}
```

### SessionData

```go
type SessionData struct {
    RefreshToken string `json:"refresh_token"`
    IP           string `json:"ip"`
    UserAgent    string `json:"user_agent"`
    CreatedAt    int64  `json:"created_at"`
    ExpiresAt    int64  `json:"expires_at"`
}
```

### Login / MFA models

```go
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
```

### LoginData

Internal model for passing user data during login:

```go
type LoginData struct {
    PasswordHash        string
    AuthMethod          string
    TOTPSecretEncrypted *string
    EmailVerifiedAt     *time.Time
    CurrentCode         string
}
```

### SSO / S2S models

```go
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
```

### Other

```go
type HotSwapEmail struct {
    Email string `json:"email" validate:"required,email"`
}

type ConfirmCode struct {
    Code string `json:"code" validate:"required,min=6"`
}
```

## Kafka Models

From `internal/model/kafka.go`:

```go
type OutboxEvent struct {
    ID        uuid.UUID
    EventType string
    Payload   []byte
    CreatedAt time.Time
}

type UserRegisteredEvent struct {
    UserID    string
    Login     string
    Email     string
    CreatedAt int64
}