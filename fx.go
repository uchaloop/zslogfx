package zslogfx

import (
	"context"
	"log/slog"
	"sync"

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

// globalLogger owns slog.Default for the lifetime of one application: the
// Logger itself holds no global state.
type globalLogger struct {
	mu       sync.Mutex
	logger   *Logger
	previous *slog.Logger
}

func (g *globalLogger) install(logger *Logger) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.logger = logger
	g.previous = slog.Default()
	slog.SetDefault(logger.Slog)
}

// uninstall restores the previous slog.Default and syncs, once. It is called
// from every path that ends the application, so it has to tolerate repeats.
// The sync runs outside the mutex: it calls into the destination, which is the
// application's code.
func (g *globalLogger) uninstall() error {
	logger := g.take()
	if logger == nil {
		return nil
	}

	return logger.Sync()
}

// take gives up the installation and returns the logger that held it, or nil
// when another path got there first.
func (g *globalLogger) take() *Logger {
	g.mu.Lock()
	defer g.mu.Unlock()

	logger := g.logger
	if logger == nil {
		return nil
	}

	if slog.Default() == logger.Slog {
		slog.SetDefault(g.previous)
	}
	g.logger = nil
	g.previous = nil

	return logger
}

type errorHook struct {
	global *globalLogger
}

func (h errorHook) HandleError(error) {
	_ = h.global.uninstall()
}

// eventLogger routes Fx events to the logger, and it also has to watch two
// events Fx reports outside the application lifecycle:
//
// A failed provide, supply, decorate or replace makes fx.New return without
// calling error hooks, yet the logger is already built for the events, so
// without this it would stay installed as slog.Default.
//
// A shutdown that fails is reported after the last OnStop hook, so that record
// needs a sync of its own. A shutdown that succeeds writes nothing.
type eventLogger struct {
	next   fxevent.Logger
	logger *Logger
	global *globalLogger
}

func (l eventLogger) LogEvent(event fxevent.Event) {
	l.next.LogEvent(event)

	switch {
	case graphFailed(event):
		_ = l.global.uninstall()
	case shutdownFailed(event):
		_ = l.logger.Sync()
	}
}

func graphFailed(event fxevent.Event) bool {
	switch e := event.(type) {
	case *fxevent.Provided:
		return e.Err != nil
	case *fxevent.Supplied:
		return e.Err != nil
	case *fxevent.Decorated:
		return e.Err != nil
	case *fxevent.Replaced:
		return e.Err != nil
	default:
		return false
	}
}

func shutdownFailed(event fxevent.Event) bool {
	switch e := event.(type) {
	case *fxevent.Stopped:
		return e.Err != nil
	case *fxevent.RolledBack:
		return e.Err != nil
	default:
		return false
	}
}

// Module installs one application logger as slog.Default, provides *Logger and
// *slog.Logger, routes Fx events through it, and on application shutdown syncs
// it and restores the previous slog.Default. Fx events are logged at debug
// level, the failures among them at error level, in the same JSON as the rest.
//
// The logger is constructed before any application component: both the Fx event
// logger and an eager fx.Invoke depend on it, so slog.Default is installed
// during fx.New - before OnStart hooks and application code run. This makes the
// logging sequence deterministic: no component logs before the logger is ready.
//
// Module must be passed at the fx.New root because fx.WithLogger is scoped to
// the scope it is declared in. It belongs to one application at a time: two
// applications alive at once take slog.Default from each other, and an
// application that replaces the Fx event logger keeps slog.Default installed
// after a failed provide.
func Module() fx.Option {
	global := &globalLogger{}

	return fx.Options(
		fx.Provide(
			func(lc fx.Lifecycle, cfg Config, p optionParams) (*Logger, error) {
				logger, err := Make(cfg, p.Options...)
				if err != nil {
					return nil, err
				}

				global.install(logger)
				lc.Append(fx.Hook{
					OnStop: func(context.Context) error {
						return global.uninstall()
					},
				})

				return logger, nil
			},
			func(logger *Logger) *slog.Logger { return logger.Slog },
		),
		// Force the logger to be built (and slog.Default installed) even if the
		// application overrides fx event logging (e.g. fx.NopLogger).
		fx.Invoke(func(*Logger) {}),
		fx.WithLogger(func(logger *Logger) fxevent.Logger {
			l := &fxevent.SlogLogger{Logger: logger.Slog}
			l.UseLogLevel(slog.LevelDebug)

			return eventLogger{next: l, logger: logger, global: global}
		}),
		fx.ErrorHook(errorHook{global: global}),
	)
}

// AsOptions provides Option constructors to Module through an Fx value group.
// Constructors may depend on any other values in the Fx graph.
//
// A value group has no order, so Options from separate AsOptions and
// SupplyOptions calls are applied in an undefined one: an application must not
// contribute two Options that set the same thing, such as two WithLevel or two
// WithWriteSyncer.
func AsOptions(ctors ...any) fx.Option {
	return utilfx.Grouped(optionGroup, ctors...)
}

// SupplyOptions passes Options that need nothing from the Fx graph to Module.
// A plain fx.Supply does not reach it: Module reads the value group AsOptions
// and SupplyOptions contribute to. The Options of one call are applied in the
// order given; see AsOptions about the order between calls.
func SupplyOptions(opts ...Option) fx.Option {
	return AsOptions(func() Option {
		return func(s *settings) {
			for _, opt := range opts {
				if opt != nil {
					opt(s)
				}
			}
		}
	})
}
