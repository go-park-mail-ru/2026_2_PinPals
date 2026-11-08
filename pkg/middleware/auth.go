package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"2026_2_PinPals/pkg/httpx"
	"github.com/golang-jwt/jwt/v5"
)

const AccessTokenCookieName = "access_token"
const CSRFTokenCookieName = "csrf_token"
const CSRFHeaderName = "X-CSRF-Token"

type contextKey string

const userIDKey contextKey = "userID"

type RevocationStore interface {
	Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}

// UserStatusChecker — необязательное расширение RevocationStore: если хранилище его
// реализует, токены удалённых (soft delete) пользователей отклоняются во всех сервисах.
type UserStatusChecker interface {
	IsUserDeleted(ctx context.Context, userID int) (bool, error)
}

type TokenManager struct {
	secret []byte
	ttl    int64
	store  RevocationStore
}

func NewTokenManager(secret []byte, ttlSeconds int64) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttlSeconds}
}

func (tm *TokenManager) SetRevocationStore(store RevocationStore) { tm.store = store }

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func (tm *TokenManager) Issue(userID int) (string, error) {
	if userID < 1 {
		return "", errors.New("invalid user id")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	now := jwt.NewNumericDate(time.Now())
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(id[:]),
			Subject:   strconv.Itoa(userID),
			IssuedAt:  now,
			ExpiresAt: jwt.NewNumericDate(now.Time.Add(time.Duration(tm.ttl) * time.Second)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
}

// TokenFromRequest извлекает токен из HttpOnly cookie; Bearer оставлен для обратной совместимости.
func TokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(AccessTokenCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return strings.TrimSpace(cookie.Value)
	}
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}

func (tm *TokenManager) parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil || token == nil || !token.Valid || claims.UserID < 1 || claims.ID == "" || claims.ExpiresAt == nil {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// rejected сообщает, что токен нельзя принимать: он отозван, ошибка хранилища
// или пользователь удалён.
func (tm *TokenManager) rejected(ctx context.Context, claims *Claims) bool {
	if tm.store == nil {
		return false
	}
	revoked, err := tm.store.IsRevoked(ctx, claims.ID)
	if err != nil || revoked {
		return true
	}
	if checker, ok := tm.store.(UserStatusChecker); ok {
		deleted, err := checker.IsUserDeleted(ctx, claims.UserID)
		if err != nil || deleted {
			return true
		}
	}
	return false
}

func (tm *TokenManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := TokenFromRequest(r)
		if tokenString == "" {
			writeUnauthorized(w)
			return
		}
		claims, err := tm.parse(tokenString)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		if tm.rejected(r.Context(), claims) {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalMiddleware разрешает анонимный доступ, но проверяет подпись и отзыв токена,
// если клиент передал cookie или Bearer-токен.
func (tm *TokenManager) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := TokenFromRequest(r)
		if tokenString == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := tm.parse(tokenString)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		if tm.rejected(r.Context(), claims) {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Revoke отзывает JWT на сервере; сама очистка cookie не заменяет этот вызов.
func (tm *TokenManager) Revoke(ctx context.Context, tokenString string) error {
	if tm.store == nil {
		return errors.New("token revocation store is not configured")
	}
	claims, err := tm.parse(tokenString)
	if err != nil {
		return err
	}
	return tm.store.Revoke(ctx, claims.ID, claims.ExpiresAt.Time)
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	value, ok := ctx.Value(userIDKey).(int)
	return value, ok && value > 0
}

func writeUnauthorized(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusUnauthorized, "не авторизован")
}

// NewCSRFToken создаёт случайный токен для double-submit CSRF-защиты.
func NewCSRFToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func SetAuthCookies(w http.ResponseWriter, accessToken, csrfToken string, ttl time.Duration, secure bool) {
	maxAge := int(ttl.Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: AccessTokenCookieName, Value: accessToken, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: CSRFTokenCookieName, Value: csrfToken, Path: "/", MaxAge: maxAge, HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func ClearAuthCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{AccessTokenCookieName, CSRFTokenCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: name == AccessTokenCookieName, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
}

// CSRF проверяет double-submit токен для всех изменяющих состояние запросов с access cookie.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie(AccessTokenCookieName); err != nil {
			next.ServeHTTP(w, r)
			return
		}
		csrfCookie, err := r.Cookie(CSRFTokenCookieName)
		if err != nil || csrfCookie.Value == "" {
			httpx.WriteError(w, http.StatusForbidden, "отсутствует CSRF-токен")
			return
		}
		header := r.Header.Get(CSRFHeaderName)
		if len(header) != len(csrfCookie.Value) || subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 {
			httpx.WriteError(w, http.StatusForbidden, "неверный CSRF-токен")
			return
		}
		next.ServeHTTP(w, r)
	})
}
