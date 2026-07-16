# API Reference

Все эндпоинты доступны по базовому пути `/api/v1`.

## Аутентификация

Большинство эндпоинтов требуют заголовок `Authorization: Bearer <jwt_token>` (middleware `RequireAuth` / `RequireStrictAuth`).

## Эндпоинты

### Регистрация

```
POST /api/v1/register
```

**Тело запроса:**
```json
{
  "login": "username",
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 3 запроса / 10 минут на IP

**Cookies:** Устанавливает `refresh_token` (HttpOnly, 1 час)

---

### Вход (определение метода аутентификации)

```
POST /api/v1/login
```

**Тело запроса:**
```json
{
  "login": "username"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "authType": "password"
  }
}
```

**Cookies:** Устанавливает `auth_session_token` (HttpOnly, 20 минут)

**Rate Limit:** 5 запросов / 1 минута на IP

Возможные значения `authType`: `password`, `password_email`, `totp`, `password_totp`

---

### Подтверждение Email

```
POST /api/v1/confirm/code
Authorization: Bearer <jwt>
```

**Тело запроса:**
```json
{
  "code": "123456"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 5 запросов / 3 минуты

---

### Повторная отправка кода

```
POST /api/v1/resend/email
Authorization: Bearer <jwt>
```

**Rate Limit:** 1 запрос / 1 минута

---

### Обновление токенов

```
POST /api/v1/refresh
Cookie: refresh_token=<token>
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "access_token": "<new_access_token>"
  }
}
```

**Rate Limit:** 20 запросов / 1 минута

---

### Смена Email

```
POST /api/v1/hot/swap/email
Authorization: Bearer <jwt>
```

**Тело запроса:**
```json
{
  "email": "newemail@example.com"
}
```

**Rate Limit:** 3 запроса / 10 минут

---

### TOTP — Генерация

```
GET /api/v1/get/totp
Authorization: Bearer <jwt>  (strict: email verified)
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "totp_url": "otpauth://totp/..."
  }
}
```

**Rate Limit:** 3 запроса / 5 минут

---

### TOTP — Подтверждение

```
POST /api/v1/confirm/totp
Authorization: Bearer <jwt>  (strict: email verified)
```

**Тело запроса:**
```json
{
  "code": "123456"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "codes": ["code1", "code2", "..."]
  }
}
```

Возвращает коды восстановления TOTP.

**Rate Limit:** 5 запросов / 5 минут

---

### MFA Авторизация

Все MFA-эндпоинты требуют middleware `RequireMFAToken` (кука `auth_session_token` из эндпоинта `/login`).

```
POST /api/v1/auth/login/password
POST /api/v1/auth/login/password-email
POST /api/v1/auth/login/password-totp
POST /api/v1/auth/login/totp
```

**Тело запроса (password):**
```json
{
  "password": "securepassword"
}
```

**Тело запроса (password-email):**
```json
{
  "password": "securepassword",
  "code": "123456"
}
```

**Тело запроса (password-totp):**
```json
{
  "password": "securepassword",
  "code": "123456"
}
```

**Тело запроса (totp):**
```json
{
  "code": "123456"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 5 запросов / 1 минута

---

### SSO — Генерация токена

```
POST /api/v1/sso
Authorization: Bearer <jwt> (strict: email verified)
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "code": "<one_time_sso_token>"
  }
}
```

**Rate Limit:** 6 запросов / 1 минута

---

### S2S — Обмен SSO токена

```
POST /api/v1/sso/partner/exchange
X-Client-Secret: <api_key>
```

**Тело запроса:**
```json
{
  "token": "<sso_token>",
  "ip": "client_ip",
  "agent": "user_agent"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "PublicToken": "<access_token>",
    "RefreshToken": "<refresh_token>"
  }
}
```

**Rate Limit:** 500 запросов / 1 минута

---

### S2S — Обновление токена

```
POST /api/v1/sso/partner/refresh
X-Client-Secret: <api_key>
```

**Тело запроса:**
```json
{
  "refreshKey": "<refresh_token>",
  "ip": "client_ip",
  "agent": "user_agent"
}
```

**Ответ (200):**
```json
{
  "success": true,
  "data": {
    "PublicToken": "<access_token>",
    "RefreshToken": "<refresh_token>"
  }
}
```

**Rate Limit:** 500 запросов / 1 минута

---

## Системные эндпоинты

### Статус ядра

```
GET /api/v1/core-status
```

### Health / Metrics

```
GET /health
GET /metrics  (опционально с Basic Auth)
```

---

## Формат ошибок

Все ошибки возвращаются в едином формате:

```json
{
  "success": false,
  "error": {
    "code": 400,
    "message": "Неверный формат запроса",
    "details": "детали ошибки"
  }
}