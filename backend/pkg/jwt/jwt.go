// Package jwt provides HS256 JWT generation and validation for SocialSentry.
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Claims is the set of fields embedded in every SocialSentry access token.
type Claims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	jwtv5.RegisteredClaims
}

// Generate returns a signed HS256 JWT for the given user.
func Generate(userID, role string, ttl time.Duration, secret []byte) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("jwt.Generate: secret is empty")
	}

	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwtv5.NewNumericDate(now),
			NotBefore: jwtv5.NewNumericDate(now),
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("jwt.Generate: %w", err)
	}
	return signed, nil
}

// Parse validates the JWT signature and expiration, returning the embedded claims.
func Parse(tokenString string, secret []byte) (*Claims, error) {
	if len(secret) == 0 {
		return nil, errors.New("jwt.Parse: secret is empty")
	}

	parsed, err := jwtv5.ParseWithClaims(tokenString, &Claims{}, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt.Parse: unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt.Parse: %w", err)
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("jwt.Parse: invalid token")
	}
	return claims, nil
}
