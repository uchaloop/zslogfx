package zslogfx

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

type trackingWriteSyncer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	syncCall int
}

func (w *trackingWriteSyncer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Write(p)
}

func (w *trackingWriteSyncer) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncCall++

	return nil
}

func (w *trackingWriteSyncer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

func (w *trackingWriteSyncer) SyncCalls() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.syncCall
}

func TestMake_UnbufferedByDefault(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(Config{}, WithWriteSyncer(ws))
	if err != nil {
		t.Fatal(err)
	}

	logger.Slog.Info("ready", "port", 8080)
	if !strings.Contains(ws.String(), `"message":"ready"`) {
		t.Fatalf("record was not written immediately: %s", ws.String())
	}
	if ws.SyncCalls() != 0 {
		t.Fatalf("Sync called before Close: %d", ws.SyncCalls())
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
}

func TestMake_BufferIsOptInAndCloseFlushesIt(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(
		Config{Buffer: BufferConfig{Enabled: true, Size: 1024, FlushInterval: time.Hour}},
		WithWriteSyncer(ws),
	)
	if err != nil {
		t.Fatal(err)
	}

	logger.Slog.Info("buffered")
	if len(ws.String()) != 0 {
		t.Fatalf("record unexpectedly written before Close: %s", ws.String())
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ws.String(), `"message":"buffered"`) {
		t.Fatalf("Close did not flush record: %s", ws.String())
	}
	if ws.SyncCalls() != 1 {
		t.Fatalf("Sync calls = %d, want 1", ws.SyncCalls())
	}
}

func TestOptionsOverrideConfig(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(
		Config{Level: "error"},
		WithLevel("debug"),
		WithCaller(false),
		WithWriteSyncer(ws),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Slog.Debug("visible")
	if !strings.Contains(ws.String(), `"message":"visible"`) {
		t.Fatalf("WithLevel did not override Config: %s", ws.String())
	}
	if strings.Contains(ws.String(), `"caller"`) {
		t.Fatalf("WithCaller(false) did not override debug default: %s", ws.String())
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{Level: "wat"}).Validate(); err == nil {
		t.Fatal("expected invalid level error")
	}
	if err := (Config{Buffer: BufferConfig{Size: -1}}).Validate(); err == nil {
		t.Fatal("expected invalid buffer size error")
	}

	enabled := true
	if err := (Config{Caller: CallerConfig{Enabled: &enabled}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWithStacktraceLevel(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(
		Config{},
		WithWriteSyncer(zapcore.AddSync(ws)),
		WithStacktraceLevel(slog.LevelError),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Slog.Error("failed")
	if !strings.Contains(ws.String(), `"error.stack_trace"`) {
		t.Fatalf("stack trace missing: %s", ws.String())
	}
}

func TestWithFields(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(
		Config{},
		WithWriteSyncer(ws),
		WithFields(slog.String("service.version", "1.2.3")),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Slog.Info("up")
	if !strings.Contains(ws.String(), `"service.version":"1.2.3"`) {
		t.Fatalf("constant field missing: %s", ws.String())
	}
}

func TestWithWriteSyncerNilIsNoOp(t *testing.T) {
	ws := &trackingWriteSyncer{}
	// nil must not clobber a syncer already set.
	logger, err := Make(Config{}, WithWriteSyncer(ws), WithWriteSyncer(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.Slog.Info("kept")
	if !strings.Contains(ws.String(), `"message":"kept"`) {
		t.Fatalf("WithWriteSyncer(nil) clobbered the destination: %s", ws.String())
	}
}

func TestNormalizeSyncErrorIgnoresBadDescriptorOnlyForStdout(t *testing.T) {
	if err := normalizeSyncError(syscall.EBADF, true); err != nil {
		t.Fatalf("stdout EBADF was not ignored: %v", err)
	}

	err := normalizeSyncError(syscall.EBADF, false)
	if !errors.Is(err, syscall.EBADF) {
		t.Fatalf("custom syncer EBADF must be returned, got: %v", err)
	}
}
