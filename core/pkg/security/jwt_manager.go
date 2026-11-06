package security

import (
	"auth-service/internal/model"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
	accessTTL  time.Duration
}

// UserClaims — структура, которую мы будем вшивать в токен
type UserClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewJWTManager(privateKeyPath, publicKeyPath, keyID string, ttlMinutes int) (*JWTManager, error) {
	if keyID == "" {
		keyID = "default-key-1"
	}

	manager := &JWTManager{
		keyID:     keyID,
		accessTTL: time.Duration(ttlMinutes) * time.Minute,
	}

	if privateKeyPath != "" {
		keyBytes, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения приватного ключа: %w", err)
		}
		manager.privateKey, err = jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга приватного ключа: %w", err)
		}
		manager.publicKey = &manager.privateKey.PublicKey
	}

	if publicKeyPath != "" {
		keyBytes, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения публичного ключа: %w", err)
		}
		manager.publicKey, err = jwt.ParseRSAPublicKeyFromPEM(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга публичного ключа: %w", err)
		}
	}

	if manager.privateKey == nil && manager.publicKey == nil {
		return nil, fmt.Errorf("необходимо указать хотя бы один ключ (private или public)")
	}

	return manager, nil
}

// GenerateToken генерирует токен с любыми кастомными полями
func (m *JWTManager) GenerateToken(userID string, ttl time.Duration, customClaims map[string]interface{}) (string, error) {
	// 1. Создаем базовые обязательные поля (Registered Claims)
	claims := jwt.MapClaims{
		"sub": userID,                     // Subject (ID юзера)
		"iat": time.Now().Unix(),          // Issued At (Когда выдан)
		"exp": time.Now().Add(ttl).Unix(), // Expiration Time (Годен до)
	}

	// 2. Динамически вливаем все кастомные поля, которые передал микросервис
	for key, value := range customClaims {
		claims[key] = value
	}

	// 3. Создаем и подписываем токен
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	token.Header["kid"] = m.keyID
	// m.privateKey — это твой ключ из RSA или строка секрета для HS256
	return token.SignedString(m.privateKey)
}

// ValidateToken проверяет токен (Для всех сервисов)
func (m *JWTManager) ValidateToken(tokenString string) (*UserClaims, error) {
	if m.publicKey == nil {
		return nil, fmt.Errorf("публичный ключ не загружен, валидация невозможна")
	}

	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return m.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка валидации токена: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("недействительный токен")
	}

	return claims, nil
}

func (m *JWTManager) ParseAndValidate(tokenString string, requiredFields ...string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return m.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка валидации токена: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("недействительный токен")
	}

	for _, field := range requiredFields {
		if _, exists := claims[field]; !exists {
			return nil, fmt.Errorf("в токене отсутствует обязательное поле: %s", field)
		}
	}

	return claims, nil
}

// PublicKey возвращает *rsa.PublicKey (если нужен сырой ключ)
func (m *JWTManager) PublicKey() *rsa.PublicKey {
	return m.publicKey
}

// GetJWKS формирует структуру JWKS из публичного RSA-ключа для /.well-known/jwks.json
func (m *JWTManager) GetJWKS() (model.JWKS, error) {
	if m.publicKey == nil {
		return model.JWKS{}, fmt.Errorf("публичный ключ не инициализирован")
	}

	// Извлекаем модуль (N) и экспоненту (E) в формате Big-Endian байтов
	nBytes := m.publicKey.N.Bytes()
	eBytes := big.NewInt(int64(m.publicKey.E)).Bytes()

	jwk := model.JWK{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: m.keyID,
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}

	return model.JWKS{
		Keys: []model.JWK{jwk},
	}, nil
}
