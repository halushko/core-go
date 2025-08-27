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
	ChatId int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type natsBotCommand struct {
	ChatId    int64    `json:"chat_id"`
	Arguments []string `json:"arguments"`
}

type natsBotFile struct {
	ChatId   int64  `json:"chat_id"`
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
func PublishTgTextMessage(queue string, chatId int64, text string) {
	msg := natsBotText{
		ChatId: chatId,
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

	chatId := msg.ChatId
	text := msg.Text
	log.Printf("[DEBUG] Отримано текст \"%s\" для чату %d", text, chatId)
	return chatId, text, nil
}

//goland:noinspection GoUnusedExportedFunction
func PublishTgCommandMessage(queue string, chatId int64, message ...string) {
	msg := natsBotCommand{
		ChatId:    chatId,
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

	chatId := msg.ChatId
	arguments := msg.Arguments

	log.Printf("[DEBUG] Отримано аргументи команди \"%v\" для чату %d", arguments, chatId)
	return chatId, arguments, nil
}

//goland:noinspection GoUnusedExportedFunction
func PublishTgFileInfoMessage(queue string, chatId int64, fileId string, fileName string, fileSize int64, mimeType string, url string) {
	msg := natsBotFile{
		ChatId:   chatId,
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

	chatId := msg.ChatId
	fileId := msg.FileId
	fileName := msg.FileName
	size := msg.Size
	mimeType := msg.MimeType
	fileUrl := msg.URL

	log.Printf("[DEBUG] Отримано файл \"%s\" для чату %d", fileName, chatId)
	return chatId, fileId, fileName, size, mimeType, fileUrl, nil
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
