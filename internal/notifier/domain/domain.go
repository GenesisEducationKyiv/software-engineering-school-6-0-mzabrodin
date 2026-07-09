package domain

type Recipient struct {
	Email      string
	UnsubToken string
}

type FailedNotification struct {
	ID         int64
	SagaID     string
	RepoName   string
	Tag        string
	ReleaseURL string
	Email      string
	Reason     string
	RetryCount int
}

type FailedConfirmation struct {
	ID         int64
	SagaID     string
	Email      string
	RepoName   string
	ConfirmURL string
	Reason     string
	RetryCount int
}
