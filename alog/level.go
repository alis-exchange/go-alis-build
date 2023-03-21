package alog

// LogLevel int is used to map the logging levels consistent with Google Cloud Logging.
type LogLevel int

const (
	// LevelDefault where the log entry has no assigned severity level.
	LevelDefault LogLevel = 0
	// LevelDebug or trace information.
	LevelDebug LogLevel = -4
	// LevelInfo Routine information, such as ongoing status or performance.
	LevelInfo LogLevel = 0
	// LevelNotice Normal but significant events, such as start up, shut down, or
	// a configuration change.
	LevelNotice LogLevel = 1
	// LevelWarning events might cause problems.
	LevelWarning LogLevel = 4
	// LevelError events are likely to cause problems.
	LevelError LogLevel = 8
	// LevelCritical events cause more severe problems or outages.
	LevelCritical LogLevel = 10
	// LevelAlert A person must take an action immediately.
	LevelAlert LogLevel = 12
	// LevelEmergency One or more systems are unusable.
	LevelEmergency LogLevel = 14
)

// String returns a name for the level.
// The uppercase name of the level is returned
// Examples:
//
//	LevelWarning.String() => "WARNING"
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelNotice:
		return "NOTICE"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	case LevelAlert:
		return "ALERT"
	case LevelEmergency:
		return "EMERGENCY"
	default:
		return "INFO"
	}
}
