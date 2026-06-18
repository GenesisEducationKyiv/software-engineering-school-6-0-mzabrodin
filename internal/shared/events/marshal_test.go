package events_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github-release-notifier/internal/shared/events"
)

type EventsSuite struct {
	suite.Suite
	sagaID string
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsSuite))
}

func (s *EventsSuite) SetupTest() {
	s.sagaID = uuid.NewString()
}

func (s *EventsSuite) TestRoundTrip() {
	pending := events.SubscriptionPending{
		SagaID:     s.sagaID,
		Email:      "user@example.com",
		RepoName:   "golang/go",
		ConfirmURL: "https://example.com/api/confirm/jwt",
	}

	data, err := events.Marshal(pending)
	s.Require().NoError(err)

	gotPending, err := events.Unmarshal[events.SubscriptionPending](data)
	s.Require().NoError(err)
	s.Equal(pending, gotPending)

	notified := events.ReleaseNotified{
		SagaID:       s.sagaID,
		RepoName:     "golang/go",
		Tag:          "v1.26.0",
		SentCount:    2,
		FailedEmails: []string{"a@example.com", "b@example.com"},
	}

	data, err = events.Marshal(notified)
	s.Require().NoError(err)

	gotNotified, err := events.Unmarshal[events.ReleaseNotified](data)
	s.Require().NoError(err)
	s.Equal(notified, gotNotified)

	noFailures := events.ReleaseNotified{
		SagaID:    s.sagaID,
		RepoName:  "golang/go",
		Tag:       "v1.26.0",
		SentCount: 0,
	}

	data, err = events.Marshal(noFailures)
	s.Require().NoError(err)
	s.NotContains(string(data), "failedEmails")

	gotNoFailures, err := events.Unmarshal[events.ReleaseNotified](data)
	s.Require().NoError(err)
	s.Equal(noFailures, gotNoFailures)
	s.Nil(gotNoFailures.FailedEmails)
}

func (s *EventsSuite) TestMarshalValidationRejects() {
	tests := map[string]events.SubscriptionPending{
		"missing saga id": {
			Email:      "user@example.com",
			RepoName:   "golang/go",
			ConfirmURL: "https://example.com/c",
		},
		"bad email": {
			SagaID:     s.sagaID,
			Email:      "not-an-email",
			RepoName:   "golang/go",
			ConfirmURL: "https://example.com/c",
		},
		"bad repo name": {
			SagaID:     s.sagaID,
			Email:      "user@example.com",
			RepoName:   "no-slash",
			ConfirmURL: "https://example.com/c",
		},
	}

	for name, evt := range tests {
		s.Run(name, func() {
			_, err := events.Marshal(evt)
			s.Require().Error(err)
		})
	}
}

func (s *EventsSuite) TestValidationErrorReportsJSONFieldName() {
	_, err := events.Marshal(events.SubscriptionPending{
		SagaID:     s.sagaID,
		Email:      "user@example.com",
		RepoName:   "no-slash",
		ConfirmURL: "https://example.com/c",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "repoName")
	s.NotContains(err.Error(), "RepoName")
}

func (s *EventsSuite) TestUnmarshalMalformedJSON() {
	_, err := events.Unmarshal[events.SubscriptionPending]([]byte("{not json"))
	s.Require().Error(err)
}

func (s *EventsSuite) TestUnmarshalInvalidAfterDecode() {
	_, err := events.Unmarshal[events.SubscriptionPending](
		[]byte(`{"sagaID":"` + s.sagaID + `","email":"x","repoName":"golang/go","confirmURL":"https://e/c"}`),
	)
	s.Require().Error(err)
}

func (s *EventsSuite) TestUnmarshalRejectsUnknownField() {
	_, err := events.Unmarshal[events.SubscriptionPending](
		[]byte(`{"sagaID":"` + s.sagaID + `","email":"user@example.com",` +
			`"repoName":"golang/go","confirmURL":"https://e/c","unexpected":"x"}`),
	)
	s.Require().Error(err)
}

func (s *EventsSuite) TestRepoNameValidator() {
	tests := map[string]struct {
		repo  string
		valid bool
	}{
		"owner/repo":              {"owner/repo", true},
		"dots dashes underscores": {"my-org_1/repo.name", true},
		"no slash":                {"no-slash", false},
		"empty repo":              {"owner/", false},
		"empty owner":             {"/repo", false},
		"too many segments":       {"owner/repo/extra", false},
	}

	for name, tc := range tests {
		s.Run(name, func() {
			_, err := events.Marshal(events.SubscriptionRemoved{
				SagaID:   s.sagaID,
				Email:    "user@example.com",
				RepoName: tc.repo,
			})
			if tc.valid {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}
