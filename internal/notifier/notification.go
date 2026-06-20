package notifier

type ReleaseNotification struct {
	To             string
	Repo           string
	Tag            string
	ReleaseURL     string
	UnsubscribeURL string
}

type BatchResult struct {
	Sent   int
	Failed []string
}
