package events

const (
	StreamSubscriptions = "SUBSCRIPTIONS"
	StreamReleases      = "RELEASES"
	StreamNotifications = "NOTIFICATIONS"
)

const (
	SubjectSubscriptionPending   = "subscriptions.pending"
	SubjectSubscriptionConfirmed = "subscriptions.confirmed"
	SubjectSubscriptionRemoved   = "subscriptions.removed"
	SubjectSubscriptionExpired   = "subscriptions.expired"
)

const (
	SubjectReleaseDetected = "releases.detected"
	SubjectReleaseNotified = "releases.notified"
)

const (
	SubjectNotificationConfirmationSent   = "notifications.confirmation.sent"
	SubjectNotificationConfirmationFailed = "notifications.confirmation.failed"
	SubjectNotificationReleaseSent        = "notifications.release.sent"
	SubjectNotificationReleaseFailed      = "notifications.release.failed"
	SubjectNotificationReleaseDead        = "notifications.release.dead"
)

var SubjectsSubscriptions = []string{
	SubjectSubscriptionPending,
	SubjectSubscriptionConfirmed,
	SubjectSubscriptionRemoved,
	SubjectSubscriptionExpired,
}

var SubjectsReleases = []string{
	SubjectReleaseDetected,
	SubjectReleaseNotified,
}

var SubjectsNotifications = []string{
	SubjectNotificationConfirmationSent,
	SubjectNotificationConfirmationFailed,
	SubjectNotificationReleaseSent,
	SubjectNotificationReleaseFailed,
	SubjectNotificationReleaseDead,
}

type SubscriptionPending struct {
	SagaID     string `json:"sagaID"     validate:"required,uuid"`
	Email      string `json:"email"      validate:"required,email"`
	RepoName   string `json:"repoName"   validate:"required,reponame"`
	ConfirmURL string `json:"confirmURL" validate:"required,url"`
}

type SubscriptionConfirmed struct {
	SagaID     string `json:"sagaID"     validate:"required,uuid"`
	Email      string `json:"email"      validate:"required,email"`
	RepoName   string `json:"repoName"   validate:"required,reponame"`
	UnsubToken string `json:"unsubToken" validate:"required"`
}

type SubscriptionRemoved struct {
	SagaID   string `json:"sagaID"   validate:"required,uuid"`
	Email    string `json:"email"    validate:"required,email"`
	RepoName string `json:"repoName" validate:"required,reponame"`
}

type SubscriptionExpired struct {
	SagaID   string `json:"sagaID"   validate:"required,uuid"`
	Email    string `json:"email"    validate:"required,email"`
	RepoName string `json:"repoName" validate:"required,reponame"`
}

type ReleaseDetected struct {
	SagaID     string `json:"sagaID"     validate:"required,uuid"`
	RepoName   string `json:"repoName"   validate:"required,reponame"`
	Tag        string `json:"tag"        validate:"required"`
	ReleaseURL string `json:"releaseURL" validate:"required,url"`
}

type ReleaseNotified struct {
	SagaID       string   `json:"sagaID"                 validate:"required,uuid"`
	RepoName     string   `json:"repoName"               validate:"required,reponame"`
	Tag          string   `json:"tag"                    validate:"required"`
	SentCount    int      `json:"sentCount"              validate:"gte=0"`
	FailedEmails []string `json:"failedEmails,omitempty" validate:"omitempty,dive,email"`
}

type NotificationConfirmationSent struct {
	SagaID string `json:"sagaID" validate:"required,uuid"`
	Email  string `json:"email"  validate:"required,email"`
}

type NotificationConfirmationFailed struct {
	SagaID string `json:"sagaID" validate:"required,uuid"`
	Email  string `json:"email"  validate:"required,email"`
	Reason string `json:"reason" validate:"required"`
}

type NotificationReleaseSent struct {
	SagaID   string `json:"sagaID"   validate:"required,uuid"`
	RepoName string `json:"repoName" validate:"required,reponame"`
	Tag      string `json:"tag"      validate:"required"`
	Email    string `json:"email"    validate:"required,email"`
}

type NotificationReleaseFailed struct {
	SagaID   string `json:"sagaID"   validate:"required,uuid"`
	RepoName string `json:"repoName" validate:"required,reponame"`
	Tag      string `json:"tag"      validate:"required"`
	Email    string `json:"email"    validate:"required,email"`
	Reason   string `json:"reason"   validate:"required"`
}

type NotificationReleaseDead struct {
	SagaID   string `json:"sagaID"   validate:"required,uuid"`
	RepoName string `json:"repoName" validate:"required,reponame"`
	Tag      string `json:"tag"      validate:"required"`
	Email    string `json:"email"    validate:"required,email"`
	Reason   string `json:"reason"   validate:"required"`
}
