package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	natsio "github.com/nats-io/nats.go"
)

// JetStreamHandler processes a message. Returning nil acknowledges it;
// returning an error negatively acknowledges it for redelivery.
type JetStreamHandler func(data []byte) error

// PublishJetStreamBytes persists a message and waits for the server's publish acknowledgement.
func (c *Client) PublishJetStreamBytes(subject string, data []byte) error {
	if c == nil || c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("nats client is not connected")
	}
	if subject == "" {
		return fmt.Errorf("subject is empty")
	}
	js, err := c.conn.JetStream()
	if err != nil {
		return fmt.Errorf("initialize JetStream: %w", err)
	}
	if _, err := js.Publish(subject, data); err != nil {
		return fmt.Errorf("publish JetStream subject %q: %w", subject, err)
	}
	return nil
}

// PublishJetStreamStruct JSON-encodes a Go value before publishing.
func (c *Client) PublishJetStreamStruct(subject string, payload any) error {
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal JetStream payload: %w", err)
	}
	return c.PublishJetStreamBytes(subject, data)
}

// StartJetStreamListener starts a durable pull consumer and blocks until ctx
// is canceled or a subscription error occurs. An existing durable consumer is
// reused; the stream must exist and contain subject.
// Messages are explicitly Ack'ed on handler success and Nak'ed on failure.
func (c *Client) StartJetStreamListener(
	ctx context.Context,
	stream, subject, durable string,
	handler JetStreamHandler,
) error {
	if c == nil || c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("nats client is not connected")
	}
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if stream == "" || subject == "" || durable == "" {
		return fmt.Errorf("stream, subject and durable must not be empty")
	}
	if handler == nil {
		return fmt.Errorf("JetStream handler is nil")
	}
	js, err := c.conn.JetStream()
	if err != nil {
		return fmt.Errorf("initialize JetStream: %w", err)
	}
	sub, err := js.PullSubscribe(
		subject, durable,
		natsio.BindStream(stream),
		natsio.ManualAck(),
		natsio.AckExplicit(),
	)
	if err != nil {
		return fmt.Errorf("pull subscribe to stream %q, durable %q: %w", stream, durable, err)
	}
	defer sub.Unsubscribe()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		messages, err := sub.Fetch(1, natsio.MaxWait(time.Second))
		if err != nil {
			if errors.Is(err, natsio.ErrTimeout) {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch JetStream messages: %w", err)
		}
		for _, msg := range messages {
			if ctx.Err() != nil {
				return nil
			}
			if err := handler(msg.Data); err != nil {
				if nakErr := msg.Nak(); nakErr != nil {
					return fmt.Errorf("handler: %v; nack: %w", err, nakErr)
				}
				continue
			}
			if err := msg.Ack(); err != nil {
				return fmt.Errorf("ack JetStream message: %w", err)
			}
		}
	}
}
