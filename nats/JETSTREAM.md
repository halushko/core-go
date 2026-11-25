# JetStream helpers

The existing core NATS client methods remain unchanged. JetStream requires
the server to have JetStream enabled and a stream configured for the subject.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

client, err := nats.NewClient()
if err != nil { return err }
defer client.Close()

// Stream POWER must already contain subjects matching power.events.
err = client.PublishJetStreamStruct("power.events", PowerEvent{Voltage: 235})
if err != nil { return err }

// Blocks until ctx is canceled. Durable consumer state is retained.
err = client.StartJetStreamListener(
    ctx, "POWER", "power.events", "power-events-handler",
    func(data []byte) error {
        // Process and persist data. Return nil to Ack or an error to Nak.
        return process(data)
    },
)
```

`PublishJetStreamBytes` and `PublishJetStreamStruct` wait for a publish
acknowledgement from JetStream. `StartJetStreamListener` creates a durable pull
consumer if absent, otherwise binds to it, processes messages one at a time,
and explicitly acknowledges successful processing. A failed handler uses Nak
and the message can be redelivered. Processing should therefore be idempotent.
The stream is not automatically created, preventing accidental stream
configuration changes.
