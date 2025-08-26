package nats

const TelegramOutputTextQueue = "TELEGRAM_OUTPUT_TEXT_QUEUE"

//goland:noinspection GoUnusedExportedFunction
func SendTgMessageToUser(userId int64, text string) {
	PublishTgTextMessage(TelegramOutputTextQueue, userId, text)
}
