# Модели данных

## User (Пользователь)

Структура из `internal/model/user.go`:

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

| Поле | Тип | Назначение |
|------|-----|------------|
| `ID` | UUID | Первичный ключ |
| `Login` | string | Имя пользователя (3-32 символа) |
| `Email` | string | Email (верифицируется через код) |
| `HashPassword` | *string | bcrypt хэш пароля (nil если не задан) |
| `AuthPreference` | AuthMethod | Предпочитаемый метод аутентификации |
| `TOTPSecretEncrypted` | *string | Зашифрованный TOTP секрет |
| `HashTOTPResetCodes` | []byte | Хэши кодов восстановления TOTP |
| `EmailVerifiedAt` | *time.Time | Время подтверждения email |
| `TOTPEnabledAt` | *time.Time | Время активации TOTP |
| `EventID` | UUID | ID события Kafka для outbox-паттерна |

## AuthMethod

Тип-перечисление для метода аутентификации:

```go
type AuthMethod string

const (
    AuthMethodPassword      AuthMethod = "password"
    AuthMethodPasswordEmail AuthMethod = "password_email"
    AuthMethodTOTP          AuthMethod = "totp"
    AuthMethodPasswordTOTP  AuthMethod = "password_totp"
)
```

## DTO (Data Transfer Objects)

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

DTO для передачи данных между слоями service → repository:

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

### Login / MFA модели

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

Внутренняя модель для передачи данных о пользователе в процессе логина:

```go
type LoginData struct {
    PasswordHash        string
    AuthMethod          string
    TOTPSecretEncrypted *string
    EmailVerifiedAt     *time.Time
    CurrentCode         string
}
```

### SSO / S2S модели

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

### Прочие

```go
type HotSwapEmail struct {
    Email string `json:"email" validate:"required,email"`
}

type ConfirmCode struct {
    Code string `json:"code" validate:"required,min=6"`
}
```

## Kafka модели

Из `internal/model/kafka.go`:

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