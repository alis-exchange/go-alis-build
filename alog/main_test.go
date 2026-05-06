package alog

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"
)

func init() {
	//SetLoggingEnvironment(EnvironmentLocal)
}

func TestIsGoogleEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{
			name: "local",
			want: false,
		},
		{
			name: "cloud run service",
			env: map[string]string{
				"K_SERVICE": "service",
			},
			want: true,
		},
		{
			name: "cloud run job",
			env: map[string]string{
				"CLOUD_RUN_JOB": "job",
			},
			want: true,
		},
		{
			name: "gke",
			env: map[string]string{
				"KUBERNETES_SERVICE_HOST": "10.0.0.1",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("K_SERVICE", "")
			t.Setenv("CLOUD_RUN_JOB", "")
			t.Setenv("KUBERNETES_SERVICE_HOST", "")
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			if got := isGoogleEnvironment(); got != tt.want {
				t.Fatalf("isGoogleEnvironment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogLevelFromEnv(t *testing.T) {
	t.Run("uses fallback when unset", func(t *testing.T) {
		t.Setenv("ALOG_LEVEL", "")

		if got := logLevelFromEnv("ALOG_LEVEL", LevelInfo); got != LevelInfo {
			t.Fatalf("logLevelFromEnv() = %v, want %v", got, LevelInfo)
		}
	})

	t.Run("uses configured level", func(t *testing.T) {
		t.Setenv("ALOG_LEVEL", "-4")

		if got := logLevelFromEnv("ALOG_LEVEL", LevelInfo); got != LevelDebug {
			t.Fatalf("logLevelFromEnv() = %v, want %v", got, LevelDebug)
		}
	})

	t.Run("panics on invalid level", func(t *testing.T) {
		t.Setenv("ALOG_LEVEL", "debug")

		defer func() {
			if recover() == nil {
				t.Fatal("logLevelFromEnv() did not panic")
			}
		}()

		_ = logLevelFromEnv("ALOG_LEVEL", LevelInfo)
	})
}

func TestConcurrentLoggingConfiguration(t *testing.T) {
	prevW := getWriter()
	prevEnv := getLoggingEnvironment()
	prevLevel := getLoggingLevel()
	t.Cleanup(func() {
		SetWriter(prevW)
		SetLoggingEnvironment(prevEnv)
		SetLevel(prevLevel)
	})

	SetWriter(io.Discard)

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				SetLevel(LevelDebug)
				SetLoggingEnvironment(EnvironmentLocal)
				Debug(ctx, "debug")
				SetLevel(LevelInfo)
				SetLoggingEnvironment(EnvironmentGoogle)
				Info(ctx, "info")
			}
		}()
	}

	wg.Wait()
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

func Test_applyCloudTraceFromContext(t *testing.T) {
	ctx := WithCloudTraceContext(context.Background(), "my-trace-id/my-span-id;o=1")
	os.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")

	g := &googleLogEntry{entry: &entry{}}
	applyCloudTraceFromContext(ctx, g)

	if g.Trace != "projects/my-project/traces/my-trace-id" {
		t.Errorf("Trace mismatch, got: %s", g.Trace)
	}
	if g.SpanID != "my-span-id" {
		t.Errorf("SpanID mismatch, got: %s", g.SpanID)
	}
	if !g.TraceSampled {
		t.Errorf("TraceSampled mismatch, expected true")
	}
}
