package entity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

// tokenBytes is the number of random bytes per token; hex-encoded to 64 chars.
const tokenBytes = 32

func NewRepository(name string) Repository {
	return Repository{Name: name}
}

func NewSubscription(repositoryID uuid.UUID, email string) (Subscription, error) {
	unsubscribeToken, err := randomToken()
	if err != nil {
		return Subscription{}, fmt.Errorf("generate unsubscribe token: %w", err)
	}

	return Subscription{
		RepositoryID:     repositoryID,
		Email:            email,
		UnsubscribeToken: unsubscribeToken,
	}, nil
}

func randomToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	return hex.EncodeToString(b), nil
}
