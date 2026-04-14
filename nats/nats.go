package nats

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

var natsURL = fmt.Sprintf("nats://%s:%s", os.Getenv("BROKER_IP"), os.Getenv("BROKER_PORT"))

type Client struct {
	conn *nats.Conn
}

type Message struct {
	data  []byte
	reply func([]byte) error
}

type ListenerHandlerFunction func(data []byte)

type ListenerHandler struct {
	Function ListenerHandlerFunction
}

type RequestListenerHandlerFunction func(msg *Message)

type RequestListenerHandler struct {
	Function RequestListenerHandlerFunction
}

//goland:noinspection GoUnusedExportedFunction
func NewClient() (*Client, error) {
	if conn, err := connect(); err != nil {
		return nil, err
	} else {
		return &Client{conn: conn}, nil
	}
}

func (c *Client) Close() {
	if c == nil || c.conn == nil || c.conn.IsClosed() {
		return
	}

	c.conn.Close()
	log.Info("NATS connection closed")
}

func (c *Client) StartListener(queue string, handler *ListenerHandler) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nats client is not initialized")
	}
	if handler == nil || handler.Function == nil {
		return fmt.Errorf("handler is nil")
	}

	if _, err := c.conn.Subscribe(queue, func(msg *nats.Msg) {
		handler.Function(msg.Data)
	}); err != nil {
		log.Errorf("Error subscribing to NATS queue %q: %v", queue, err)
		return fmt.Errorf("subscribe to queue %q: %w", queue, err)
	}

	if err := c.conn.Flush(); err != nil {
		log.Errorf("Error flushing NATS queue %q: %v", queue, err)
		return fmt.Errorf("flush queue %q: %w", queue, err)
	}

	if err := c.conn.LastError(); err != nil {
		log.Errorf("NATS error for queue %q: %v", queue, err)
		return fmt.Errorf("last error for queue %q: %w", queue, err)
	}

	log.Infof("Successfully subscribed to queue %q", queue)
	return nil
}

func (c *Client) StartRequestListener(queue string, handler *RequestListenerHandler) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nats client is not initialized")
	}
	if handler == nil || handler.Function == nil {
		return fmt.Errorf("handler is nil")
	}

	if _, err := c.conn.Subscribe(queue, func(msg *nats.Msg) {
		wrapped := &Message{
			data: msg.Data,
			reply: func(data []byte) error {
				if msg.Reply == "" {
					return fmt.Errorf("no reply subject")
				}
				return msg.Respond(data)
			},
		}

		handler.Function(wrapped)
	}); err != nil {
		return fmt.Errorf("subscribe to queue %q: %w", queue, err)
	}

	if err := c.conn.Flush(); err != nil {
		return fmt.Errorf("flush queue %q: %w", queue, err)
	}

	if err := c.conn.LastError(); err != nil {
		return fmt.Errorf("last error for queue %q: %w", queue, err)
	}

	return nil
}

func (c *Client) Request(queue string, payload any, timeout time.Duration, response any) error {
	if response == nil {
		return fmt.Errorf("response target is nil")
	}
	if c == nil || c.conn == nil {
		return fmt.Errorf("nats client is not initialized")
	}
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}

	t := reflect.TypeOf(payload)
	if t.Kind() == reflect.Ptr {
		if t.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("payload must be a struct or pointer to struct, got %s", t.Kind())
		}
	} else if t.Kind() != reflect.Struct {
		return fmt.Errorf("payload must be a struct or pointer to struct, got %s", t.Kind())
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal struct for queue %q: %w", queue, err)
	}

	msg, err := c.conn.Request(queue, data, timeout)
	if err != nil {
		log.Errorf("Request to NATS queue %q failed: %v", queue, err)
		return fmt.Errorf("request to queue %q: %w", queue, err)
	}

	if err := json.Unmarshal(msg.Data, response); err != nil {
		return fmt.Errorf("unmarshal response from queue %q: %w", queue, err)
	}

	return nil
}

func (m *Message) Respond(v any) error {
	if m == nil {
		return fmt.Errorf("message is nil")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if m.reply == nil {
		return fmt.Errorf("reply handler is nil")
	}
	return m.reply(data)
}

func (m *Message) Unmarshal(v any) error {
	if m == nil {
		return fmt.Errorf("message is nil")
	}
	if v == nil {
		return fmt.Errorf("target is nil")
	}
	return json.Unmarshal(m.data, v)
}

func (c *Client) PublishBytes(queue string, message []byte) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nats client is not initialized")
	}

	if err := c.conn.Publish(queue, message); err != nil {
		log.Errorf("Error publishing to NATS queue %q: %v", queue, err)
		return fmt.Errorf("publish to queue %q: %w", queue, err)
	}

	log.Debugf("Message has been sent to NATS queue %q", queue)
	return nil
}

func (c *Client) PublishString(queue string, message string) error {
	return c.PublishBytes(queue, []byte(message))
}

func (c *Client) PublishStruct(queue string, payload any) error {
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}

	t := reflect.TypeOf(payload)
	if t.Kind() == reflect.Ptr {
		if t.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("payload must be a struct or pointer to struct, got %s", t.Kind())
		}
	} else if t.Kind() != reflect.Struct {
		return fmt.Errorf("payload must be a struct or pointer to struct, got %s", t.Kind())
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal struct for queue %q: %w", queue, err)
	}

	return c.PublishBytes(queue, data)
}

func connect() (*nats.Conn, error) {
	log.Debugf("Connecting to NATS: %s", natsURL)

	var lastErr error
	for i := 0; i < 5; i++ {
		nc, err := nats.Connect(
			natsURL,
			nats.Name("app-nats-client"),
			nats.ReconnectWait(3*time.Second),
			nats.MaxReconnects(5),
			nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
				log.Warnf("Disconnected from NATS: %v", err)
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				log.Infof("Reconnected to NATS: %s", nc.ConnectedUrl())
			}),
			nats.ClosedHandler(func(_ *nats.Conn) {
				log.Warn("NATS connection closed")
			}),
		)
		if err != nil {
			lastErr = err
			log.Infof("Error connecting to NATS (try %d): %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Infof("Connected to NATS: %s", natsURL)
		return nc, nil
	}

	return nil, fmt.Errorf("unable to connect to NATS %q: %w", natsURL, lastErr)
}
