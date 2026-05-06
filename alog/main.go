package alog

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var (
	loggingLevel       atomic.Int64
	loggingEnvironment atomic.Value
	logWriter          atomic.Value
	writerMu           sync.Mutex
)

type writerHolder struct {
	writer io.Writer
}

func init() {
	logWriter.Store(writerHolder{writer: os.Stderr})

	// Set the default logging environment
	// We'll use environment variable to set the defaults
	// https://cloud.google.com/run/docs/container-contract#env-vars
	// Cloud Run exposes an env of K_SERVICE
	// Cloud Run exposes an env of CLOUD_RUN_JOB
	// GKE Autopilot exposes an env of KUBERNETES_SERVICE_HOST
	if isGoogleEnvironment() {
		setLoggingEnvironment(EnvironmentGoogle)
		setLoggingLevel(logLevelFromEnv("ALOG_LEVEL", LevelInfo))
	} else {
		setLoggingEnvironment(EnvironmentLocal)
		setLoggingLevel(LevelDebug)
	}
}

func isGoogleEnvironment() bool {
	return os.Getenv("K_SERVICE") != "" ||
		os.Getenv("CLOUD_RUN_JOB") != "" ||
		os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

func logLevelFromEnv(name string, fallback LogLevel) LogLevel {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	level, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("%s must be an integer", name))
	}

	return LogLevel(level)
}

// Debug logs a Debug level log.
//
// This method only prints a log when the Logging Level set to LevelDebug.  The debug logs will
// also include a SourceLocation attribute which will provide file, method and line number details of
// the particular log.
func Debug(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelDebug {
		(&entry{Message: msg, Level: LevelDebug, Ctx: ctx}).Output()
	}
}

// Debugf logs a Debug level log with the given context.
//
// This method only prints a log when the Logging Level set to LevelDebug.  The debug logs will
// also include a SourceLocation attribute which will provide file, method and line number details of
// the particular log.
func Debugf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelDebug {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelDebug, Ctx: ctx}).Output()
	}
}

// Info logs an Info level log.
func Info(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelInfo {
		(&entry{Message: msg, Level: LevelInfo, Ctx: ctx}).Output()
	}
}

// Infof logs an Info level log with the given context.
func Infof(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelInfo {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelInfo, Ctx: ctx}).Output()
	}
}

// Notice logs a Notice level log with the given context.
func Notice(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelNotice {
		(&entry{Message: msg, Level: LevelNotice, Ctx: ctx}).Output()
	}
}

// Noticef logs a Notice level log with the given context.
func Noticef(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelNotice {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelNotice, Ctx: ctx}).Output()
	}
}

// Warn logs a Warning log with the given context.
func Warn(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelWarning {
		(&entry{Message: msg, Level: LevelWarning, Ctx: ctx}).Output()
	}
}

// Warnf logs a Warning log with the given context.
func Warnf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelWarning {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelWarning, Ctx: ctx}).Output()
	}
}

// Error logs an Error log with the given context.
func Error(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelError {
		(&entry{Message: msg, Level: LevelError, Ctx: ctx}).Output()
	}
}

// Errorf logs an Error log with the given context.
func Errorf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelError {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelError, Ctx: ctx}).Output()
	}
}

// Critical logs a Critical log with the given context.
func Critical(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelCritical {
		(&entry{Message: msg, Level: LevelCritical, Ctx: ctx}).Output()
	}
}

// Criticalf logs a Critical log with the given context.
func Criticalf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelCritical {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelCritical, Ctx: ctx}).Output()
	}
}

// Fatal logs a Critical log and exists the program.
func Fatal(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelCritical {
		(&entry{Message: msg, Level: LevelCritical, Ctx: ctx}).Output()
	}
	os.Exit(1)
}

// Fatalf logs a Critical log and exists the program.
func Fatalf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelCritical {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelCritical, Ctx: ctx}).Output()
	}
	os.Exit(1)
}

// Alert logs an Alert log with the given context.
func Alert(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelAlert {
		(&entry{Message: msg, Level: LevelAlert, Ctx: ctx}).Output()
	}
}

// Alertf logs an Alert log with the given context.
func Alertf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelAlert {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelAlert, Ctx: ctx}).Output()
	}
}

// Emergency logs an Emergency log with the given context.
func Emergency(ctx context.Context, msg string) {
	if getLoggingLevel() <= LevelEmergency {
		(&entry{Message: msg, Level: LevelEmergency, Ctx: ctx}).Output()
	}
}

// Emergencyf logs an Emergency log with the given context.
func Emergencyf(ctx context.Context, format string, a ...any) {
	if getLoggingLevel() <= LevelEmergency {
		(&entry{Message: fmt.Sprintf(format, a...), Level: LevelEmergency, Ctx: ctx}).Output()
	}
}

// SetLevel sets the minimum logging level.
func SetLevel(level LogLevel) {
	setLoggingLevel(level)
}

// SetLoggingEnvironment is used to manually configure the logging environment.
func SetLoggingEnvironment(e LoggingEnvironment) {
	setLoggingEnvironment(e)
}

// SetWriter configures the io.Writer used for log output. Default is os.Stderr.
func SetWriter(writer io.Writer) {
	logWriter.Store(writerHolder{writer: writer})
}

func getLoggingLevel() LogLevel {
	return LogLevel(loggingLevel.Load())
}

func setLoggingLevel(level LogLevel) {
	loggingLevel.Store(int64(level))
}

func getLoggingEnvironment() LoggingEnvironment {
	if value := loggingEnvironment.Load(); value != nil {
		return value.(LoggingEnvironment)
	}
	return EnvironmentLocal
}

func setLoggingEnvironment(e LoggingEnvironment) {
	loggingEnvironment.Store(e)
}

func getWriter() io.Writer {
	if value := logWriter.Load(); value != nil {
		return value.(writerHolder).writer
	}
	return os.Stderr
}
