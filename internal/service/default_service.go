package service

import (
	"auth-service/core/pkg/security"
	"auth-service/internal/model"
	"auth-service/internal/repository"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.opentelemetry.io/otel"
	"golang.org/x/crypto/bcrypt"
)

type TokenEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(cryptoText string) (string, error)
}

type DefaultService struct {
	repo       *repository.DefaultRepository
	encryptor  TokenEncryptor
	jwtManager *security.JWTManager
	isProd     bool
}

func NewDefaultService(repo *repository.DefaultRepository, enc TokenEncryptor, jwtManager *security.JWTManager, isProd bool) *DefaultService {
	return &DefaultService{
		repo:       repo,
		encryptor:  enc,
		jwtManager: jwtManager,
		isProd:     isProd,
	}
}

var tracer = otel.Tracer("default-service")

// ____/\\\\\\\\\______/\\\\\\\\\\\\\\\_____/\\\\\\\\\\\\__/\\\\\\\\\\\_____/\\\\\\\\\\\____/\\\\\\\\\\\\\\\__/\\\\\\\\\\\\\\\____/\\\\\\\\\_____
//  __/\\\///////\\\___\/\\\///////////____/\\\//////////__\/////\\\///____/\\\/////////\\\_\///////\\\/////__\/\\\///////////___/\\\///////\\\___
//   _\/\\\_____\/\\\___\/\\\______________/\\\_________________\/\\\______\//\\\______\///________\/\\\_______\/\\\_____________\/\\\_____\/\\\___
//    _\/\\\\\\\\\\\/____\/\\\\\\\\\\\_____\/\\\____/\\\\\\\_____\/\\\_______\////\\\_______________\/\\\_______\/\\\\\\\\\\\_____\/\\\\\\\\\\\/____
//     _\/\\\//////\\\____\/\\\///////______\/\\\___\/////\\\_____\/\\\__________\////\\\____________\/\\\_______\/\\\///////______\/\\\//////\\\____
//      _\/\\\____\//\\\___\/\\\_____________\/\\\_______\/\\\_____\/\\\_____________\////\\\_________\/\\\_______\/\\\_____________\/\\\____\//\\\___
//       _\/\\\_____\//\\\__\/\\\_____________\/\\\_______\/\\\_____\/\\\______/\\\______\//\\\________\/\\\_______\/\\\_____________\/\\\_____\//\\\__
//        _\/\\\______\//\\\_\/\\\\\\\\\\\\\\\_\//\\\\\\\\\\\\/___/\\\\\\\\\\\_\///\\\\\\\\\\\/_________\/\\\_______\/\\\\\\\\\\\\\\\_\/\\\______\//\\\_
//         _\///________\///__\///////////////___\////////////____\///////////____\///////////___________\///________\///////////////__\///________\///__

func (s *DefaultService) Register(ctx context.Context, req *model.RegisterRequest, clientIP string, userAgent string) (*model.RegisterResponse, string, error) {
	ctx, span := tracer.Start(ctx, "service.Register")
	defer span.End()

	exist, err := s.repo.WatchExistUserData(ctx, req)

	if err != nil {
		return nil, "", fmt.Errorf("ошибка при проверке базы данных: %w", err)
	}

	if exist {
		return nil, "", fmt.Errorf("Логин или пароль уже заняты")
	}

	UUID, err := uuid.NewV7()
	sessionID, _ := uuid.NewV7()
	if err != nil {
		return nil, "", fmt.Errorf("Ошибка генерации UUID")
	}

	salt := 10
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), salt)

	code := generateNumericCode(6)
	if !s.isProd {
		slog.Debug("Generated code", slog.String("code", code))
	}

	now := time.Now()

	registerData := model.RepoRegisterResponse{
		UUID:         UUID.String(),
		Login:        req.Login,
		Email:        req.Email,
		HashPassword: string(hashedPassword),
		SessionID:    sessionID.String(),
		Code:         code,
		CreatedAt:    now.UnixMilli(),
	}

	err = s.repo.Register(ctx, registerData)
	if err != nil {
		return nil, "", err
	}

	expiresAt := now.Add(7 * 24 * time.Hour)

	// Собираем любую нужную дату для токена
	payload := map[string]interface{}{
		"email":      false,
		"role":       "user",
		"session_id": sessionID.String(),
		"platform":   "web",
	}

	refreshToken, err := s.jwtManager.GenerateToken(UUID.String(), 7*24*time.Hour, payload)
	if err != nil {
		return nil, "", err
	}

	session := model.SessionData{
		RefreshToken: refreshToken,
		IP:           clientIP,
		UserAgent:    userAgent,
		CreatedAt:    now.UnixMilli(),
		ExpiresAt:    expiresAt.UnixMilli(),
	}

	key := fmt.Sprintf("session:%s:%s", UUID.String(), sessionID.String())

	err = s.repo.NewSession(ctx, key, session)
	if err != nil {
		return nil, "", err
	}

	accessToken, err := s.jwtManager.GenerateToken(UUID.String(), 5*time.Minute, payload)
	if err != nil {
		return nil, "", err
	}

	user := &model.RegisterResponse{
		ID:          UUID.String(),
		Login:       req.Login,
		Email:       req.Email,
		AccessToken: accessToken,
	}

	return user, refreshToken, nil
}

func (s *DefaultService) ResendEmail(ctx context.Context, UUID string) error {
	ctx, span := tracer.Start(ctx, "service.ResendEmail")
	defer span.End()

	code := generateNumericCode(6)
	if !s.isProd {
		slog.Debug("Generated code", slog.String("code", code))
	}

	err := s.repo.SendEmailCode(ctx, UUID, code)
	if err != nil {
		return err
	}

	return nil
}

func (s *DefaultService) HotSwapEmail(ctx context.Context, UUID string, Email string) error {
	ctx, span := tracer.Start(ctx, "service.HotSwapEmail")
	defer span.End()

	code := generateNumericCode(6)
	err := s.repo.HotSwapEmail(ctx, UUID, Email, code)
	if err != nil {
		return err
	}

	return nil
}

func (s *DefaultService) ConfirmCode(ctx context.Context, code string, UUID string, ip string, userAgent string) (string, string, error) {
	ctx, span := tracer.Start(ctx, "service.ConfirmCode")
	defer span.End()

	err := s.repo.ConfirmCode(ctx, code, UUID)
	if err != nil {
		return "", "", fmt.Errorf("ошибка проверки кода email: %w", err)
	}

	err = s.repo.DelUsedCode(ctx, UUID)
	if err != nil {
		slog.Error("Не удалось удалить использованный код", slog.Any("error", err))
	}

	return createSession(ctx, s.repo, s.jwtManager, false, UUID, ip, userAgent)
}

// __/\\\\\\\\\\\\\\\_______/\\\\\_______/\\\\\\\\\\\\\\\__/\\\\\\\\\\\\\___
//  _\///////\\\/////______/\\\///\\\____\///////\\\/////__\/\\\/////////\\\_
//   _______\/\\\_________/\\\/__\///\\\________\/\\\_______\/\\\_______\/\\\_
//    _______\/\\\________/\\\______\//\\\_______\/\\\_______\/\\\\\\\\\\\\\/__
//     _______\/\\\_______\/\\\_______\/\\\_______\/\\\_______\/\\\/////////____
//      _______\/\\\_______\//\\\______/\\\________\/\\\_______\/\\\_____________
//       _______\/\\\________\///\\\__/\\\__________\/\\\_______\/\\\_____________
//        _______\/\\\__________\///\\\\\/___________\/\\\_______\/\\\_____________
//         _______\///_____________\/////_____________\///________\///______________

func (s *DefaultService) NewTOTP(ctx context.Context, UUID string) (string, error) {
	ctx, span := tracer.Start(ctx, "service.NewTOTP")
	defer span.End()

	login, err := s.repo.FetchTOTPData(ctx, UUID)
	if err != nil {
		return "", err
	}

	pureTfaSecret, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "https://kraito.ru",
		AccountName: login,
	})
	if err != nil {
		return "", errors.New("failed to generate secret TOTP")
	}

	totpURL := pureTfaSecret.URL()

	if !s.isProd {
		slog.Debug("TOTP URL", slog.String("url", totpURL))
	}
	tfaSecret := pureTfaSecret.Secret()

	encryptedSecret, err := s.encryptor.Encrypt(tfaSecret)
	if err != nil {
		return "", fmt.Errorf("encryption error: %w", err)
	}

	err = s.repo.CacheTOTP(ctx, UUID, encryptedSecret)
	if err != nil {
		return "", fmt.Errorf("ошибка сохранения кода: %w", err)
	}

	return totpURL, nil
}

func (s *DefaultService) ConfirmTOTP(ctx context.Context, UUID string, code string) ([]string, error) {
	ctx, span := tracer.Start(ctx, "service.ConfirmTOTP")
	defer span.End()

	encryptedSecret, err := s.repo.GetTOTPSecretCache(ctx, UUID)
	if err != nil {
		return nil, err
	}

	secret, err := s.encryptor.Decrypt(encryptedSecret)
	if err != nil {
		return nil, fmt.Errorf("не удалось расшифровать код: %w", err)
	}

	isValid := totp.Validate(code, secret)
	if !isValid {
		return nil, fmt.Errorf("invalid code")

	}

	plainCodes, err := GenerateBackupCodes(10, 8)
	if err != nil {
		return nil, err
	}

	hashedCodes, err := HashBackupCodes(plainCodes)
	if err != nil {
		return nil, err
	}

	jsonHashedCodes, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации хэшей: %w", err)
	}

	err = s.repo.SaveTOTPData(ctx, UUID, encryptedSecret, jsonHashedCodes)
	if err != nil {
		return nil, fmt.Errorf("ошибка сохранения данных")
	}

	return plainCodes, nil
}

// ____/\\\\\\\\\______/\\\\\\\\\\\\\\\__/\\\\\\\\\\\\\\\____/\\\\\\\\\______/\\\\\\\\\\\\\\\_____/\\\\\\\\\\\____/\\\________/\\\_
//  __/\\\///////\\\___\/\\\///////////__\/\\\///////////___/\\\///////\\\___\/\\\///////////____/\\\/////////\\\_\/\\\_______\/\\\_
//   _\/\\\_____\/\\\___\/\\\_____________\/\\\_____________\/\\\_____\/\\\___\/\\\______________\//\\\______\///__\/\\\_______\/\\\_
//    _\/\\\\\\\\\\\/____\/\\\\\\\\\\\_____\/\\\\\\\\\\\_____\/\\\\\\\\\\\/____\/\\\\\\\\\\\_______\////\\\_________\/\\\\\\\\\\\\\\\_
//     _\/\\\//////\\\____\/\\\///////______\/\\\///////______\/\\\//////\\\____\/\\\///////___________\////\\\______\/\\\/////////\\\_
//      _\/\\\____\//\\\___\/\\\_____________\/\\\_____________\/\\\____\//\\\___\/\\\_____________________\////\\\___\/\\\_______\/\\\_
//       _\/\\\_____\//\\\__\/\\\_____________\/\\\_____________\/\\\_____\//\\\__\/\\\______________/\\\______\//\\\__\/\\\_______\/\\\_
//        _\/\\\______\//\\\_\/\\\\\\\\\\\\\\\_\/\\\_____________\/\\\______\//\\\_\/\\\\\\\\\\\\\\\_\///\\\\\\\\\\\/___\/\\\_______\/\\\_
//         _\///________\///__\///////////////__\///______________\///________\///__\///////////////____\///////////_____\///________\///__

func (s *DefaultService) Refresh(
	ctx context.Context,
	refreshToken,
	ip,
	userAgent string,
) (string, string, error) {
	ctx, span := tracer.Start(ctx, "service.Refresh")
	defer span.End()

	claims, err := s.jwtManager.ParseAndValidate(refreshToken, "sub", "session_id")
	if err != nil {
		return "", "", err
	}

	// 2. Безопасно приводим типы
	UUID := claims["sub"].(string)
	sessionID := claims["session_id"].(string)

	// 3. Проверяем опциональное поле email (по умолчанию считаем true)
	emailVerified := true
	if emailClaim, exists := claims["email"].(bool); exists {
		emailVerified = emailClaim // Если в токене есть "email": false, переменная станет false
	}

	key := fmt.Sprintf("session:%s:%s", UUID, sessionID)

	exists, err := s.repo.ExistSession(ctx, key)
	if err != nil || exists == 0 {
		return "", "", fmt.Errorf("сессия не найдена или была завершена")
	}

	payload := map[string]interface{}{
		"role":       "user",
		"session_id": sessionID,
		"platform":   "web",
	}

	if !emailVerified {
		payload["email"] = false
	}

	newAccess, err := s.jwtManager.GenerateToken(UUID, 5*time.Minute, payload)
	if err != nil {
		return "", "", err
	}

	newRefresh, err := s.jwtManager.GenerateToken(UUID, 7*24*time.Hour, payload)
	if err != nil {
		return "", "", err
	}

	now := time.Now()

	newSessionData := model.SessionData{
		RefreshToken: newRefresh,
		IP:           ip,
		UserAgent:    userAgent,
		CreatedAt:    now.UnixMilli(),
		ExpiresAt:    now.Add(7 * 24 * time.Hour).UnixMilli(),
	}

	err = s.repo.NewSession(ctx, key, newSessionData)
	if err != nil {
		return "", "", err
	}

	return newAccess, newRefresh, nil
}

// __/\\\___________________/\\\\\__________/\\\\\\\\\\\\__/\\\\\\\\\\\__/\\\\\_____/\\\_
//  _\/\\\_________________/\\\///\\\______/\\\//////////__\/////\\\///__\/\\\\\\___\/\\\_
//   _\/\\\_______________/\\\/__\///\\\___/\\\_________________\/\\\_____\/\\\/\\\__\/\\\_
//    _\/\\\______________/\\\______\//\\\_\/\\\____/\\\\\\\_____\/\\\_____\/\\\//\\\_\/\\\_
//     _\/\\\_____________\/\\\_______\/\\\_\/\\\___\/////\\\_____\/\\\_____\/\\\\//\\\\/\\\_
//      _\/\\\_____________\//\\\______/\\\__\/\\\_______\/\\\_____\/\\\_____\/\\\_\//\\\/\\\_
//       _\/\\\______________\///\\\__/\\\____\/\\\_______\/\\\_____\/\\\_____\/\\\__\//\\\\\\_
//        _\/\\\\\\\\\\\\\\\____\///\\\\\/_____\//\\\\\\\\\\\\/___/\\\\\\\\\\\_\/\\\___\//\\\\\_
//         _\///////////////_______\/////________\////////////____\///////////__\///_____\/////__

func (s *DefaultService) Login(ctx context.Context, login string) (string, string, error) {
	ctx, span := tracer.Start(ctx, "service.Login")
	defer span.End()

	UUID, authType, err := s.repo.Login(ctx, login)
	if err != nil {
		return "", "", err
	}

	payload := map[string]interface{}{
		"type": "mfa_session",
	}

	token, err := s.jwtManager.GenerateToken(UUID, 20*time.Minute, payload)
	if err != nil {
		return "", "", err
	}

	if authType == "AuthMethodPasswordEmail" {
		code := generateNumericCode(6)

		err = s.repo.SendEmailCode(ctx, UUID, code)
		if err != nil {
			return "", "", err
		}
	}

	return token, authType, nil
}

func (s *DefaultService) LoginAuthMethodPassword(ctx context.Context, UUID string, password string, ip string, userAgent string) (string, string, error) {
	ctx, span := tracer.Start(ctx, "service.LoginAuthMethodPassword")
	defer span.End()

	data, err := s.repo.GetAuthData(ctx, UUID, "")
	emailLock := false
	if err != nil {
		return "", "", err
	}

	if data.AuthMethod != "password" {
		return "", "", fmt.Errorf("неверный метод авторизации")
	}

	if data.EmailVerifiedAt == nil {
		emailLock = true
	}

	err = bcrypt.CompareHashAndPassword([]byte(data.PasswordHash), []byte(password))
	if err != nil {

		return "", "", fmt.Errorf("пароль не верен: %w", err)
	}

	return createSession(ctx, s.repo, s.jwtManager, emailLock, UUID, ip, userAgent)
}

func (s *DefaultService) LoginAuthMethodPasswordEmail(
	ctx context.Context,
	UUID string,
	password string,
	code string,
	ip string,
	userAgent string,
) (string, string, error) {
	ctx, span := tracer.Start(ctx, "service.LoginAuthMethodPasswordEmail")
	defer span.End()

	data, err := s.repo.GetAuthData(ctx, UUID, "AuthMethodPasswordEmail")
	if err != nil {
		return "", "", err
	}

	if data.AuthMethod != "password_email" {
		return "", "", fmt.Errorf("неверный метод авторизации")
	}

	if data.CurrentCode != code {
		return "", "", fmt.Errorf("неверный код подтверждения")
	}

	err = bcrypt.CompareHashAndPassword([]byte(data.PasswordHash), []byte(password))
	if err != nil {
		return "", "", fmt.Errorf("пароль не верен: %w", err)
	}

	err = s.repo.DelUsedCode(ctx, UUID)
	if err != nil {
		slog.Warn("Не удалось удалить использованный код", slog.Any("error", err))
	}

	return createSession(ctx, s.repo, s.jwtManager, false, UUID, ip, userAgent)
}

func (s *DefaultService) LoginAuthMethodPasswordTOTP(ctx context.Context,
	UUID string,
	password string,
	code string,
	ip string,
	userAgent string,
) (string, string, error) {
	data, err := s.repo.GetAuthData(ctx, UUID, "")
	if err != nil {
		return "", "", err
	}

	if data.AuthMethod != "password_totp" {
		return "", "", fmt.Errorf("неверный метод авторизации")
	}

	if data.TOTPSecretEncrypted == nil {
		return "", "", fmt.Errorf("ошибка: секрет TOTP не найден")
	}
	tfa_secret, err := s.encryptor.Decrypt(*data.TOTPSecretEncrypted)
	if err != nil {
		return "", "", err
	}

	isValid := totp.Validate(code, tfa_secret)
	if !isValid {
		return "", "", fmt.Errorf("invalid code")

	}

	err = bcrypt.CompareHashAndPassword([]byte(data.PasswordHash), []byte(password))
	if err != nil {
		return "", "", fmt.Errorf("пароль не верен: %w", err)
	}

	return createSession(ctx, s.repo, s.jwtManager, false, UUID, ip, userAgent)
}

func (s *DefaultService) LoginAuthMethodTOTP(
	ctx context.Context,
	UUID string,
	code string,
	ip string,
	userAgent string,
) (string, string, error) {
	data, err := s.repo.GetAuthData(ctx, UUID, "")
	if err != nil {
		return "", "", err
	}

	if data.AuthMethod != "totp" {
		return "", "", fmt.Errorf("неверный метод авторизации")
	}

	if data.TOTPSecretEncrypted == nil {
		return "", "", fmt.Errorf("ошибка: секрет TOTP не найден")
	}
	tfa_secret, err := s.encryptor.Decrypt(*data.TOTPSecretEncrypted)
	if err != nil {
		return "", "", err
	}

	isValid := totp.Validate(code, tfa_secret)
	if !isValid {
		return "", "", fmt.Errorf("invalid code")
	}

	return createSession(ctx, s.repo, s.jwtManager, false, UUID, ip, userAgent)
}

// _____/\\\\\\\\\\\______/\\\\\\\\\_________/\\\\\\\\\\\___
//  ___/\\\/////////\\\__/\\\///////\\\_____/\\\/////////\\\_
//   __\//\\\______\///__\///______\//\\\___\//\\\______\///__
//    ___\////\\\___________________/\\\/_____\////\\\_________
//     ______\////\\\_____________/\\\//__________\////\\\______
//      _________\////\\\_______/\\\//________________\////\\\___
//       __/\\\______\//\\\____/\\\/____________/\\\______\//\\\__
//        _\///\\\\\\\\\\\/____/\\\\\\\\\\\\\\\_\///\\\\\\\\\\\/___
//         ___\///////////_____\///////////////____\///////////_____

// Создание SSO токена
func (s *DefaultService) PostSSOToken(ctx context.Context, UUID string) (string, error) {
	token := uuid.New().String()

	err := s.repo.PostSSOToken(ctx, UUID, token)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить токен: %w", err)
	}

	return token, nil
}

// Обмен SSO токена на jwt сессиию
func (s *DefaultService) GetSSOToken(ctx context.Context, token string, ip string, userAgent string) (string, string, error) {
	UUID, err := s.repo.GetSSOToken(ctx, token)
	if err != nil {
		return "", "", fmt.Errorf("не удалось получить сессию по токену: %w", err)
	}

	return createSession(ctx, s.repo, s.jwtManager, false, UUID, ip, userAgent)
}

// Регистрация API ключа
func (s *DefaultService) RegisterPartner(ctx context.Context, UUID string, req model.SRequest2GetAPI) (string, error) {
	origin, err := NormalizeOrigin(req.URL)
	if err != nil {
		return "", err
	}
	req.URL = origin

	rawAPIKey := "sk_live_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	hashedKey := HashAPIKey(rawAPIKey)

	payload := struct {
		URL          string `json:"url"`
		OwnerUUID    string `json:"owner_uuid"`
		CompanyName  string `json:"company_name,omitempty"`
		FullName     string `json:"full_name"`
		CompanyEmail string `json:"company_email,omitempty"`
		TokenHash    string `json:"token_hash"`
	}{
		URL:          req.URL,
		OwnerUUID:    UUID,
		CompanyName:  req.CompanyName,
		FullName:     req.FullName,
		CompanyEmail: req.CompanyEmail,
		TokenHash:    hashedKey,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации данных партнера: %w", err)
	}

	err = s.repo.CreatePartner(ctx, req.UUID, origin, hashedKey, jsonData)
	if err != nil {
		return "", fmt.Errorf("ошибка создания партнера в БД: %w", err)
	}

	return rawAPIKey, nil
}

// ==================
//  Функции настроек
// ==================

// ===================
//	Внутрение функции
// ===================

func createSession(
	ctx context.Context,
	repo *repository.DefaultRepository,
	jwtManager *security.JWTManager,
	emailLock bool,
	UUID string,
	ip string,
	userAgent string,
) (string, string, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return "", "", err
	}

	payload := map[string]interface{}{
		"role":       "user",
		"session_id": sessionID.String(),
		"platform":   "web",
	}

	if emailLock {
		payload["email"] = false
	}

	refreshToken, err := jwtManager.GenerateToken(UUID, 7*24*time.Hour, payload)
	if err != nil {
		return "", "", err
	}

	accessToken, err := jwtManager.GenerateToken(UUID, 5*time.Minute, payload)
	if err != nil {
		return "", "", err
	}

	now := time.Now()
	newSessionData := model.SessionData{
		RefreshToken: refreshToken,
		IP:           ip,
		UserAgent:    userAgent,
		CreatedAt:    now.UnixMilli(),
		ExpiresAt:    now.Add(7 * 24 * time.Hour).UnixMilli(),
	}

	key := fmt.Sprintf("session:%s:%s", UUID, sessionID.String())
	err = repo.NewSession(ctx, key, newSessionData)
	if err != nil {
		return "", "", err
	}

	return refreshToken, accessToken, nil
}

func generateNumericCode(length int) string {
	digits := "0123456789"
	code := make([]byte, length)
	rand.Read(code)
	for i := 0; i < length; i++ {
		code[i] = digits[int(code[i])%len(digits)]
	}
	return string(code)
}

func GenerateBackupCodes(count int, length int) ([]string, error) {
	const backupCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	codes := make([]string, count)

	for i := 0; i < count; i++ {
		bytes := make([]byte, length)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("ошибка криптографического генератора: %w", err)
		}

		// Превращаем случайные байты в символы из нашего алфавита
		for j := 0; j < length; j++ {
			bytes[j] = backupCodeCharset[int(bytes[j])%len(backupCodeCharset)]
		}

		if length == 8 {
			codes[i] = string(bytes[:4]) + "-" + string(bytes[4:])
		} else {
			codes[i] = string(bytes)
		}
	}

	return codes, nil
}

func HashBackupCodes(plainCodes []string) ([]string, error) {
	// Создаем срез нужной длины сразу, чтобы избежать лишних аллокаций памяти
	hashedCodes := make([]string, 0, len(plainCodes))

	for _, code := range plainCodes {
		// bcrypt.DefaultCost (обычно 10) — это отличный баланс между безопасностью и скоростью
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("ошибка хэширования кода: %w", err)
		}

		hashedCodes = append(hashedCodes, string(hashBytes))
	}

	return hashedCodes, nil
}

func NormalizeOrigin(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	// Проверяем, есть ли протокол. Если юзер ввел просто "shop.ru", подставляем https://
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("неверный формат URL: %w", err)
	}

	// u.Scheme - это http или https
	// u.Host - это домен (и порт, если он есть), например, shop.ru или localhost:3000
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

func HashAPIKey(apiKey string) string {
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	return hex.EncodeToString(hasher.Sum(nil))
}
