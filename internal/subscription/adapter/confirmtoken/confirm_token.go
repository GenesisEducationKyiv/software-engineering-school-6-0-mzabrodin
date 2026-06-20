package confirmtoken

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrExpired = errors.New("confirmation token expired")
	ErrInvalid = errors.New("invalid confirmation token")
)

type jwtClaims struct {
	Repo string `json:"repo"`
	jwt.RegisteredClaims
}

type Tokenizer struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Tokenizer {
	return &Tokenizer{secret: []byte(secret), ttl: ttl}
}

func (t *Tokenizer) Issue(email, repo string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		Repo: repo,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("sign confirmation token: %w", err)
	}

	return signed, nil
}

func (t *Tokenizer) Verify(token string) (email, repo string, err error) {
	var claims jwtClaims

	if _, err = jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return t.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})); err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", ErrExpired
		}

		return "", "", fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	return claims.Subject, claims.Repo, nil
}
