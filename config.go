package zslogfx

import (
	"log/slog"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/uchaloop/validate"
)

const (
	defaultBufferSize    = 4 * 1024 * 1024
	defaultFlushInterval = time.Second
)

// Config is the serializable logger configuration.
//
// The tags are deliberately passive: zslogfx does not read the environment.
// Applications may fill Config with github.com/uchaloop/confmaker, another
// configuration library, or plain Go code.
type Config struct {
	Level  string       `env:"LEVEL"`
	Caller CallerConfig `envPrefix:"CALLER_"`
	Buffer BufferConfig `envPrefix:"BUFFER_"`
}

// CallerConfig controls caller annotations. Unless Enabled is explicitly set,
// caller annotations are enabled at debug level and disabled otherwise.
type CallerConfig struct {
	Enabled *bool `env:"ENABLED"`
}

// BufferConfig controls optional write buffering. Buffering is disabled by
// default. Size and FlushInterval use zap defaults when left at zero.
type BufferConfig struct {
	Enabled       bool          `env:"ENABLED"`
	Size          int           `env:"SIZE"`
	FlushInterval time.Duration `env:"FLUSH_INTERVAL"`
}

// ConfigName is the default instance name, "log": a loader such as confmaker
// reads LOG_LEVEL and the rest of LOG_* unless the application names the
// instance itself.
func (Config) ConfigName() string { return "log" }

// Validate checks values that can be validated independently of runtime
// Options. It is called automatically by confmaker, and reports every
// problem at once rather than the first: a deployment is fixed in a config map
// and rolled out, so one report is one round trip.
func (c Config) Validate() error {
	var errs validate.Errors

	if len(c.Level) != 0 {
		if _, err := zapcore.ParseLevel(c.Level); err != nil {
			errs.Addf("level: %w", err)
		}
	}

	errs.Require(c.Buffer.Size >= 0, "buffer.size must not be negative")
	errs.Require(c.Buffer.FlushInterval >= 0, "buffer.flush_interval must not be negative")

	return errs.Err()
}

type settings struct {
	level           string
	caller          *bool
	stacktraceLevel *slog.Level
	buffer          BufferConfig
	fields          []slog.Attr
	writeSyncer     zapcore.WriteSyncer
}

func settingsFromConfig(cfg Config) settings {
	level := cfg.Level
	if len(level) == 0 {
		level = zapcore.InfoLevel.String()
	}

	return settings{
		level:  level,
		caller: cfg.Caller.Enabled,
		buffer: cfg.Buffer,
	}
}
