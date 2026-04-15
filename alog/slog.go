package alog

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// slogHandler implements [slog.Handler] by forwarding each record through alog's [entry] output
// (Google JSON or local ANSI), merging attributes into the log line via [WithLogFields].
type slogHandler struct {
	opts   slog.HandlerOptions
	attrs  []slog.Attr
	groups []string
}

// alogLeveler implements [slog.Leveler] by reading [loggingLevel]. It is used when
// [slog.HandlerOptions.Level] is nil so the slog logger stays aligned with [SetLevel].
type alogLeveler struct{}

func (alogLeveler) Level() slog.Level {
	return slog.Level(loggingLevel)
}

// NewSlogLogger returns a [*slog.Logger] whose records are written using the same path as
// [Info], [Warn], and other alog functions: [SetLevel], [SetLoggingEnvironment], [SetWriter],
// and context helpers ([WithCloudTraceContext], [WithLogFields], etc.) all apply.
//
// opts may be nil; in that case default [slog.HandlerOptions] are used.
// If opts.Level is nil, it is set to an internal [slog.Leveler] derived from [loggingLevel]
// ([SetLevel]) so slog's level gate matches alog. If you set opts.Level yourself, that
// leveler is applied first, then alog's minimum level still applies.
func NewSlogLogger(opts *slog.HandlerOptions) *slog.Logger {
	var o slog.HandlerOptions
	if opts != nil {
		o = *opts
	}
	if o.Level == nil {
		o.Level = alogLeveler{}
	}
	return slog.New(&slogHandler{opts: o})
}

func (h *slogHandler) clone() *slogHandler {
	h2 := *h
	h2.attrs = append([]slog.Attr{}, h.attrs...)
	h2.groups = append([]string{}, h.groups...)
	return &h2
}

func (h *slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts.Level != nil {
		// alogDerivedLeveler does not use slog's level >= minLevel rule; filtering matches
		// package alog via loggingLevel <= slogLevelToLogLevel (see slogLevelToLogLevel).
		if _, ok := h.opts.Level.(alogLeveler); !ok {
			if level < h.opts.Level.Level() {
				return false
			}
		}
	}
	return loggingLevel <= slogLevelToLogLevel(level)
}

func (h *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx == nil {
		ctx = context.Background()
	}

	fields := make(map[string]any)
	prefix := groupPrefix(h.groups)

	for _, a := range h.attrs {
		a = h.applyReplaceAttr(a)
		if a.Key != "" || a.Value.Kind() == slog.KindGroup {
			attrToMap(a, prefix, fields)
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		a = h.applyReplaceAttr(a)
		if a.Key != "" || a.Value.Kind() == slog.KindGroup {
			attrToMap(a, prefix, fields)
		}
		return true
	})

	if h.opts.AddSource {
		if src := r.Source(); src != nil && src.File != "" {
			fields["src"] = fmt.Sprintf("%s:%d", src.File, src.Line)
		} else if r.PC != 0 {
			fs := runtime.CallersFrames([]uintptr{r.PC})
			f, _ := fs.Next()
			fields["src"] = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
	}

	if len(fields) > 0 {
		ctx = mergeLogFields(ctx, fields)
	}

	lvl := slogLevelToLogLevel(r.Level)
	e := entry{Message: r.Message, Level: lvl, Ctx: ctx}
	return e.Output()
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := h.clone()
	h2.attrs = append(h2.attrs, attrs...)
	return h2
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *slogHandler) applyReplaceAttr(a slog.Attr) slog.Attr {
	if h.opts.ReplaceAttr == nil {
		return a
	}
	return h.opts.ReplaceAttr(h.groups, a)
}

func groupPrefix(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	return strings.Join(groups, ".") + "."
}

// slogLevelToLogLevel maps [slog.Level] to the nearest [LogLevel] on alog's scale.
func slogLevelToLogLevel(l slog.Level) LogLevel {
	switch {
	case l < slog.LevelInfo:
		return LevelDebug
	case l < 4: // slog.LevelWarn
		return LevelInfo
	case l < slog.LevelError:
		return LevelWarning
	case l < 10:
		return LevelError
	case l < 12:
		return LevelCritical
	case l < 14:
		return LevelAlert
	default:
		return LevelEmergency
	}
}

func attrToMap(a slog.Attr, prefix string, out map[string]any) {
	a.Value = a.Value.Resolve()
	key := prefix + a.Key
	switch a.Value.Kind() {
	case slog.KindString:
		out[key] = a.Value.String()
	case slog.KindInt64:
		out[key] = a.Value.Int64()
	case slog.KindUint64:
		out[key] = a.Value.Uint64()
	case slog.KindFloat64:
		out[key] = a.Value.Float64()
	case slog.KindBool:
		out[key] = a.Value.Bool()
	case slog.KindTime:
		out[key] = a.Value.Time()
	case slog.KindDuration:
		out[key] = a.Value.Duration()
	case slog.KindAny:
		out[key] = a.Value.Any()
	case slog.KindGroup:
		gp := prefix
		if a.Key != "" {
			gp = key + "."
		}
		for _, g := range a.Value.Group() {
			attrToMap(g, gp, out)
		}
	default:
		if a.Key != "" {
			out[key] = a.Value.Any()
		}
	}
}

func mergeLogFields(ctx context.Context, extra map[string]any) context.Context {
	if len(extra) == 0 {
		return ctx
	}
	prev, _ := ctx.Value(logFieldsKey{}).(map[string]any)
	merged := make(map[string]any, len(prev)+len(extra))
	for k, v := range prev {
		merged[k] = v
	}
	for k, v := range extra {
		if _, ok := merged[k]; !ok {
			merged[k] = v
		}
	}
	return context.WithValue(ctx, logFieldsKey{}, merged)
}
