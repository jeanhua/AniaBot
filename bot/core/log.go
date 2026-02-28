package core

import (
	"log"
	"os"
)

var aniaLogger = log.New(os.Stderr, "[AniaBot] ", log.Ltime)

func Logger() *log.Logger {
	return aniaLogger
}
