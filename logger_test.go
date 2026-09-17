package zslogfx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// lines decodes every JSON record written so far.
func (w *trackingWriteSyncer) lines(t *testing.T) []map[string]any {
	t.Helper()

	var records []map[string]any
	for line := range strings.Lines(w.String()) {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("record is not JSON: %v: %s", err, line)
		}
		records = append(records, record)
	}

	return records
}

func TestMake_WritesImmediatelyAndSyncsOnDemand(t *testing.T) {
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
		t.Fatalf("Sync called without being asked: %d", ws.SyncCalls())
	}

	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}

	logger.Slog.Info("after sync")
	if !strings.Contains(ws.String(), `"message":"after sync"`) {
		t.Fatalf("record after Sync was not written: %s", ws.String())
	}
	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}
	if ws.SyncCalls() != 2 {
		t.Fatalf("Sync calls = %d, want 2: Sync is not once-only", ws.SyncCalls())
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
	defer logger.Sync()

	logger.Slog.Debug("visible")
	if !strings.Contains(ws.String(), `"message":"visible"`) {
		t.Fatalf("WithLevel did not override Config: %s", ws.String())
	}
	if strings.Contains(ws.String(), `"caller"`) {
		t.Fatalf("WithCaller(false) did not override debug default: %s", ws.String())
	}
}

func TestLevels(t *testing.T) {
	// zapslog raises no record above zapcore.ErrorLevel, so a zap level beyond
	// error would silence the logger instead of filtering it.
	for _, level := range []string{"", "debug", "info", "warn", "warning", "error", "INFO"} {
		if err := (Config{Level: level}).Validate(); err != nil {
			t.Errorf("Validate() rejected level %q: %v", level, err)
		}
		if _, err := Make(Config{}, WithLevel(level)); err != nil {
			t.Errorf("Make() rejected level %q: %v", level, err)
		}
	}

	for _, level := range []string{"dpanic", "panic", "fatal", "wat"} {
		if err := (Config{Level: level}).Validate(); err == nil {
			t.Errorf("Validate() accepted level %q", level)
		}
		if _, err := Make(Config{}, WithLevel(level)); err == nil {
			t.Errorf("Make() accepted level %q", level)
		}
	}
}

func TestErrorLevelStillLogs(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(Config{Level: "error"}, WithWriteSyncer(ws))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	logger.Slog.Error("failed")
	logger.Slog.Warn("dropped")
	if !strings.Contains(ws.String(), `"message":"failed"`) {
		t.Fatalf("error record is missing: %s", ws.String())
	}
	if strings.Contains(ws.String(), `"message":"dropped"`) {
		t.Fatalf("warn record passed an error-level logger: %s", ws.String())
	}
}

func TestOptionOverridesAnInvalidConfigLevel(t *testing.T) {
	if _, err := Make(Config{Level: "fatal"}, WithLevel("info")); err != nil {
		t.Fatalf("WithLevel did not override the Config level: %v", err)
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
	defer logger.Sync()

	logger.Slog.Error("failed")

	stack, _ := ws.lines(t)[0]["error.stack_trace"].(string)
	if !strings.HasPrefix(stack, "github.com/uchaloop/zslogfx.TestWithStacktraceLevel\n") {
		t.Fatalf("stack trace does not start at the logging call: %q", stack)
	}
}

func TestCallerPointsAtTheLoggingCall(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(Config{}, WithWriteSyncer(ws), WithCaller(true))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	_, file, line, _ := runtime.Caller(0)
	logger.Slog.Error("failed")

	caller, _ := ws.lines(t)[0]["caller"].(string)
	want := filepath.Base(file) + ":" + strconv.Itoa(line+1)
	if !strings.HasSuffix(caller, want) {
		t.Fatalf("caller = %q, want it to end in %q", caller, want)
	}
}

func TestTimestampAndDurationUnits(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(Config{}, WithWriteSyncer(ws))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	logger.Slog.Info("handled", slog.Duration("event.duration", time.Second))

	record := ws.lines(t)[0]
	// ECS gives event.duration in nanoseconds.
	if got := record["event.duration"]; got != float64(time.Second) {
		t.Errorf("event.duration = %v, want %v", got, float64(time.Second))
	}

	timestamp, _ := record["@timestamp"].(string)
	if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
		t.Errorf("@timestamp %q is not RFC3339Nano: %v", timestamp, err)
	}
}

func TestStacktraceIsNotCapturedByDefault(t *testing.T) {
	allocs := func(opts ...Option) float64 {
		logger, err := Make(Config{}, append([]Option{WithWriteSyncer(zapcore.AddSync(discard{}))}, opts...)...)
		if err != nil {
			t.Fatal(err)
		}
		defer logger.Sync()

		return testing.AllocsPerRun(100, func() {
			logger.Slog.Error("failed")
		})
	}

	// Compared with a capturing logger rather than zero: the race detector
	// adds allocations of its own.
	withoutStack := allocs()
	withStack := allocs(WithStacktraceLevel(slog.LevelError))
	if withoutStack >= withStack {
		t.Fatalf("allocations without a stack trace = %v, with = %v: the stack is still captured", withoutStack, withStack)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

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
	defer logger.Sync()

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
	defer logger.Sync()

	logger.Slog.Info("kept")
	if !strings.Contains(ws.String(), `"message":"kept"`) {
		t.Fatalf("WithWriteSyncer(nil) clobbered the destination: %s", ws.String())
	}
}

// unsafeSink is a destination that is not safe for concurrent use.
type unsafeSink struct {
	bytes.Buffer
}

func (*unsafeSink) Sync() error { return nil }

func TestWithWriteSyncerSerializesWrites(t *testing.T) {
	sink := &unsafeSink{}
	logger, err := Make(Config{}, WithWriteSyncer(sink))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	const records = 100
	var wg sync.WaitGroup
	for range records {
		wg.Go(func() { logger.Slog.Info("concurrent") })
	}
	wg.Wait()

	if got := strings.Count(sink.String(), `"message":"concurrent"`); got != records {
		t.Fatalf("records = %d, want %d", got, records)
	}
}

type formattedError struct{}

func (formattedError) Error() string { return "formatted" }

func (formattedError) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte("formatted\nwith detail")) }

type pointerError struct{ text string }

func (e *pointerError) Error() string { return e.text }

func TestErrorAttributeBecomesErrorMessage(t *testing.T) {
	var nilErr *pointerError

	tests := []struct {
		name string
		log  func(*slog.Logger)
		want map[string]any
	}{
		{
			name: "error value",
			log:  func(l *slog.Logger) { l.Error("failed", "error", errors.New("boom")) },
			want: map[string]any{"error.message": "boom"},
		},
		{
			name: "formatter has no verbose key",
			log:  func(l *slog.Logger) { l.Error("failed", "error", formattedError{}) },
			want: map[string]any{"error.message": "formatted"},
		},
		{
			name: "joined errors have no causes key",
			log:  func(l *slog.Logger) { l.Error("failed", "error", errors.Join(errors.New("a"), errors.New("b"))) },
			want: map[string]any{"error.message": "a\nb"},
		},
		{
			name: "string value as Fx writes it",
			log:  func(l *slog.Logger) { l.Error("failed", slog.String("error", "boom")) },
			want: map[string]any{"error.message": "boom"},
		},
		{
			name: "nil pointer error",
			log:  func(l *slog.Logger) { l.Error("failed", "error", error(nilErr)) },
			want: map[string]any{"error.message": "<nil>"},
		},
		{
			name: "logger attributes",
			log:  func(l *slog.Logger) { l.With("error", errors.New("boom")).Error("failed") },
			want: map[string]any{"error.message": "boom"},
		},
		{
			name: "error group stays an object",
			log:  func(l *slog.Logger) { l.Error("failed", slog.Group("error", slog.String("code", "E1"))) },
			want: map[string]any{"error": map[string]any{"code": "E1"}},
		},
		{
			name: "error in an inlined group",
			log: func(l *slog.Logger) {
				l.Error("failed", slog.Group("", slog.String("error", "boom"), slog.Int("status", 500)))
			},
			want: map[string]any{"error.message": "boom", "status": float64(500)},
		},
		{
			name: "error in a nested inlined group",
			log: func(l *slog.Logger) {
				l.Error("failed", slog.Group("", slog.Group("", slog.Any("error", errors.New("boom")))))
			},
			want: map[string]any{"error.message": "boom"},
		},
		{
			name: "log valuer resolving to an error",
			log:  func(l *slog.Logger) { l.Error("failed", "error", valuerError{err: errors.New("boom")}) },
			want: map[string]any{"error.message": "boom"},
		},
		{
			name: "error inside a group is not renamed",
			log:  func(l *slog.Logger) { l.WithGroup("request").Error("failed", "error", errors.New("boom")) },
			want: map[string]any{"request": map[string]any{"error": "boom"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &trackingWriteSyncer{}
			logger, err := Make(Config{}, WithWriteSyncer(ws))
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Sync()

			tt.log(logger.Slog)

			record := ws.lines(t)[0]
			for _, key := range []string{"@timestamp", "log.level", "message"} {
				delete(record, key)
			}
			if !reflect.DeepEqual(record, tt.want) {
				t.Fatalf("record = %v, want %v", record, tt.want)
			}
		})
	}
}

func TestDefaultRecordSchema(t *testing.T) {
	ws := &trackingWriteSyncer{}
	logger, err := Make(Config{Level: "debug"}, WithWriteSyncer(ws), WithStacktraceLevel(slog.LevelError))
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	logger.Slog.Error("failed", "error", errors.New("boom"))

	record := ws.lines(t)[0]
	for _, key := range []string{"@timestamp", "log.level", "message", "caller", "error.message", "error.stack_trace"} {
		if _, ok := record[key]; !ok {
			t.Errorf("field %q is missing: %v", key, record)
		}
	}
	if len(record) != 6 {
		t.Errorf("record has %d fields, want 6: %v", len(record), record)
	}
}

// valuerError reports an error through slog.LogValuer and counts its
// resolutions: zapslog resolves too, and resolving twice is one call too many.
type valuerError struct {
	err error
}

var resolutions atomic.Int64

func (v valuerError) LogValue() slog.Value {
	resolutions.Add(1)

	return slog.AnyValue(v.err)
}

// inlineValuer resolves to a group under an empty key, which inlines it.
type inlineValuer struct{}

func (inlineValuer) LogValue() slog.Value {
	resolutions.Add(1)

	return slog.GroupValue(slog.String("error", "boom"))
}

func TestLogValuerIsResolvedOnce(t *testing.T) {
	tests := []struct {
		name string
		attr slog.Attr
	}{
		{
			name: "under the error key",
			attr: slog.Any(errorKey, valuerError{err: errors.New("boom")}),
		},
		{
			// Resolving twice here would also let a value that resolves
			// differently each time hide the error from the first pass.
			name: "inlined under an empty key",
			attr: slog.Any("", inlineValuer{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &trackingWriteSyncer{}
			logger, err := Make(Config{}, WithWriteSyncer(ws))
			if err != nil {
				t.Fatal(err)
			}
			defer logger.Sync()

			resolutions.Store(0)
			logger.Slog.Error("failed", tt.attr)

			if got := resolutions.Load(); got != 1 {
				t.Errorf("LogValue calls = %d, want 1", got)
			}
			if got := ws.lines(t)[0]["error.message"]; got != "boom" {
				t.Errorf("error.message = %v, want boom: %s", got, ws.String())
			}
		})
	}
}

// failingSyncer fails every Sync with err.
type failingSyncer struct {
	err error
}

func (failingSyncer) Write(p []byte) (int, error) { return len(p), nil }

func (s failingSyncer) Sync() error { return s.err }

func TestNormalizeSyncError(t *testing.T) {
	syncErr := errors.New("sync failed")

	if err := normalizeSyncError(syncErr, failingSyncer{err: syncErr}); !errors.Is(err, syncErr) {
		t.Fatalf("custom destination error = %v, want %v", err, syncErr)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if err := normalizeSyncError(syncErr, w); err != nil {
		t.Fatalf("pipe error was not ignored: %v", err)
	}

	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := normalizeSyncError(syncErr, file); !errors.Is(err, syncErr) {
		t.Fatalf("regular file error = %v, want %v", err, syncErr)
	}
}

func TestSyncReportsDestinationError(t *testing.T) {
	syncErr := errors.New("sync failed")
	logger, err := Make(Config{}, WithWriteSyncer(failingSyncer{err: syncErr}))
	if err != nil {
		t.Fatal(err)
	}

	if err := logger.Sync(); !errors.Is(err, syncErr) {
		t.Fatalf("Sync() = %v, want %v", err, syncErr)
	}
}
