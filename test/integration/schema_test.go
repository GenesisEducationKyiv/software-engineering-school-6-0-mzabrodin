//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchema_TablesExist verifies that migrations created the expected tables.
func TestSchema_TablesExist(t *testing.T) {
	for _, table := range []string{"repositories", "subscriptions"} {
		var exists bool
		row := testPool.QueryRow(context.Background(),
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)", table)
		require.NoError(t, row.Scan(&exists))
		assert.True(t, exists, "table %q should exist", table)
	}
}

// TestSchema_UniqueConstraint_EmailRepository verifies that subscribing the same
// email to the same repository twice violates the DB-level unique constraint.
func TestSchema_UniqueConstraint_EmailRepository(t *testing.T) {
	truncateAll(t)

	repoID := uuid.New()
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO repositories (id, name) VALUES ($1, $2)", repoID, testRepoName)
	require.NoError(t, err)

	_, err = testPool.Exec(context.Background(),
		`INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token)
		 VALUES ($1, $2, $3, $4)`,
		repoID, testEmail, "token-a"+uuid.NewString(), "unsub-a"+uuid.NewString())
	require.NoError(t, err)

	_, err = testPool.Exec(context.Background(),
		`INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token)
		 VALUES ($1, $2, $3, $4)`,
		repoID, testEmail, "token-b"+uuid.NewString(), "unsub-b"+uuid.NewString())
	assert.Error(t, err, "duplicate email+repository_id should violate unique constraint")
}

// TestSchema_ForeignKey_RepositoryID verifies that inserting a subscription with a
// non-existent repository_id violates the FK constraint.
func TestSchema_ForeignKey_RepositoryID(t *testing.T) {
	truncateAll(t)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token)
		 VALUES ($1, $2, $3, $4)`,
		uuid.New(), testEmail, uuid.NewString(), uuid.NewString())
	assert.Error(t, err, "non-existent repository_id should violate FK constraint")
}

// TestSchema_CascadeDelete verifies that deleting a repository removes its subscriptions.
func TestSchema_CascadeDelete(t *testing.T) {
	truncateAll(t)

	repoID := uuid.New()
	_, err := testPool.Exec(context.Background(),
		"INSERT INTO repositories (id, name) VALUES ($1, $2)", repoID, testRepoName)
	require.NoError(t, err)

	_, err = testPool.Exec(context.Background(),
		`INSERT INTO subscriptions (repository_id, email, confirm_token, unsubscribe_token)
		 VALUES ($1, $2, $3, $4)`,
		repoID, testEmail, uuid.NewString(), uuid.NewString())
	require.NoError(t, err)

	_, err = testPool.Exec(context.Background(),
		"DELETE FROM repositories WHERE id=$1", repoID)
	require.NoError(t, err)

	var count int
	row := testPool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM subscriptions WHERE repository_id=$1", repoID)
	require.NoError(t, row.Scan(&count))
	assert.Zero(t, count, "subscriptions should be deleted when repository is deleted")
}
