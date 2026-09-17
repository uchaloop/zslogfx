package zslogfx

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestModuleInstallsAndRestoresDefault(t *testing.T) {
	previous := slog.Default()
	ws := &trackingWriteSyncer{}

	var provided *slog.Logger
	app := fxtest.New(
		t,
		Module(),
		fx.Supply(Config{}),
		AsOptions(func() Option { return WithWriteSyncer(ws) }),
		fx.Populate(&provided),
	)

	if slog.Default() != provided {
		t.Fatal("Module did not install the provided logger as slog.Default")
	}
	slog.Info("global")
	if !strings.Contains(ws.String(), `"message":"global"`) {
		t.Fatalf("global slog did not use zap backend: %s", ws.String())
	}

	app.RequireStart()
	app.RequireStop()
	if slog.Default() != previous {
		t.Fatal("Module did not restore the previous slog.Default")
	}
	// A shutdown that succeeds writes no record after the OnStop hook, so one
	// sync is all it takes.
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
}

func TestModuleUninstallsOnInvokeError(t *testing.T) {
	previous := slog.Default()
	ws := &trackingWriteSyncer{}
	app := fx.New(
		Module(),
		fx.Supply(Config{}),
		AsOptions(func() Option { return WithWriteSyncer(ws) }),
		fx.Invoke(func() error {
			slog.Info("before failure")
			return context.Canceled
		}),
	)
	if app.Err() == nil {
		t.Fatal("expected build error")
	}
	if !strings.Contains(ws.String(), `"message":"before failure"`) {
		t.Fatalf("record before the failure is missing: %s", ws.String())
	}
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
	if slog.Default() != previous {
		t.Fatal("invoke error did not restore the previous slog.Default")
	}
}

func TestModuleUninstallsOnProvideError(t *testing.T) {
	previous := slog.Default()
	ws := &trackingWriteSyncer{}
	app := fx.New(
		Module(),
		fx.Supply(Config{}),
		AsOptions(func() Option { return WithWriteSyncer(ws) }),
		fx.Provide(
			func() int { return 1 },
			func() int { return 2 },
		),
	)
	if app.Err() == nil {
		t.Fatal("expected provide error")
	}
	if !strings.Contains(ws.String(), `"error.message":`) {
		t.Fatalf("provide error was not logged: %s", ws.String())
	}
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
	if slog.Default() != previous {
		t.Fatal("provide error did not restore the previous slog.Default")
	}
}

func TestSupplyOptions(t *testing.T) {
	ws := &trackingWriteSyncer{}
	app := fxtest.New(
		t,
		Module(),
		fx.Supply(Config{}),
		SupplyOptions(
			WithWriteSyncer(ws),
			WithFields(slog.String("service.name", "orders")),
		),
	)
	app.RequireStart()
	slog.Info("up")
	app.RequireStop()

	if !strings.Contains(ws.String(), `"service.name":"orders"`) {
		t.Fatalf("supplied options were not applied: %s", ws.String())
	}
}

func TestSupplyOptionsAppliesInOrder(t *testing.T) {
	ws := &trackingWriteSyncer{}
	app := fxtest.New(
		t,
		Module(),
		fx.Supply(Config{}),
		SupplyOptions(
			WithWriteSyncer(ws),
			WithLevel("error"),
			WithLevel("debug"),
		),
	)
	app.RequireStart()
	slog.Debug("last option wins")
	app.RequireStop()

	if !strings.Contains(ws.String(), `"message":"last option wins"`) {
		t.Fatalf("the Options of one call were not applied in order: %s", ws.String())
	}
}

func TestModuleLogsStopErrorAfterUninstall(t *testing.T) {
	ws := &trackingWriteSyncer{}
	app := fxtest.New(
		t,
		Module(),
		fx.Supply(Config{}),
		AsOptions(func() Option { return WithWriteSyncer(ws) }),
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStop: func(context.Context) error { return errors.New("stop hook failed") },
			})
		}),
	)

	app.RequireStart()
	if err := app.Stop(context.Background()); err == nil {
		t.Fatal("expected stop error")
	}
	if !strings.Contains(ws.String(), `"message":"stop failed","error.message":"stop hook failed"`) {
		t.Fatalf("stop error was not logged: %s", ws.String())
	}
	// This record is written after the last OnStop hook, so the failure has to
	// be synced on top of the hook's own sync.
	if ws.SyncCalls() != 2 {
		t.Fatalf("Sync calls = %d, want 2", ws.SyncCalls())
	}
}

func TestModuleWithoutOptions(t *testing.T) {
	// The options value group has zero contributors; the graph must still be
	// valid. ValidateApp checks the graph without running constructors, so no
	// logger is installed and nothing is written.
	if err := fx.ValidateApp(fx.Supply(Config{}), Module()); err != nil {
		t.Fatalf("Module without AsOptions: %v", err)
	}
}
