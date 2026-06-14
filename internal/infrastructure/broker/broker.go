package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var ErrTerminal = errors.New("terminal message error")

const maxDeliver = 5

type Handler func(ctx context.Context, data []byte) error

type Conn struct {
	nc  *nats.Conn
	js  jetstream.JetStream
	log *slog.Logger
}

func Connect(url string, log *slog.Logger) (*Conn, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()

		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	return &Conn{nc: nc, js: js, log: log.With("component", "broker")}, nil
}

func (c *Conn) EnsureStream(ctx context.Context, name string, subjects []string) error {
	if _, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
	}); err != nil {
		return fmt.Errorf("ensure stream %q: %w", name, err)
	}

	return nil
}

func (c *Conn) Publish(ctx context.Context, subject string, data []byte) error {
	if _, err := c.js.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("publish to %q: %w", subject, err)
	}

	return nil
}

func (c *Conn) Consume(ctx context.Context, stream, durable, subject string, handler Handler) (func(), error) {
	cons, err := c.js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    maxDeliver,
	})
	if err != nil {
		return nil, fmt.Errorf("create consumer %q: %w", durable, err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		c.dispatch(ctx, subject, handler, msg)
	})
	if err != nil {
		return nil, fmt.Errorf("start consume %q: %w", durable, err)
	}

	return cc.Stop, nil
}

func (c *Conn) dispatch(ctx context.Context, subject string, handler Handler, msg jetstream.Msg) {
	switch err := handler(ctx, msg.Data()); {
	case err == nil:
		c.ack(msg.Ack(), subject)
	case errors.Is(err, ErrTerminal):
		c.log.ErrorContext(ctx, "dropping poison message", "subject", subject, "error", err)
		c.ack(msg.Term(), subject)
	default:
		c.log.ErrorContext(ctx, "message handler failed; redelivering", "subject", subject, "error", err)
		c.ack(msg.Nak(), subject)
	}
}

func (c *Conn) ack(err error, subject string) {
	if err != nil {
		c.log.Warn("failed to acknowledge message", "subject", subject, "error", err)
	}
}

func (c *Conn) Close() error {
	if err := c.nc.Drain(); err != nil {
		return fmt.Errorf("drain nats: %w", err)
	}

	return nil
}
