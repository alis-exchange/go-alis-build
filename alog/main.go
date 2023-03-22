package alog

import (
	"context"
	"fmt"
	"log"
)

var (
	loggingLevel       LogLevel
	loggingEnvironment LoggingEnvironment
)

func init() {

	// Disable log prefixes such as the default timestamp.
	// Prefix text prevents the message from being parsed as JSON.
	// A timestamp is added when shipping logs to Google Cloud Logging.
	log.SetFlags(0)

	// Set the default Log Level
	loggingLevel = LevelDefault

	// Set the default logging environment to GOOGLE.
	loggingEnvironment = EnvironmentGoogle
}

// Debug logs a Debug level log.
func Debug(ctx context.Context, msg string) {
	if loggingLevel <= LevelDebug {
		log.Println(&entry{Message: msg, Level: LevelDebug, Ctx: ctx})
	}
}

// Debugf logs a Debug level log with the given context.
func Debugf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelDebug {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelDebug, Ctx: ctx})
	}
}

// Info logs an Info level log.
func Info(ctx context.Context, msg string) {
	if loggingLevel <= LevelInfo {
		log.Println(&entry{Message: msg, Level: LevelInfo, Ctx: ctx})
	}
}

// Infof logs an Info level log with the given context.
func Infof(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelInfo {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelInfo, Ctx: ctx})
	}
}

// Notice logs a Notice level log with the given context.
func Notice(ctx context.Context, msg string) {
	if loggingLevel <= LevelNotice {
		log.Println(&entry{Message: msg, Level: LevelNotice, Ctx: ctx})
	}
}

// Noticef logs a Notice level log with the given context.
func Noticef(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelNotice {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelNotice, Ctx: ctx})
	}
}

// Warn logs a Warning log with the given context.
func Warn(ctx context.Context, msg string) {
	if loggingLevel <= LevelWarning {
		log.Println(&entry{Message: msg, Level: LevelWarning, Ctx: ctx})
	}
}

// Warnf logs a Warning log with the given context.
func Warnf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelWarning {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelWarning, Ctx: ctx})
	}
}

// Error logs an Error log with the given context.
func Error(ctx context.Context, msg string) {
	if loggingLevel <= LevelError {
		log.Println(&entry{Message: msg, Level: LevelError, Ctx: ctx})
	}
}

// Errorf logs an Error log with the given context.
func Errorf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelError {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelError, Ctx: ctx})
	}
}

// Critical logs a Critical log with the given context.
func Critical(ctx context.Context, msg string) {
	if loggingLevel <= LevelCritical {
		log.Println(&entry{Message: msg, Level: LevelCritical, Ctx: ctx})
	}
}

// Criticalf logs a Critical log with the given context.
func Criticalf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelCritical {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelCritical, Ctx: ctx})
	}
}

// Alert logs an Alert log with the given context.
func Alert(ctx context.Context, msg string) {
	if loggingLevel <= LevelAlert {
		log.Println(&entry{Message: msg, Level: LevelAlert, Ctx: ctx})
	}
}

// Alertf logs an Alert log with the given context.
func Alertf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelAlert {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelAlert, Ctx: ctx})
	}
}

// Emergency logs an Emergency log with the given context.
func Emergency(ctx context.Context, msg string) {
	if loggingLevel <= LevelEmergency {
		log.Println(&entry{Message: msg, Level: LevelEmergency, Ctx: ctx})
	}
}

// Emergencyf logs an Emergency log with the given context.
func Emergencyf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelEmergency {
		log.Println(&entry{Message: fmt.Sprintf(format, a...), Level: LevelEmergency, Ctx: ctx})
	}
}

// SetLevel sets the minimum logging level.
func SetLevel(level LogLevel) {
	loggingLevel = level
}

// SetLoggingEnvironment sets the logging environment.
func SetLoggingEnvironment(e LoggingEnvironment) {
	loggingEnvironment = e
}
