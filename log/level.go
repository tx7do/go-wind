package log

// Level represents a log severity level. Higher values are more severe.
// It is used as an argument to [Logger.Enabled] to check whether a given
// level would be emitted by the underlying backend.
type Level int

// Standard log levels ordered from least to most severe.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the human-readable name of the level, e.g. "DEBUG".
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
