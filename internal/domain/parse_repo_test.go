package domain

import (
	"errors"
	"testing"
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
		if err != nil {
			t.Errorf("ParseRepo(%q) unexpected error: %v", tc.input, err)
			continue
		}

		if owner != tc.wantOwner || name != tc.wantName {
			t.Errorf("ParseRepo(%q) = (%q, %q), want (%q, %q)", tc.input, owner, name, tc.wantOwner, tc.wantName)
		}
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
		if !errors.Is(err, ErrInvalidRepo) {
			t.Errorf("ParseRepo(%q) = %v, want ErrInvalidRepo", tc, err)
		}
	}
}
