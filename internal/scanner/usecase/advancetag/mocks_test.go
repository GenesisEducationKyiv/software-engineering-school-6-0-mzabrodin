package advancetag

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type mockRepository struct{ mock.Mock }

func (m *mockRepository) AdvanceTag(ctx context.Context, repoName, tag string) error {
	return m.Called(ctx, repoName, tag).Error(0)
}
