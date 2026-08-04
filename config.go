package zslogfx

import (
	"fmt"
	"log/slog"
	"time"

	"go.uber.org/zap/zapcore"
)

const (
	defaultBufferSize    = 4 * 1024 * 1024
	defaultFlushInterval = time.Second
)

// Config is the serializable logger configuration.
//
// The tags are deliberately passive: zslogfx does not read files or the
// environment. Applications may fill Config with confmaker/confx, another
// configuration library, or plain Go code. With confmaker, file values are
// decoded through the koanf tags and environment values override them through
// the env tags.
type Config struct {
	Level  string       `koanf:"level" env:"LEVEL"`
	Caller CallerConfig `koanf:"caller" envPrefix:"CALLER_"`
	Buffer BufferConfig `koanf:"buffer" envPrefix:"BUFFER_"`
}

// CallerConfig controls caller annotations. Unless Enabled is explicitly set,
// caller annotations are enabled at debug level and disabled otherwise.
type CallerConfig struct {
	Enabled *bool `koanf:"enabled" env:"ENABLED"`
}

// BufferConfig controls optional write buffering. Buffering is disabled by
// default. Size and FlushInterval use zap defaults when left at zero.
type BufferConfig struct {
	Enabled       bool          `koanf:"enabled" env:"ENABLED"`
	Size          int           `koanf:"size" env:"SIZE"`
	FlushInterval time.Duration `koanf:"flush_interval" env:"FLUSH_INTERVAL"`
}

// Validate checks values that can be validated independently of runtime
// Options. It is called automatically by confmaker/confx.
func (c Config) Validate() error {
	if len(c.Level) != 0 {
		if _, err := zapcore.ParseLevel(c.Level); err != nil {
			return fmt.Errorf("level: %w", err)
		}
	}
	if c.Buffer.Size < 0 {
		return fmt.Errorf("buffer.size must not be negative")
	}
	if c.Buffer.FlushInterval < 0 {
		return fmt.Errorf("buffer.flush_interval must not be negative")
	}

	return nil
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
