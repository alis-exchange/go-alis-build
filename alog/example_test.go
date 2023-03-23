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
	// Output:
	// {"message": "Some debug message", "Severity": "DEBUG"}
}

func ExampleSetLevel() {

	ctx := context.Background()

	// Set the minimum logging level to DEFAULT
	alog.SetLevel(alog.LevelWarning)

	// The following will not print given that the minimum logging level of WARNING
	alog.Info(ctx, "Some info message which will not print given the minimum logging level")

	alog.Error(ctx, "Some error message which will print given the minimum logging level")
	// Output:
	// {"message": "Some error message which will print given the minimum logging level", "Severity": "ERROR"}
}
func ExampleSetLevelNoLog() {

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
	// Output:
	// {"message": "some info: the info message", "Severity": "INFO"}
}

func ExampleSetLoggingEnvironment() {
	ctx := context.Background()

	// Set the logging environment to local
	alog.SetLoggingEnvironment(alog.EnvironmentLocal)

	alog.Info(ctx, "Some info message")
	// Output:
	// INFO some info message

}
