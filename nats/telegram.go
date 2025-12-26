package nats

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

const TelegramOutputTextQueue = "TELEGRAM_OUTPUT_TEXT_QUEUE"

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

//goland:noinspection GoUnusedExportedFunction
func PublishTgTextMessage(queue string, chatId int64, text string) {
	msg := natsBotText{
		ChatId: chatId,
		Text:   text,
	}

	if jsonData, err := json.Marshal(msg); err == nil {
		publishMessageToNats(queue, jsonData)
	} else {
		log.Errorf("%v", err)
	}
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotText(data []byte) (int64, string, error) {
	var msg natsBotText

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorf("Error while parsing message from NATS: %v", err)
		return 0, "", err
	}

	chatId := msg.ChatId
	text := msg.Text
	log.Debugf("Received text \"%s\" for chat %d", text, chatId)
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
		log.Errorf("%v", err)
		return
	}

	publishMessageToNats(queue, jsonData)
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotCommand(data []byte) (int64, []string, error) {
	var msg natsBotCommand

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorf("Error while parsing message from NATS: %v", err)
		return 0, nil, err
	}

	chatId := msg.ChatId
	arguments := msg.Arguments

	log.Debugf("Received command arguments \"%v\" for chat %d", arguments, chatId)
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
		log.Errorf("%v", err)
		return
	}

	publishMessageToNats(queue, jsonData)
}

//goland:noinspection GoUnusedExportedFunction
func ParseTgBotFile(data []byte) (int64, string, string, int64, string, string, error) {
	var msg natsBotFile

	err := json.Unmarshal(data, &msg)
	if err != nil {
		log.Errorf("Error while parsing message from NATS: %v", err)
		return 0, "", "", 0, "", "", err
	}

	chatId := msg.ChatId
	fileId := msg.FileId
	fileName := msg.FileName
	size := msg.Size
	mimeType := msg.MimeType
	fileUrl := msg.URL

	log.Debugf("Received file \"%s\" for chat %d", fileName, chatId)
	return chatId, fileId, fileName, size, mimeType, fileUrl, nil
}

//goland:noinspection GoUnusedExportedFunction
func SendTgMessageToUser(userId int64, text string) {
	PublishTgTextMessage(TelegramOutputTextQueue, userId, text)
}
