# API Reference

All endpoints are available at the base path `/api/v1`.

## Authentication

Most endpoints require the `Authorization: Bearer <jwt_token>` header (middleware `RequireAuth` / `RequireStrictAuth`).

## Endpoints

### Registration

```
POST /api/v1/register
```

**Request Body:**
```json
{
  "login": "username",
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 3 requests / 10 minutes per IP

**Cookies:** Sets `refresh_token` (HttpOnly, 1 hour)

---

### Login (determine auth method)

```
POST /api/v1/login
```

**Request Body:**
```json
{
  "login": "username"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "authType": "password"
  }
}
```

**Cookies:** Sets `auth_session_token` (HttpOnly, 20 minutes)

**Rate Limit:** 5 requests / 1 minute per IP

Possible `authType` values: `password`, `password_email`, `totp`, `password_totp`

---

### Confirm Email

```
POST /api/v1/confirm/code
Authorization: Bearer <jwt>
```

**Request Body:**
```json
{
  "code": "123456"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 5 requests / 3 minutes

---

### Resend Code

```
POST /api/v1/resend/email
Authorization: Bearer <jwt>
```

**Rate Limit:** 1 request / 1 minute

---

### Token Refresh

```
POST /api/v1/refresh
Cookie: refresh_token=<token>
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "access_token": "<new_access_token>"
  }
}
```

**Rate Limit:** 20 requests / 1 minute

---

### Change Email

```
POST /api/v1/hot/swap/email
Authorization: Bearer <jwt>
```

**Request Body:**
```json
{
  "email": "newemail@example.com"
}
```

**Rate Limit:** 3 requests / 10 minutes

---

### TOTP — Generate

```
GET /api/v1/get/totp
Authorization: Bearer <jwt>  (strict: email verified)
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "totp_url": "otpauth://totp/..."
  }
}
```

**Rate Limit:** 3 requests / 5 minutes

---

### TOTP — Confirm

```
POST /api/v1/confirm/totp
Authorization: Bearer <jwt>  (strict: email verified)
```

**Request Body:**
```json
{
  "code": "123456"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "codes": ["code1", "code2", "..."]
  }
}
```

Returns TOTP recovery codes.

**Rate Limit:** 5 requests / 5 minutes

---

### MFA Authorization

All MFA endpoints require the `RequireMFAToken` middleware (`auth_session_token` cookie from the `/login` endpoint).

```
POST /api/v1/auth/login/password
POST /api/v1/auth/login/password-email
POST /api/v1/auth/login/password-totp
POST /api/v1/auth/login/totp
```

**Request Body (password):**
```json
{
  "password": "securepassword"
}
```

**Request Body (password-email):**
```json
{
  "password": "securepassword",
  "code": "123456"
}
```

**Request Body (password-totp):**
```json
{
  "password": "securepassword",
  "code": "123456"
}
```

**Request Body (totp):**
```json
{
  "code": "123456"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

**Rate Limit:** 5 requests / 1 minute

---

### SSO — Generate Token

```
POST /api/v1/sso
Authorization: Bearer <jwt> (strict: email verified)
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "code": "<one_time_sso_token>"
  }
}
```

**Rate Limit:** 6 requests / 1 minute

---

### S2S — Exchange SSO Token

```
POST /api/v1/sso/partner/exchange
X-Client-Secret: <api_key>
```

**Request Body:**
```json
{
  "token": "<sso_token>",
  "ip": "client_ip",
  "agent": "user_agent"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "PublicToken": "<access_token>",
    "RefreshToken": "<refresh_token>"
  }
}
```

**Rate Limit:** 500 requests / 1 minute

---

### S2S — Refresh Token

```
POST /api/v1/sso/partner/refresh
X-Client-Secret: <api_key>
```

**Request Body:**
```json
{
  "refreshKey": "<refresh_token>",
  "ip": "client_ip",
  "agent": "user_agent"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "PublicToken": "<access_token>",
    "RefreshToken": "<refresh_token>"
  }
}
```

**Rate Limit:** 500 requests / 1 minute

---

## System Endpoints

### Core Status

```
GET /api/v1/core-status
```

### Health / Metrics

```
GET /health
GET /metrics  (optionally with Basic Auth)
```

---

## Error Format

All errors follow a unified format:

```json
{
  "success": false,
  "error": {
    "code": 400,
    "message": "Invalid request format",
    "details": "error details"
  }
}