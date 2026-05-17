package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepo_Valid(t *testing.T) {
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
		owner, name, err := ParseRepo(tc.input)
		require.NoError(t, err, "ParseRepo(%q)", tc.input)
		assert.Equal(t, tc.wantOwner, owner, "ParseRepo(%q) owner", tc.input)
		assert.Equal(t, tc.wantName, name, "ParseRepo(%q) name", tc.input)
	}
}

func TestParseRepo_Invalid(t *testing.T) {
	cases := []string{
		"",
		"noslash",
		"/repo",
		"owner/",
		"owner/repo/extra",
		"owner repo",
	}

	for _, tc := range cases {
		_, _, err := ParseRepo(tc)
		assert.ErrorIs(t, err, ErrInvalidRepo, "ParseRepo(%q)", tc)
	}
}
