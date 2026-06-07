package metrics

import "context"

type UseCase[In, Out any] interface {
	Execute(ctx context.Context, in In) (Out, error)
}

type Metered[In, Out any] struct {
	operation string
	inner     UseCase[In, Out]
}

func NewMetered[In, Out any](operation string, inner UseCase[In, Out]) Metered[In, Out] {
	return Metered[In, Out]{operation: operation, inner: inner}
}

func (m Metered[In, Out]) Execute(ctx context.Context, in In) (out Out, err error) {
	defer func() {
		SubscriptionOperationsTotal.WithLabelValues(m.operation, ResultLabel(err)).Inc()
	}()

	return m.inner.Execute(ctx, in)
}
