# API reference для фронтенда и LLM

Ниже собрана практическая спецификация HTTP API сервиса авторизации.

## Базовая информация

- Базовый URL: http://localhost:7910
- Версия API: /api/v1
- Формат тела: JSON
- Формат ответов:

```json
{
  "success": true,
  "data": { ... },
  "error": {
    "code": 400,
    "message": "Описание ошибки",
    "details": "..."
  }
}
```

- Для защищённых маршрутов нужен заголовок:

```http
Authorization: Bearer <jwt>
```

- Для MFA-потока используется временный токен второго этапа, который приходит на шаге логина и должен отправляться в том же заголовке `Authorization`.

- Обновление refresh-токена происходит через cookie `refresh_token`. В браузере обязательно используйте `credentials: 'include'`.

---

## 1. Регистрация и вход

### POST /api/v1/register
Создаёт нового пользователя.

Запрос:

```json
{
  "login": "alex",
  "email": "alex@example.com",
  "password": "strongPassword123"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

### POST /api/v1/login
Начинает вход по логину.

Запрос:

```json
{
  "login": "alex"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "authType": "password",
    "code": "<mfa_or_code_token>"
  }
}
```

Примечание: если метод авторизации `password_email`, сервис отправит код подтверждения на почту.

---

## 2. Подтверждение email

### POST /api/v1/resend/email
Повторно отправляет код подтверждения.

Требуется авторизация через обычный JWT.

Ответ:

```json
{
  "success": true,
  "data": {
    "message": "Новый код подтверждения отправлен на почту"
  }
}
```

### POST /api/v1/confirm/code
Подтверждает email кодом.

Требуется авторизация через обычный JWT.

Запрос:

```json
{
  "code": "123456"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

---

## 3. Refresh и выход из сессии

### POST /api/v1/refresh
Обновляет access-token и refresh-token.

Использует cookie `refresh_token`.

Ответ:

```json
{
  "success": true,
  "data": {
    "access_token": "<new_access_token>"
  }
}
```

---

## 4. Смена email

### POST /api/v1/hot/swap/email
Меняет email пользователя.

Требуется авторизация через обычный JWT.

Запрос:

```json
{
  "email": "new@example.com"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": ""
}
```

---

## 5. TOTP

### GET /api/v1/get/totp
Генерирует ссылку для подключения TOTP.

Требуется строгая авторизация (email должен быть подтверждён).

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "totp_url": "otpauth://totp/..."
  }
}
```

### POST /api/v1/confirm/totp
Подтверждает TOTP-код и возвращает backup-коды.

Требуется строгая авторизация.

Запрос:

```json
{
  "code": "123456"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "codes": ["12345678", "87654321"]
  }
}
```

---

## 6. MFA-поток входа

Все маршруты ниже требуют `Authorization: Bearer <mfa_session_token>`.

### POST /api/v1/auth/login/password

Запрос:

```json
{
  "password": "strongPassword123"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "jwt": "<access_token>"
  }
}
```

### POST /api/v1/auth/login/password-email

Запрос:

```json
{
  "password": "strongPassword123",
  "code": "123456"
}
```

### POST /api/v1/auth/login/password-totp

Запрос:

```json
{
  "password": "strongPassword123",
  "code": "123456"
}
```

### POST /api/v1/auth/login/totp

Запрос:

```json
{
  "code": "123456"
}
```

### POST /api/v1/auth/login/resend/email
Повторная отправка кода подтверждения в MFA-потоке.

---

## 7. SSO

### POST /api/v1/sso
Создаёт SSO-токен для текущего пользователя.

Требуется строгая авторизация.

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "code": "<sso_code>"
  }
}
```

### POST /api/v1/sso/partner/exchange
Внутренний обмен SSO-токена между сервисами.

Требуется API key.

Запрос:

```json
{
  "token": "<sso_token>",
  "ip": "127.0.0.1",
  "agent": "service-client"
}
```

Успешный ответ:

```json
{
  "success": true,
  "data": {
    "PublicToken": "<public_token>",
    "RefreshToken": "<refresh_token>"
  }
}
```

### POST /api/v1/sso/partner/refresh
Обновляет SSO refresh-токен.

Требуется API key.

Запрос:

```json
{
  "refreshKey": "<refresh_key>",
  "ip": "127.0.0.1",
  "agent": "service-client"
}
```

---

## 8. Системный маршрут

### GET /api/v1/core-status
Проверка доступности сервиса.

---

## 9. Рекомендации для фронтенда

- Используйте `fetch`/`axios` с `credentials: 'include'` для маршрутов, которые работают с `refresh_token`.
- Для защищённых маршрутов передавайте `Authorization: Bearer <token>`.
- Для ошибок ориентируйтесь на поле `error.message`.
- Все успешные ответы упакованы в формат `{ success, data }`.
- Для MFA‑сценария сначала вызовите `/login`, затем продолжайте по выбранному способу авторизации.

## 10. Краткая схема потока

1. `POST /register` → получить JWT
2. `POST /login` → получить временный MFA-токен / код
3. `POST /auth/login/...` → завершить аутентификацию и получить JWT
4. `POST /refresh` → обновить токены
