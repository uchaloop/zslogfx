package zslogfx

import (
	"fmt"
	"log/slog"
	"strings"

	"go.uber.org/zap/zapcore"
)

// Config is the serializable logger configuration.
//
// The tags are deliberately passive: zslogfx does not read the environment.
// Applications may fill Config with github.com/uchaloop/confmaker, another
// configuration library, or plain Go code.
// Register with the explicit instance name "log" to read LOG_*.
type Config struct {
	// Level is debug, info, warn (or warning) or error; empty means info.
	Level  string       `env:"LEVEL" envDescription:"Log level: debug, info, warn, warning or error; unset or empty uses info."`
	Caller CallerConfig `envPrefix:"CALLER_"`
}

// CallerConfig controls caller annotations. Unless Enabled is explicitly set,
// caller annotations are enabled at debug level and disabled otherwise.
type CallerConfig struct {
	Enabled *bool `env:"ENABLED" envDescription:"Include caller annotations; when unset, enabled at debug level and disabled otherwise."`
}

// Validate reports a level Make would reject. It is called automatically by
// confmaker, so a bad value fails the deployment rather than the first record.
// Make validates the level it ends up with, which an Option may have replaced.
func (c Config) Validate() error {
	if len(c.Level) == 0 {
		return nil
	}

	_, err := parseLevel(c.Level)

	return err
}

// parseLevel accepts the four levels log/slog has (plus zap's "warning" for
// "warn"), rather than every level
// zapcore.ParseLevel takes: zapslog raises no record above zapcore.ErrorLevel,
// so a core at dpanic, panic or fatal level would silently drop everything.
func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return 0, fmt.Errorf("level %q is not debug, info, warn or error", level)
	}
}

type settings struct {
	level           string
	caller          *bool
	stacktraceLevel *slog.Level
	fields          []slog.Attr
	writeSyncer     zapcore.WriteSyncer
}

func settingsFromConfig(cfg Config) settings {
	return settings{
		level:  cfg.Level,
		caller: cfg.Caller.Enabled,
	}
}
