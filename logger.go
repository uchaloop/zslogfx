// Package zslogfx builds a standard log/slog logger backed by zap and wires its
// ownership into an Uber Fx application.
//
// Services use a plain *slog.Logger; zap is only the fast JSON backend, through
// Uber's official go.uber.org/zap/exp/zapslog handler. The output is one fixed
// JSON format, ECS-aligned for OpenSearch. Module installs the logger as
// slog.Default so any code can log without receiving it as an argument, and also
// provides it for explicit injection. The library adds no write buffer: every
// logging call hands the record to the destination synchronously.
package zslogfx

import (
	"log/slog"
	"math"
	"os"

	uberzapslog "go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
)

// Logger wraps the *slog.Logger a service uses and the sync of the destination
// behind it. zap is only the backend.
type Logger struct {
	Slog *slog.Logger

	syncFn func() error
}

// Make builds a logger without installing it as slog.Default. It validates the
// settings Config and the Options add up to, so an Option really does override
// Config.
func Make(cfg Config, opts ...Option) (*Logger, error) {
	s := settingsFromConfig(cfg)
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}

	if len(s.level) == 0 {
		s.level = zapcore.InfoLevel.String()
	}

	level, err := parseLevel(s.level)
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

	// zapslog captures a stack trace at error level unless told otherwise, and
	// without a StacktraceKey the encoder would drop it: an unreachable level
	// skips the capture.
	stacktraceLevel := slog.Level(math.MaxInt)
	if s.stacktraceLevel != nil {
		encoderCfg.StacktraceKey = "error.stack_trace"
		stacktraceLevel = *s.stacktraceLevel
	}

	var sink zapcore.WriteSyncer = os.Stdout
	if s.writeSyncer != nil {
		sink = s.writeSyncer
	}

	// zapcore.Core does not serialize writes itself.
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), zapcore.Lock(sink), level)

	handler := uberzapslog.NewHandler(
		core,
		uberzapslog.WithCaller(includeCaller),
		uberzapslog.AddStacktraceAt(stacktraceLevel),
		// errorHandler.Handle is one more frame between slog and zapslog.
		uberzapslog.WithCallerSkip(1),
	)

	slogLogger := slog.New(errorHandler{next: handler})
	if len(s.fields) > 0 {
		args := make([]any, len(s.fields))
		for i, attr := range s.fields {
			args[i] = attr
		}
		slogLogger = slogLogger.With(args...)
	}

	return &Logger{
		Slog: slogLogger,
		syncFn: func() error {
			return normalizeSyncError(core.Sync(), sink)
		},
	}, nil
}

// Sync flushes what the destination itself buffers. The logger adds no buffer
// of its own, so no record waits for it. Sync may be called as often as the
// application likes, and the logger stays usable afterwards.
func (l *Logger) Sync() error {
	if l == nil || l.syncFn == nil {
		return nil
	}

	return l.syncFn()
}

// defaultEncoderConfig is the single JSON format: ECS-aligned field names, a
// timestamp that keeps nanoseconds and writes its offset as ECS wants it
// (+02:00, not +0200), a numeric duration in nanoseconds - the unit ECS gives
// event.duration - and a scalar "caller" key that does not collide with the
// ECS log.origin object.
func defaultEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "@timestamp",
		LevelKey:       "log.level",
		MessageKey:     "message",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeDuration: zapcore.NanosDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// normalizeSyncError swallows the error Sync returns for a file that is not a
// regular file - a pipe, a terminal, /dev/null - which is the normal case for
// stdout in a container. fsync means nothing there, and the platforms disagree
// on the error: EINVAL on Linux, EBADF or ENODEV on macOS. Writes to an
// *os.File carry no userspace buffer, so no record is lost either way. A
// regular file and any other destination report their failures.
func normalizeSyncError(err error, sink zapcore.WriteSyncer) error {
	if err == nil {
		return nil
	}

	file, ok := sink.(*os.File)
	if !ok {
		return err
	}

	info, statErr := file.Stat()
	if statErr != nil || info.Mode().IsRegular() {
		return err
	}

	return nil
}
