# Безопасность

## JWT Токены

Используется RS256 (RSA с SHA-256) подпись.

### Access Token

- TTL: настраиваемый (по умолчанию 15 минут)
- Передаётся в заголовке `Authorization: Bearer <token>`
- Содержит: `sub` (user_id), `iat`, `exp`, `email` (bool), роль

### Refresh Token

- TTL: настраиваемый (по умолчанию 1 час)
- Хранится в HttpOnly куке `refresh_token`
- Не доступен из JavaScript (защита от XSS)
- При обновлении — старый токен инвалидируется (ротация)

### SSO Token (S2S)

- Одноразовый токен для передачи сессии партнёрским сервисам
- Генерируется через `POST /api/v1/sso`
- TTL: 30 секунд
- Для обмена требуется API-ключ (`X-Client-Secret`)

## Управление ключами

RSA ключи хранятся в `config/certs/`:
- `private_key.pem` — приватный ключ (для подписи токенов)
- `public_key.pem` — публичный ключ (для проверки подписи)

Загружаются через `security.NewJWTManager(privateKeyPath, publicKeyPath, ttlMinutes)`.

## Хэширование паролей

Пароли хэшируются через **bcrypt** (cost factor настраивается):
```go
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
```

## Шифрование чувствительных данных

Модуль `core/pkg/security/encrypt.go` предоставляет:

```go
type Encryptor struct { ... }

func NewEncryptor(masterKey string) (*Encryptor, error)
func (e *Encryptor) Encrypt(plaintext string) (string, error)
func (e *Encryptor) Decrypt(ciphertext string) (string, error)
```

Используется для шифрования TOTP секретов перед сохранением в БД. Мастер-ключ передаётся через конфигурацию.

## API-ключи (S2S)

Партнёрские сервисы аутентифицируются через API-ключи:
- Ключ передаётся в заголовке `X-Client-Secret`
- Валидируется через Redis (`HGET clients:secrets <key>`)
- Связывает `api_key` → `client_id`

## MFA (Multi-Factor Authentication)

Поддерживаемые методы (перечисление `AuthMethod`):
- `password` — только пароль
- `password_email` — пароль + email код
- `totp` — только TOTP
- `password_totp` — пароль + TOTP

### Email коды

- Отправляются при регистрации для подтверждения email
- Хранятся в Redis с TTL
- Rate limit на отправку: 1/минуту

### TOTP

- Используется библиотека `pquerna/otp`
- Секрет шифруется перед сохранением в БД
- Коды восстановления хэшируются (bcrypt) и хранятся в БД
- При подтверждении TOTP возвращаются одноразовые коды восстановления

## Rate Limiting

На основе Redis, настраивается по эндпоинтам через `pkg/config/app_config.go`. Ключ формируется из пути + IP (или ID пользователя).

## CORS

Настроен в `app.go` → `WithCORS()`:
- Разрешённые origins: из конфига
- Разрешённые методы: GET, POST, PUT, DELETE, OPTIONS
- Разрешённые заголовки: Content-Type, Authorization, X-Client-Secret
- Credentials: true (для кук)