package domain

import (
	"time"

	"github.com/google/uuid"
)

type SagaType string

const SagaTypeSubscribe SagaType = "subscribe"

type State string

const (
	StatePending          State = "pending"
	StateConfirmationSent State = "confirmation_sent"
	StateCompleted        State = "completed"
	StateCompensated      State = "compensated"
	StateExpired          State = "expired"
)

type Saga struct {
	ID        uuid.UUID
	Type      SagaType
	State     State
	Email     string
	RepoName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var allowedTransitions = map[State][]State{
	StatePending:          {StateConfirmationSent, StateCompleted, StateCompensated, StateExpired},
	StateConfirmationSent: {StateCompleted, StateCompensated, StateExpired},
}

func SourceStates(to State) []State {
	var sources []State

	for from, targets := range allowedTransitions {
		for _, t := range targets {
			if t == to {
				sources = append(sources, from)
			}
		}
	}

	return sources
}
