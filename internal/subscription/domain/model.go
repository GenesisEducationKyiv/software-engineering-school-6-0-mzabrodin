package domain

type SubscriptionView struct {
	Email     string
	Repo      string
	Confirmed bool
}

type ConfirmResult struct {
	Confirmed  bool
	UnsubToken string
}

type RemovedSubscription struct {
	Email string
	Repo  string
}

type ExpiredSubscription struct {
	Email string
	Repo  string
}
