package database

import (
	"io/ioutil"
	"log"
	"os"
)

type LogLevel int

const (
	LogInfo LogLevel = iota
	LogDebug
	LogNoise

	logLevelCount
	LogNone LogLevel = -1
)

var (
	logLevel = LogNone
	loggers  []*log.Logger
)

func init() {
	loggers = append(loggers,
		log.New(ioutil.Discard, "INFO: ", 0),
		log.New(ioutil.Discard, "DEBUG: ", 0),
		log.New(ioutil.Discard, "NOISE: ", 0),
	)
	if len(loggers) != int(logLevelCount) {
		panic("missing Logger")
	}
}

func SetLogLevel(level LogLevel) {
	if logLevel == level {
		return
	}
	if level >= logLevelCount {
		level = logLevelCount - 1
	}

	logLevel = level

	for i := LogLevel(0); i < logLevelCount; i++ {
		if level >= i {
			loggers[i].SetOutput(ioutil.Discard)
			loggers[i].SetFlags(0)
		} else {
			loggers[i].SetOutput(os.Stdout)
			loggers[i].SetFlags(log.Ltime | log.Lmicroseconds | log.Ldate | log.Lshortfile)
		}
	}
}

func logger(level LogLevel) *log.Logger {
	return loggers[level]
}
