# Security

## JWT Tokens

Uses RS256 (RSA with SHA-256) signing.

### Access Token

- TTL: configurable (default 15 minutes)
- Sent in header `Authorization: Bearer <token>`
- Contains: `sub` (user_id), `iat`, `exp`, `email` (bool), role

### Refresh Token

- TTL: configurable (default 1 hour)
- Stored in HttpOnly cookie `refresh_token`
- Not accessible from JavaScript (XSS protection)
- On refresh — old token is invalidated (rotation)

### SSO Token (S2S)

- One-time token for passing session to partner services
- Generated via `POST /api/v1/sso`
- TTL: 30 seconds
- Requires API key for redemption (`X-Client-Secret`)

## Key Management

RSA keys are stored in `config/certs/`:
- `private_key.pem` — private key (for signing tokens)
- `public_key.pem` — public key (for verifying signature)

Loaded via `security.NewJWTManager(privateKeyPath, publicKeyPath, ttlMinutes)`.

## Password Hashing

Passwords are hashed using **bcrypt** (configurable cost factor):
```go
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
```

## Sensitive Data Encryption

The `core/pkg/security/encrypt.go` module provides:

```go
type Encryptor struct { ... }

func NewEncryptor(masterKey string) (*Encryptor, error)
func (e *Encryptor) Encrypt(plaintext string) (string, error)
func (e *Encryptor) Decrypt(ciphertext string) (string, error)
```

Used for encrypting TOTP secrets before storing in the DB. The master key is passed via configuration.

## API Keys (S2S)

Partner services authenticate via API keys:
- Key is sent in the `X-Client-Secret` header
- Validated via Redis (`HGET clients:secrets <key>`)
- Links `api_key` → `client_id`

## MFA (Multi-Factor Authentication)

Supported methods (enum `AuthMethod`):
- `password` — password only
- `password_email` — password + email code
- `totp` — TOTP only
- `password_totp` — password + TOTP

### Email Codes

- Sent during registration for email verification
- Stored in Redis with TTL
- Rate limit on sending: 1/minute

### TOTP

- Uses `pquerna/otp` library
- Secret is encrypted before storing in DB
- Recovery codes are hashed (bcrypt) and stored in DB
- When confirming TOTP, one-time recovery codes are returned

## Rate Limiting

Redis-based, configured per endpoint via `pkg/config/app_config.go`. Key is formed from path + IP (or user ID).

## CORS

Configured in `app.go` → `WithCORS()`:
- Allowed origins: from config
- Allowed methods: GET, POST, PUT, DELETE, OPTIONS
- Allowed headers: Content-Type, Authorization, X-Client-Secret
- Credentials: true (for cookies)