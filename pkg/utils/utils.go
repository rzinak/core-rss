package utils

import (
	"fmt"
	"os"
	"sync"
)

type Logger struct {
	file *os.File
	mu   sync.Mutex
}

var (
	instance *Logger
	once     sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		logFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		instance = &Logger{
			file: logFile,
		}
	})
	return instance
}

func (l *Logger) Log(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.file, format+"\n", args...)
	l.file.Sync()
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
