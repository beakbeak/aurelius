package media

import (
	"log/slog"

	"github.com/beakbeak/aurelius/pkg/aurelib"
)

func init() {
	aurelib.SetLogger(aurelibLogger{})
}

type aurelibLogger struct{}

func (aurelibLogger) Log(
	aurelibLevel aurelib.LogLevel,
	message string,
) {
	switch aurelibLevel {
	case aurelib.LogPanic:
		slog.Error(message)
	case aurelib.LogFatal:
		slog.Warn(message)
	default:
		if aurelibLevel <= aurelib.LogInfo {
			slog.Debug(message, "level", aurelibLevel.String())
		}
	}
}
