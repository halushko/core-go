package nats

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
)

type ListenerHandlerFunction func(data []byte)

type ListenerHandler struct {
	Function ListenerHandlerFunction
}

//goland:noinspection GoUnusedExportedFunction
func StartNatsListener(queue string, handler *ListenerHandler) {
	nc, err := connect()
	if err != nil {
		log.Errorf("Error connecting to NATS (queue \"%s\"): %v", queue, err)
	}
	if _, err = nc.Subscribe(queue, func(msg *nats.Msg) { handler.Function(msg.Data) }); err != nil {
		log.Errorf("Error subscribing to NATS queue \"%s\": %v", queue, err)
	}

	if err = nc.Flush(); err != nil {
		log.Errorf("Error flushing NATS queue \"%s\": %v", queue, err)
	}

	if err = nc.LastError(); err != nil {
		log.Errorf("Error for NATS queue \"%s\": %v", queue, err)
	}

	log.Infof("Successfully subscribed to queue \"%s\"", queue)
}

func publishMessageToNats(queue string, message []byte) {
	nc, err := connect()
	if err != nil {
		log.Errorf("Error connecting to NATS (queue \"%s\"): %v", queue, err)
	}

	defer nc.Close()

	if err2 := nc.Publish(queue, message); err2 == nil {
		log.Debugf("Message has been sent to NATS queue \"%s\"", queue)
	} else {
		log.Errorf("Error publishing to NATS queue \"%s\": %v", queue, err2)
	}
}

func connect() (*nats.Conn, error) {
	ip := os.Getenv("BROKER_IP")
	port := os.Getenv("BROKER_PORT")
	natsUrl := fmt.Sprintf("nats://%s:%s", ip, port)

	log.Debugf("Connecting to NATS: %s", natsUrl)

	for i := 0; i < 5; i++ {
		nc, err := nats.Connect(natsUrl)
		if err != nil {
			log.Infof("Error connecting to NATS (try %d): %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Infof("Connected to NATS: %s", natsUrl)
		return nc, nil
	}
	return nil, fmt.Errorf("unable to connect to NATS: %s", natsUrl)
}
