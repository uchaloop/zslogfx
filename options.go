package zslogfx

import (
	"log/slog"
	"time"

	"go.uber.org/zap/zapcore"
)

// Option customizes logger construction after Config has been applied.
type Option func(*settings)

// WithLevel overrides Config.Level.
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

// WithStacktraceLevel adds a stack trace to records at or above level.
func WithStacktraceLevel(level slog.Level) Option {
	return func(s *settings) {
		s.stacktraceLevel = &level
	}
}

// WithFields adds slog attributes to every record.
func WithFields(attrs ...slog.Attr) Option {
	return func(s *settings) {
		s.fields = append(s.fields, attrs...)
	}
}

// WithWriteSyncer replaces stdout as the destination. A nil syncer is ignored
// (stdout stays the default). It does not enable the built-in buffer; wrap the
// destination yourself when custom buffering is required.
func WithWriteSyncer(ws zapcore.WriteSyncer) Option {
	return func(s *settings) {
		if ws != nil {
			s.writeSyncer = ws
		}
	}
}

// WithBuffer enables the built-in buffer. Zero values select the defaults:
// 4 MiB and a one-second flush interval.
func WithBuffer(size int, flushInterval time.Duration) Option {
	return func(s *settings) {
		s.buffer = BufferConfig{
			Enabled:       true,
			Size:          size,
			FlushInterval: flushInterval,
		}
	}
}

// WithoutBuffer disables buffering even when Config enables it.
func WithoutBuffer() Option {
	return func(s *settings) {
		s.buffer = BufferConfig{}
	}
}
