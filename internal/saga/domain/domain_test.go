package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceStates(t *testing.T) {
	t.Parallel()

	cases := map[State][]State{
		StateConfirmationSent: {StatePending},
		StateCompleted:        {StatePending, StateConfirmationSent},
		StateCompensated:      {StatePending, StateConfirmationSent},
		StateExpired:          {StatePending, StateConfirmationSent},
	}

	for to, want := range cases {
		assert.ElementsMatch(t, want, SourceStates(to), "sources for %s", to)
	}
}

func TestTerminalStatesHaveNoSources(t *testing.T) {
	t.Parallel()
	assert.Empty(t, SourceStates(StatePending))
}
