package log

import (
	"bufio"
	"fmt"
	"os"
	"time"

	ll "github.com/evilsocket/islazy/log"
)

var (
	logs             chan string
	bufferedLogDelay time.Duration
	timeFmt          = "2006-01-02 15:04:05"
)

func Init(enable bool, logFilePath string, delay time.Duration) {
	logs = make(chan string, 512)

	if enable == true {

		bufferedLogDelay = delay

		_, err := os.Stat(logFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				n, err := os.Create(logFilePath)
				if err != nil {
					ll.Fatal("Error creating Buffered LogFile %s: %s", logFilePath, err)
					os.Exit(0)
				}
				n.Close()
			}
		}

		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			Error("Error opening file at %s", logFilePath)
		}

		go BufLog(file)
	}
}

func BufLogInfo(input string) {
	logs <- time.Now().Format(timeFmt) + " [INF] " + input
}

func BufLogError(input string) {
	logs <- time.Now().Format(timeFmt) + " [ERR] " + input
}

func BufLogWarn(input string) {
	logs <- time.Now().Format(timeFmt) + " [WAR] " + input
}

func Raw(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func Debug(format string, args ...interface{}) {
	ll.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	ll.Info(format, args...)
}

func Important(format string, args ...interface{}) {
	ll.Important(format, args...)
}

func Warning(format string, args ...interface{}) {
	ll.Warning(format, args...)
}

func Error(format string, args ...interface{}) {
	ll.Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	ll.Fatal(format, args...)
}

func BufLog(logFile *os.File) error {

	bufWriter := bufio.NewWriter(logFile)

	for logEntry := range logs {
		_, err := bufWriter.WriteString(logEntry + "\n")
		err = bufWriter.Flush()
		if err != nil {
			return err
		}
	}

	time.Sleep(bufferedLogDelay)
	err := BufLog(logFile)
	if err != nil {
		ll.Info("BufLog error: %s", err)
	}
	return nil
}
