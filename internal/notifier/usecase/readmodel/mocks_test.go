package readmodel

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type mockStore struct{ mock.Mock }

func (m *mockStore) Upsert(ctx context.Context, email, repoName, unsubToken string) error {
	return m.Called(ctx, email, repoName, unsubToken).Error(0)
}

func (m *mockStore) Delete(ctx context.Context, email, repoName string) error {
	return m.Called(ctx, email, repoName).Error(0)
}
