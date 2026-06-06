package entity_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/entity"
)

type ParseRepoSuite struct {
	suite.Suite
}

func TestParseRepoSuite(t *testing.T) {
	suite.Run(t, new(ParseRepoSuite))
}

func (s *ParseRepoSuite) TestValid() {
	cases := []struct {
		input     string
		wantOwner string
		wantName  string
	}{
		{"golang/go", "golang", "go"},
		{"owner/repo-name", "owner", "repo-name"},
		{"My.Org/My_Repo.123", "My.Org", "My_Repo.123"},
	}

	for _, tc := range cases {
		s.Run(tc.input, func() {
			owner, name, err := entity.ParseRepo(tc.input)
			s.Require().NoError(err)
			s.Equal(tc.wantOwner, owner)
			s.Equal(tc.wantName, name)
		})
	}
}

func (s *ParseRepoSuite) TestInvalid() {
	cases := []string{
		"",
		"noslash",
		"/repo",
		"owner/",
		"owner/repo/extra",
		"owner repo",
	}

	for _, tc := range cases {
		s.Run(tc, func() {
			_, _, err := entity.ParseRepo(tc)
			s.ErrorIs(err, entity.ErrInvalidRepo)
		})
	}
}
