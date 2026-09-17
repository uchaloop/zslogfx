package zslogfx

import (
	"log/slog"

	"go.uber.org/zap/zapcore"
)

// Option customizes logger construction after Config has been applied.
type Option func(*settings)

// WithLevel overrides Config.Level. An empty level, like an empty
// Config.Level, means info.
func WithLevel(level string) Option {
	return func(s *settings) {
		s.level = level
	}
}

// WithCaller overrides Config.Caller.Enabled.
func WithCaller(enabled bool) Option {
	return func(s *settings) {
		s.caller = &enabled
	}
}

// WithStacktraceLevel writes error.stack_trace on records at or above level.
// Without it no stack trace is written, and none is collected.
func WithStacktraceLevel(level slog.Level) Option {
	return func(s *settings) {
		s.stacktraceLevel = &level
	}
}

// WithFields adds slog attributes to every record. Options accumulate, so
// several calls add up.
func WithFields(attrs ...slog.Attr) Option {
	return func(s *settings) {
		s.fields = append(s.fields, attrs...)
	}
}

// WithWriteSyncer replaces stdout as the destination. A nil syncer is ignored
// (stdout stays the default). The logger serializes writes to it, so ws does
// not have to be safe for concurrent use.
func WithWriteSyncer(ws zapcore.WriteSyncer) Option {
	return func(s *settings) {
		if ws != nil {
			s.writeSyncer = ws
		}
	}
}
