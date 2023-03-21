package alog

import (
	"context"
	"testing"
)

func init() {
	SetLoggingEnvironment(EnvironmentLocal)
}

func TestAlert(t *testing.T) {

	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "ALERT1",
			args: args{
				ctx: context.Background(),
				msg: "Some alert going on here.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Alert(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestAlertf(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Alertf(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestCritical(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Critical(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestCriticalf(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Criticalf(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestEmergency(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Emergency(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestEmergencyf(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Emergencyf(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestError(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "ERROR1",
			args: args{
				ctx: context.Background(),
				msg: "Some error going on here.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Error(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestErrorf(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Errorf(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestInfo(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Info(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestInfof(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Infof(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name string
		l    LogLevel
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotice(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Notice(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestNoticef(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Noticef(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func TestSetLevel(t *testing.T) {
	type args struct {
		level LogLevel
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.args.level)
		})
	}
}

func TestSetLoggingEnvironment(t *testing.T) {
	type args struct {
		e LoggingEnvironment
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLoggingEnvironment(tt.args.e)
		})
	}
}

func TestWarn(t *testing.T) {
	type args struct {
		ctx context.Context
		msg string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Warn(tt.args.ctx, tt.args.msg)
		})
	}
}

func TestWarnf(t *testing.T) {
	type args struct {
		ctx    context.Context
		format string
		a      []any
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Warnf(tt.args.ctx, tt.args.format, tt.args.a...)
		})
	}
}

func Test_entry_String(t *testing.T) {
	type fields struct {
		Message        string
		Severity       string
		Level          LogLevel
		Trace          string
		SourceLocation logEntrySourceLocation
		Ctx            context.Context
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := entry{
				Message:        tt.fields.Message,
				Severity:       tt.fields.Severity,
				Level:          tt.fields.Level,
				Trace:          tt.fields.Trace,
				SourceLocation: tt.fields.SourceLocation,
				Ctx:            tt.fields.Ctx,
			}
			if got := e.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTrace(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTrace(tt.args.ctx); got != tt.want {
				t.Errorf("getTrace() = %v, want %v", got, tt.want)
			}
		})
	}
}
