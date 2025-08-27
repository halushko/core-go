package logger

import (
	"fmt"
	"log"
	"os"
)

const defaultLogFolder = "./logs"

//goland:noinspection GoUnusedExportedFunction
func SoftPrepareLogFile(fileName string) *os.File {
	logFolderPath := os.Getenv("LOG_FOLDER_PATH")
	if logFolderPath == "" {
		fmt.Printf("[DEBUG] log file destination env LOG_FOLDER_PATH is empty. Using default: %s", defaultLogFolder)
		logFolderPath = defaultLogFolder
	}

	filePath := fmt.Sprintf("%s/%s", logFolderPath, fileName)

	logFile, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		log.Println(fmt.Sprintf("[ERROR] can't open log file: %v", err))
		return nil
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Println(fmt.Sprintf("[DEBUG] Log file created: %s", filePath))
	return logFile
}

//goland:noinspection GoUnusedExportedFunction
func SoftLogClose(file *os.File) {
	if file == nil {
		log.Println("[ERROR] logfile is nil")
		return
	}

	err := file.Close()
	if err != nil {
		log.Println(fmt.Sprintf("[ERROR] Can't close log file %s: %v", file.Name(), err))
	}
}
