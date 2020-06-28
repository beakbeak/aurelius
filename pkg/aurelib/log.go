package aurelib

/*
#cgo pkg-config: libavutil

#include <libavutil/avutil.h>
*/
import "C"

// A LogLevel represents the type of information in a log message from FFmpeg. It corresponds to
// FFmpeg's AV_LOG_* constants.
type LogLevel int

// The following comments have been extracted from the FFmpeg documentation for
// the noted AV_LOG_* constants.
const (
	// "Print no output." (AV_LOG_QUIET)
	LogQuiet LogLevel = iota

	// "Something went really wrong and we will crash now." (AV_LOG_PANIC)
	LogPanic

	// "Something went wrong and recovery is not possible. For example, no
	// header was found for a format which depends on headers or an illegal
	// combination of parameters is used." (AV_LOG_FATAL)
	LogFatal

	// "Something went wrong and cannot losslessly be recovered. However, not
	// all future data is affected." (AV_LOG_ERROR)
	LogError

	// "Something somehow does not look correct. This may or may not lead to
	// problems. An example would be the use of '-vstrict -2'." (AV_LOG_WARNING)
	LogWarning

	// "Standard information." (AV_LOG_INFO)
	LogInfo

	// "Detailed information." (AV_LOG_VERBOSE)
	LogVerbose

	// "Stuff which is only useful for libav* developers." (AV_LOG_DEBUG)
	LogDebug

	// "Extremely verbose debugging, useful for libav* development."
	// (AV_LOG_TRACE)
	LogTrace
)

// A Logger handles log messages produced by FFmpeg.
type Logger interface {
	Log(level LogLevel, message string) // Must be thread-safe!
}

var logger Logger

// SetLogger sets the Logger used to handle messages from FFmpeg.
// By default, log messages are discarded.
func SetLogger(value Logger) {
	logger = value
}

func toLogLevel(avLevel C.int) (LogLevel, bool /*ok*/) {
	switch avLevel {
	case C.AV_LOG_QUIET:
		return LogQuiet, true
	case C.AV_LOG_PANIC:
		return LogPanic, true
	case C.AV_LOG_FATAL:
		return LogFatal, true
	case C.AV_LOG_ERROR:
		return LogError, true
	case C.AV_LOG_WARNING:
		return LogWarning, true
	case C.AV_LOG_INFO:
		return LogInfo, true
	case C.AV_LOG_VERBOSE:
		return LogVerbose, true
	case C.AV_LOG_DEBUG:
		return LogDebug, true
	case C.AV_LOG_TRACE:
		return LogTrace, true
	}
	// An undocumented log level was used
	return LogTrace, false
}

//export logMessage
func logMessage(
	avLevel C.int,
	cMessage *C.char,
) {
	if logger == nil {
		return
	}
	if level, ok := toLogLevel(avLevel); ok {
		logger.Log(level, C.GoString(cMessage))
	}
}
