package middleware

import (
	"2026_2_PinPals/internal/httpx"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

type TokenManager struct {
	secret []byte
	ttl    int64
}

func NewTokenManager(secret []byte, ttlSeconds int64) *TokenManager {
	return &TokenManager{secret: secret, ttl: ttlSeconds}
}

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func (tm *TokenManager) Issue(userID int) (string, error) {
	if userID < 1 {
		return "", errors.New("invalid user id")
	}

	now := jwt.NewNumericDate(time.Now())

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(userID),
			IssuedAt:  now,
			ExpiresAt: jwt.NewNumericDate(now.Time.Add(time.Duration(tm.ttl) * time.Second)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

func (tm *TokenManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			writeUnauthorized(w)
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
		if tokenString == "" {
			writeUnauthorized(w)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return tm.secret, nil
		})
		if err != nil || !token.Valid || claims.UserID < 1 {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	value, ok := ctx.Value(userIDKey).(int)
	return value, ok && value > 0
}

func writeUnauthorized(w http.ResponseWriter) {
	httpx.WriteError(
		w,
		http.StatusUnauthorized,
		"unauthorized",
	)
}
