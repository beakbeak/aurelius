package media

import (
	"io/ioutil"
	"log"
	"os"
	"sb/aurelius/pkg/aurelib"
)

type LogLevel int

const (
	LogInfo LogLevel = iota
	LogDebug
	LogNoise
	LogLevelCount
	LogNone LogLevel = -1
)

var (
	logLevel = LogNone
	loggers  = [...]*log.Logger{
		log.New(ioutil.Discard, "INFO: ", 0),
		log.New(ioutil.Discard, "DEBUG: ", 0),
		log.New(ioutil.Discard, "NOISE: ", 0),
	}
)

func init() {
	if len(loggers) != int(LogLevelCount) {
		panic("missing Logger")
	}
}

func SetLogLevel(level LogLevel) {
	if logLevel == level {
		return
	}
	if level >= LogLevelCount {
		level = LogLevelCount - 1
	}

	if level >= LogDebug {
		aurelib.SetLogLevel(aurelib.LogInfo)
	}

	logLevel = level

	for i := LogLevel(0); i < LogLevelCount; i++ {
		if level >= i {
			loggers[i].SetOutput(os.Stdout)
			loggers[i].SetFlags(log.Ltime | log.Lmicroseconds | log.Ldate | log.Lshortfile)
		} else {
			loggers[i].SetOutput(ioutil.Discard)
			loggers[i].SetFlags(0)
		}
	}
}

func logger(level LogLevel) *log.Logger {
	return loggers[level]
}
