package zslogfx

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/uchaloop/utilfx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

const optionGroup = "zslogfx_options"

type optionParams struct {
	fx.In

	// The group tag must match optionGroup (a struct tag cannot use a const).
	Options []Option `group:"zslogfx_options"`
}

type errorHook struct {
	logger atomic.Pointer[Logger]
}

func (h *errorHook) HandleError(error) {
	if logger := h.logger.Load(); logger != nil {
		_ = logger.Close()
	}
}

// Module installs one application logger as slog.Default, provides *Logger and
// *slog.Logger, routes Fx events through it, and closes it on application
// shutdown.
//
// The logger is constructed before any application component: fx.WithLogger and
// the eager fx.Invoke below both depend on it, so slog.Default is installed
// during fx.New - before OnStart hooks and application code run. This makes the
// logging sequence deterministic: no component logs before the logger is ready.
//
// Module must be passed at the fx.New root because fx.WithLogger is scoped to
// the scope it is declared in.
func Module() fx.Option {
	hook := &errorHook{}

	return fx.Options(
		fx.Provide(
			func(lc fx.Lifecycle, cfg Config, p optionParams) (*Logger, error) {
				logger, err := Make(cfg, p.Options...)
				if err != nil {
					return nil, err
				}

				previous := slog.Default()
				slog.SetDefault(logger.Slog)
				closeLogger := logger.closeFn
				logger.closeFn = func() error {
					if slog.Default() == logger.Slog {
						slog.SetDefault(previous)
					}

					return closeLogger()
				}
				hook.logger.Store(logger)

				lc.Append(fx.Hook{
					OnStop: func(context.Context) error {
						return logger.Close()
					},
				})

				return logger, nil
			},
			func(logger *Logger) *slog.Logger { return logger.Slog },
		),
		// Force the logger to be built (and slog.Default installed) even if the
		// application overrides fx event logging (e.g. fx.NopLogger).
		fx.Invoke(func(*Logger) {}),
		fx.WithLogger(func(logger *slog.Logger) fxevent.Logger {
			l := &fxevent.SlogLogger{Logger: logger}
			l.UseLogLevel(slog.LevelDebug)

			return l
		}),
		fx.ErrorHook(hook),
	)
}

// AsOptions provides Option constructors to Module through an Fx value group.
// Constructors may depend on any other values in the Fx graph.
func AsOptions(ctors ...any) fx.Option {
	return utilfx.Grouped(optionGroup, ctors...)
}
