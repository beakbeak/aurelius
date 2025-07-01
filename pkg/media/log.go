package media

import (
	"context"
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
	var level slog.Level
	switch {
	case aurelibLevel == aurelib.LogInfo:
		level = slog.LevelInfo
	case aurelibLevel == aurelib.LogWarning:
		level = slog.LevelWarn
	case aurelibLevel < aurelib.LogWarning:
		level = slog.LevelError
	default:
		return
	}
	slog.Log(context.TODO(), level, message)
}
