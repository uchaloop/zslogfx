// Package zslogfx builds a standard log/slog logger backed by zap and wires its
// ownership into an Uber Fx application.
//
// Services use a plain *slog.Logger; zap is only the fast JSON backend, through
// Uber's official go.uber.org/zap/exp/zapslog handler. The output is one fixed
// JSON format, ECS-aligned for OpenSearch. Module installs the logger as
// slog.Default so any code can log without receiving it as an argument, and also
// provides it for explicit injection. Writes are unbuffered by default; buffering
// is opt-in.
package zslogfx

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"syscall"

	uberzapslog "go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// Logger owns the slog view of the logging core and the resources behind it.
// zap is only the backend; the logger a service uses is a plain *slog.Logger.
//
// Close is safe to call more than once. Fx applications normally do not need to
// call it directly; Module registers it with the application lifecycle.
type Logger struct {
	Slog *slog.Logger

	closeOnce sync.Once
	closeErr  error
	closeFn   func() error
}

// Make builds a logger without installing it as slog.Default.
func Make(cfg Config, opts ...Option) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	s := settingsFromConfig(cfg)
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}

	level, err := zapcore.ParseLevel(s.level)
	if err != nil {
		return nil, err
	}

	encoderCfg := defaultEncoderConfig()

	includeCaller := level == zapcore.DebugLevel
	if s.caller != nil {
		includeCaller = *s.caller
	}
	if includeCaller {
		encoderCfg.CallerKey = "caller"
	}
	if s.stacktraceLevel != nil {
		encoderCfg.StacktraceKey = "error.stack_trace"
	}

	ws := s.writeSyncer
	if ws == nil {
		ws = zapcore.AddSync(os.Stdout)
	}

	var buffered *zapcore.BufferedWriteSyncer
	if s.buffer.Enabled {
		size := s.buffer.Size
		if size == 0 {
			size = defaultBufferSize
		}
		interval := s.buffer.FlushInterval
		if interval == 0 {
			interval = defaultFlushInterval
		}
		buffered = &zapcore.BufferedWriteSyncer{
			WS:            ws,
			Size:          size,
			FlushInterval: interval,
		}
		ws = buffered
	}

	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), ws, level)

	handlerOptions := make([]uberzapslog.HandlerOption, 0, 2)
	if includeCaller {
		handlerOptions = append(handlerOptions, uberzapslog.WithCaller(true))
	}
	if s.stacktraceLevel != nil {
		handlerOptions = append(handlerOptions, uberzapslog.AddStacktraceAt(*s.stacktraceLevel))
	}

	slogLogger := slog.New(uberzapslog.NewHandler(core, handlerOptions...))
	if len(s.fields) > 0 {
		args := make([]any, len(s.fields))
		for i, attr := range s.fields {
			args[i] = attr
		}
		slogLogger = slogLogger.With(args...)
	}

	l := &Logger{
		Slog: slogLogger,
	}

	flush := core.Sync
	if buffered != nil {
		flush = buffered.Stop
	}
	l.closeFn = func() error {
		return normalizeSyncError(flush())
	}

	return l, nil
}

// Close flushes pending records and stops buffer resources, when enabled.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		if l.closeFn != nil {
			l.closeErr = l.closeFn()
		}
	})

	return l.closeErr
}

// defaultEncoderConfig is the single JSON format: ECS-aligned field names, a
// numeric (millisecond) duration so durations aggregate in OpenSearch, and a
// scalar "caller" key that does not collide with the ECS log.origin object.
func defaultEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "@timestamp",
		LevelKey:       "log.level",
		NameKey:        "log.logger",
		MessageKey:     "message",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeName:     zapcore.FullNameEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// normalizeSyncError swallows the errors a Sync returns for descriptors where
// fsync is not meaningful - terminals, pipes, /dev/null - which is the normal
// case for stdout/stderr. They do not indicate lost userspace-buffered records.
// A real file returns a different error (e.g. EIO), which passes through.
func normalizeSyncError(err error) error {
	if errors.Is(err, os.ErrInvalid) ||
		errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTTY) {
		return nil
	}

	return err
}
