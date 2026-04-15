package alog_test

import (
	"context"
	"go.alis.build/alog"
)

func ExampleDebug() {

	// no need to instantiate a logger, simply import and use the methods.
	ctx := context.Background()

	// We first need to change the minimum log level from the default (INFO) to DEBUG
	alog.SetLevel(alog.LevelDebug)

	alog.Debug(ctx, "Some debug message")
	// Outputs a JSON structure similar to:
	// {"message":"Some debug message", "severity":"DEBUG", "time":"...", "logging.googleapis.com/sourceLocation":{...}}
}

func ExampleSetLevel() {

	ctx := context.Background()

	// Set the minimum logging level to DEFAULT
	alog.SetLevel(alog.LevelWarning)

	// The following will not print given that the minimum logging level of WARNING
	alog.Info(ctx, "Some info message which will not print given the minimum logging level")

	alog.Error(ctx, "Some error message which will print given the minimum logging level")
	// Outputs a JSON structure similar to:
	// {"message": "Some error message which will print given the minimum logging level", "severity": "ERROR", "time":"..."}
}
func ExampleSetLevel_noLog() {

	ctx := context.Background()

	// Set the minimum logging level to DEFAULT
	alog.SetLevel(alog.LevelWarning)

	alog.Info(ctx, "Some info message which will not print given the minimum logging level")
	// Output:
	//
}

func ExampleInfof() {
	ctx := context.Background()

	// Using the 'f' style from the fmt.Sprintf package to print logs
	alog.Infof(ctx, "some info: %s", "the info message")
	// Outputs a JSON structure similar to:
	// {"message": "some info: the info message", "severity": "INFO", "time":"..."}
}

func ExampleSetLoggingEnvironment() {
	ctx := context.Background()

	// Set the logging environment to local
	alog.SetLoggingEnvironment(alog.EnvironmentLocal)

	alog.Info(ctx, "Some info message")
	// Outputs:
	// [32mINFO[0m Some info message
}
