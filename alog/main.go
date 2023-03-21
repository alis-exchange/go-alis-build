package alog // import "go.alis.build/alog"

import (
	"context"
	"fmt"
	"log"
)

var (
	loggingLevel       LogSeverity
	loggingEnvironment LoggingEnvironment
)

func init() {

	// Disable log prefixes such as the default timestamp.
	// Prefix text prevents the message from being parsed as JSON.
	// A timestamp is added when shipping logs to Google Cloud Logging.
	log.SetFlags(0)

	// Set the default Log Level
	loggingLevel = DEBUG

	// Set the default logging environment to GOOGLE.
	loggingEnvironment = GOOGLE
}

// Info logs an Info level log.
func Info(ctx context.Context, msg string) {
	log.Println(&entry{Message: msg, Severity: INFO, Ctx: ctx})
}

// Infof logs an Info level log.
func Infof(ctx context.Context, format string, a ...any) {
	log.Println(&entry{Message: fmt.Sprintf(format, a), Severity: INFO, Ctx: ctx})
}

// Warn logs a Warning log.
func Warn(ctx context.Context, msg string) {
	log.Println(&entry{Message: msg, Severity: WARNING, Ctx: ctx})
}

// Warnf logs a Warning log.
func Warnf(ctx context.Context, format string, a ...any) {
	log.Println(&entry{Message: fmt.Sprintf(format, a), Severity: WARNING, Ctx: ctx})
}

// Error logs an Error log.
func Error(ctx context.Context, msg string) {
	log.Println(&entry{Message: msg, Severity: ERROR, Ctx: ctx})
}

// Errorf logs an Error log.
func Errorf(ctx context.Context, format string, a ...any) {
	log.Println(&entry{Message: fmt.Sprintf(format, a), Severity: ERROR, Ctx: ctx})
}

// SetLevel sets the minimum logging level.
func SetLevel(severity LogSeverity) {
	loggingLevel = severity
}

// SetLoggingEnvironment sets the logging environment.
func SetLoggingEnvironment(e LoggingEnvironment) {
	loggingEnvironment = e
}
