package repository

import "github-release-notifier/internal/notifier/domain"

type recipientRow struct {
	Email      string `db:"email"`
	UnsubToken string `db:"unsub_token"`
}

func (row recipientRow) toDomain() domain.Recipient {
	return domain.Recipient{
		Email:      row.Email,
		UnsubToken: row.UnsubToken,
	}
}

type failedNotificationRow struct {
	ID         int64  `db:"id"`
	SagaID     string `db:"saga_id"`
	RepoName   string `db:"repo_name"`
	Tag        string `db:"tag"`
	ReleaseURL string `db:"release_url"`
	Email      string `db:"email"`
	Reason     string `db:"reason"`
	RetryCount int    `db:"retry_count"`
}

func (row failedNotificationRow) toDomain() domain.FailedNotification {
	return domain.FailedNotification{
		ID:         row.ID,
		SagaID:     row.SagaID,
		RepoName:   row.RepoName,
		Tag:        row.Tag,
		ReleaseURL: row.ReleaseURL,
		Email:      row.Email,
		Reason:     row.Reason,
		RetryCount: row.RetryCount,
	}
}

type failedConfirmationRow struct {
	ID         int64  `db:"id"`
	SagaID     string `db:"saga_id"`
	Email      string `db:"email"`
	RepoName   string `db:"repo_name"`
	ConfirmURL string `db:"confirm_url"`
	Reason     string `db:"reason"`
	RetryCount int    `db:"retry_count"`
}

func (row failedConfirmationRow) toDomain() domain.FailedConfirmation {
	return domain.FailedConfirmation{
		ID:         row.ID,
		SagaID:     row.SagaID,
		Email:      row.Email,
		RepoName:   row.RepoName,
		ConfirmURL: row.ConfirmURL,
		Reason:     row.Reason,
		RetryCount: row.RetryCount,
	}
}

func toRecipients(rows []recipientRow) []domain.Recipient {
	recipients := make([]domain.Recipient, 0, len(rows))
	for _, row := range rows {
		recipients = append(recipients, row.toDomain())
	}

	return recipients
}

func toFailedNotifications(rows []failedNotificationRow) []domain.FailedNotification {
	failed := make([]domain.FailedNotification, 0, len(rows))
	for _, row := range rows {
		failed = append(failed, row.toDomain())
	}

	return failed
}

func toFailedConfirmations(rows []failedConfirmationRow) []domain.FailedConfirmation {
	failed := make([]domain.FailedConfirmation, 0, len(rows))
	for _, row := range rows {
		failed = append(failed, row.toDomain())
	}

	return failed
}
