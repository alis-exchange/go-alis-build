package alog

import (
	"context"
	"fmt"
	"log"
	"os"
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
		(&entry{Message: msg, Level: LevelDebug, Ctx: ctx}).Output()
	}
}

// Debugf logs a Debug level log with the given context.
func Debugf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelDebug {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelDebug, Ctx: ctx}).Output()
	}
}

// Info logs an Info level log.
func Info(ctx context.Context, msg string) {
	if loggingLevel <= LevelInfo {
		(&entry{Message: msg, Level: LevelInfo, Ctx: ctx}).Output()
	}
}

// Infof logs an Info level log with the given context.
func Infof(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelInfo {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelInfo, Ctx: ctx}).Output()
	}
}

// Notice logs a Notice level log with the given context.
func Notice(ctx context.Context, msg string) {
	if loggingLevel <= LevelNotice {
		(&entry{Message: msg, Level: LevelNotice, Ctx: ctx}).Output()
	}
}

// Noticef logs a Notice level log with the given context.
func Noticef(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelNotice {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelNotice, Ctx: ctx}).Output()
	}
}

// Warn logs a Warning log with the given context.
func Warn(ctx context.Context, msg string) {
	if loggingLevel <= LevelWarning {
		(&entry{Message: msg, Level: LevelWarning, Ctx: ctx}).Output()
	}
}

// Warnf logs a Warning log with the given context.
func Warnf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelWarning {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelWarning, Ctx: ctx}).Output()
	}
}

// Error logs an Error log with the given context.
func Error(ctx context.Context, msg string) {
	if loggingLevel <= LevelError {
		(&entry{Message: msg, Level: LevelError, Ctx: ctx}).Output()
	}
}

// Errorf logs an Error log with the given context.
func Errorf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelError {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelError, Ctx: ctx}).Output()
	}
}

// Critical logs a Critical log with the given context.
func Critical(ctx context.Context, msg string) {
	if loggingLevel <= LevelCritical {
		(&entry{Message: msg, Level: LevelCritical, Ctx: ctx}).Output()
	}
}

// Criticalf logs a Critical log with the given context.
func Criticalf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelCritical {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelCritical, Ctx: ctx}).Output()
	}
}

// Fatal logs a Critical log and exists the program.
func Fatal(ctx context.Context, msg string) {
	if loggingLevel <= LevelCritical {
		(&entry{Message: msg, Level: LevelCritical, Ctx: ctx}).Output()
	}
	os.Exit(1)
}

// Fatalf logs a Critical log and exists the program.
func Fatalf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelCritical {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelCritical, Ctx: ctx}).Output()
	}
	os.Exit(1)
}

// Alert logs an Alert log with the given context.
func Alert(ctx context.Context, msg string) {
	if loggingLevel <= LevelAlert {
		(&entry{Message: msg, Level: LevelAlert, Ctx: ctx}).Output()
	}
}

// Alertf logs an Alert log with the given context.
func Alertf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelAlert {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelAlert, Ctx: ctx}).Output()
	}
}

// Emergency logs an Emergency log with the given context.
func Emergency(ctx context.Context, msg string) {
	if loggingLevel <= LevelEmergency {
		(&entry{Message: msg, Level: LevelEmergency, Ctx: ctx}).Output()
	}
}

// Emergencyf logs an Emergency log with the given context.
func Emergencyf(ctx context.Context, format string, a ...any) {
	if loggingLevel <= LevelEmergency {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelEmergency, Ctx: ctx}).Output()
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
