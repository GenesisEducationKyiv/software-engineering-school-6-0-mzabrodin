//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type SchemaSuite struct {
	suite.Suite
}

func TestSchemaSuite(t *testing.T) {
	suite.Run(t, new(SchemaSuite))
}

func (s *SchemaSuite) SetupTest() {
	truncateAll(s.T())
}

func (s *SchemaSuite) TestTablesExist() {
	for _, table := range []string{"repositories", "subscriptions"} {
		var exists bool
		row := testPool.QueryRow(s.T().Context(),
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)", table)
		s.Require().NoError(row.Scan(&exists))
		s.True(exists, "table %q should exist", table)
	}
}

func (s *SchemaSuite) TestUniqueConstraint_EmailRepository() {
	repoID := uuid.New()
	_, err := testPool.Exec(s.T().Context(),
		"INSERT INTO repositories (id, name) VALUES ($1, $2)", repoID, testRepoName)
	s.Require().NoError(err)

	_, err = testPool.Exec(s.T().Context(),
		`INSERT INTO subscriptions (repository_id, email, unsubscribe_token)
		 VALUES ($1, $2, $3)`,
		repoID, testEmail, "unsub-a"+uuid.NewString())
	s.Require().NoError(err)

	_, err = testPool.Exec(s.T().Context(),
		`INSERT INTO subscriptions (repository_id, email, unsubscribe_token)
		 VALUES ($1, $2, $3)`,
		repoID, testEmail, "unsub-b"+uuid.NewString())
	s.Error(err, "duplicate email+repository_id should violate unique constraint")
}

func (s *SchemaSuite) TestForeignKey_RepositoryID() {
	_, err := testPool.Exec(s.T().Context(),
		`INSERT INTO subscriptions (repository_id, email, unsubscribe_token)
		 VALUES ($1, $2, $3)`,
		uuid.New(), testEmail, uuid.NewString())
	s.Error(err, "non-existent repository_id should violate FK constraint")
}

func (s *SchemaSuite) TestCascadeDelete() {
	repoID := uuid.New()
	_, err := testPool.Exec(s.T().Context(),
		"INSERT INTO repositories (id, name) VALUES ($1, $2)", repoID, testRepoName)
	s.Require().NoError(err)

	_, err = testPool.Exec(s.T().Context(),
		`INSERT INTO subscriptions (repository_id, email, unsubscribe_token)
		 VALUES ($1, $2, $3)`,
		repoID, testEmail, uuid.NewString())
	s.Require().NoError(err)

	_, err = testPool.Exec(s.T().Context(),
		"DELETE FROM repositories WHERE id=$1", repoID)
	s.Require().NoError(err)

	var count int
	row := testPool.QueryRow(s.T().Context(),
		"SELECT COUNT(*) FROM subscriptions WHERE repository_id=$1", repoID)
	s.Require().NoError(row.Scan(&count))
	s.Zero(count, "subscriptions should be deleted when repository is deleted")
}
