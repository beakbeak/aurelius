package media

import (
	"sb/aurelius/pkg/aurelib"
)

// A Logger implements a subset of methods provided by the standard library's log.Logger.
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
}

var log Logger = discardLogger{}

// SetLogger sets the Logger used to deliver log messages.
// By default, log messages are discarded.
func SetLogger(value Logger) {
	if value != nil {
		log = value
	} else {
		log = discardLogger{}
	}
}

func init() {
	aurelib.SetLogger(aurelibLogger{})
}

type discardLogger struct{}

func (discardLogger) Print(v ...interface{})                 {}
func (discardLogger) Printf(format string, v ...interface{}) {}

type aurelibLogger struct{}

func (aurelibLogger) Log(
	level aurelib.LogLevel,
	message string,
) {
	if level > aurelib.LogInfo {
		return
	}
	log.Print(message)
}
