package nats

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

type natsBotText struct {
	UserId int64  `json:"user_id"`
	Text   string `json:"text"`
}

type natsBotCommand struct {
	UserId    int64    `json:"user_id"`
	Arguments []string `json:"arguments"`
}

type natsBotFile struct {
	UserId   int64  `json:"user_id"`
	FileId   string `json:"file_id"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	URL      string `json:"url"`
}

type ListenerHandlerFunction func(data []byte)

type ListenerHandler struct {
	Function ListenerHandlerFunction
}

//goland:noinspection GoUnusedExportedFunction
func PublishTgTextMessage(queue string, userId int64, text string) {
	msg := natsBotText{
		UserId: userId,
		Text:   text,
	}

	if jsonData, err := json.Marshal(msg); err == nil {
		publishMessageToNats(queue, jsonData)
	} else {
		log.Printf("[ERROR] %v", err)
	}
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotText(data []byte) (int64, string, error) {
	var msg natsBotText

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Printf("[ERROR] Помилка при розборі повідомлення з NATS: %v", err)
		return 0, "", err
	}

	userId := msg.UserId
	text := msg.Text
	log.Printf("[DEBUG] Отримано текст \"%s\" для користувача %d", text, userId)
	return userId, text, nil
}

//goland:noinspection GoUnusedExportedFunction
func PublishTgCommandMessage(queue string, userId int64, message []string) {
	msg := natsBotCommand{
		UserId:    userId,
		Arguments: message,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] %v", err)
		return
	}

	publishMessageToNats(queue, jsonData)
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotCommand(data []byte) (int64, []string, error) {
	var msg natsBotCommand

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Printf("[ERROR] Помилка при розборі повідомлення з NATS: %v", err)
		return 0, nil, err
	}

	userId := msg.UserId
	arguments := msg.Arguments

	log.Printf("[DEBUG] Отримано аргументи команди \"%v\" для користувача %d", arguments, userId)
	return userId, arguments, nil
}

//goland:noinspection GoUnusedExportedFunction
func PublishTgFileInfoMessage(queue string, userId int64, fileId string, fileName string, fileSize int64, mimeType string, url string) {
	msg := natsBotFile{
		UserId:   userId,
		FileId:   fileId,
		FileName: fileName,
		Size:     fileSize,
		MimeType: mimeType,
		URL:      url,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] %v", err)
		return
	}

	publishMessageToNats(queue, jsonData)
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotFile(data []byte) (int64, string, string, int64, string, string, error) {
	var msg natsBotFile

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Printf("[ERROR] Помилка при розборі повідомлення з NATS: %v", err)
		return 0, "", "", 0, "", "", err
	}

	userId := msg.UserId
	fileId := msg.FileId
	fileName := msg.FileName
	size := msg.Size
	mimeType := msg.MimeType
	fileUrl := msg.URL

	log.Printf("[DEBUG] Отримано файл \"%s\" для користувача %d", fileName, userId)
	return userId, fileId, fileName, size, mimeType, fileUrl, nil
}

//goland:noinspection GoUnusedExportedFunction
func StartNatsListener(queue string, handler *ListenerHandler) {
	nc, err := connect()
	if err != nil {
		log.Printf("[ERROR] Помилка під'єднання до NATS (черга \"%s\"): %v", err, queue)
	}
	if _, err = nc.Subscribe(queue, func(msg *nats.Msg) { handler.Function(msg.Data) }); err != nil {
		log.Printf("[ERROR] Помилка підписки до черги \"%s\" в NATS: %v", queue, err)
	}

	if err = nc.Flush(); err != nil {
		log.Printf("[ERROR] Помилка flash в черзі \"%s\" NATS : %v", queue, err)
	}

	if err = nc.LastError(); err != nil {
		log.Printf("[ERROR] Помилка для черги \"%s\" в NATS: %v", queue, err)
	}

	log.Printf("[INFO] Підписка до черги \"%s\" вдала", queue)
}

func publishMessageToNats(queue string, message []byte) {
	nc, err := connect()
	if err != nil {
		log.Printf("[ERROR] Помилка під'єднання до NATS (черга \"%s\"): %v", err, queue)
	}

	defer nc.Close()

	if err2 := nc.Publish(queue, message); err2 == nil {
		log.Printf("[DEBUG] Повідомлення надіслано в чергу \"%s\" NATS", queue)
	} else {
		log.Printf("[ERROR] Помилка публікації в чергу \"%s\" NATS: %v", queue, err)
	}
}

func connect() (*nats.Conn, error) {
	ip := os.Getenv("BROKER_IP")
	port := os.Getenv("BROKER_PORT")
	natsUrl := fmt.Sprintf("nats://%s:%s", ip, port)

	log.Printf("[DEBUG] Під'єднуюся до NATS: %s", natsUrl)

	for i := 0; i < 5; i++ {
		nc, err := nats.Connect(natsUrl)
		if err != nil {
			log.Printf("[INFO] Error connecting to NATS (%d try): %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("[INFO] Під'єднався до NATS: %s", natsUrl)
		return nc, nil
	}
	return nil, fmt.Errorf("неможливо під'єднатися до NATS: %s", natsUrl)
}
