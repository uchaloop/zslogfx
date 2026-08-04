package zslogfx

import (
	"context"
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
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
}

func TestModuleClosesOnBuildError(t *testing.T) {
	previous := slog.Default()
	ws := &trackingWriteSyncer{}
	app := fx.New(
		Module(),
		fx.Supply(Config{Buffer: BufferConfig{Enabled: true}}),
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
		t.Fatalf("build error did not flush buffer: %s", ws.String())
	}
	if slog.Default() != previous {
		t.Fatal("build error did not restore the previous slog.Default")
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
