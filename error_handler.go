package zslogfx

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
)

const (
	errorKey        = "error"
	errorMessageKey = "error.message"
)

// errorHandler moves a top-level "error" attribute to "error.message".
//
// ECS makes error an object: a scalar "error" beside "error.stack_trace" is a
// mapping conflict in OpenSearch. Fx writes its own failures under "error", so
// the rename happens here rather than at call sites. The value becomes the
// error text, which also keeps zap from adding its non-ECS "errorVerbose" and
// "errorCauses" keys. An "error" group is already an object and stays as it is,
// and so does an "error" attribute inside a named group. A group with an empty
// key is inlined, which puts its attributes at the top level, so those are
// normalized too.
type errorHandler struct {
	next    slog.Handler
	grouped bool
}

func (h errorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h errorHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.grouped || !hasErrorAttr(r) {
		return h.next.Handle(ctx, r)
	}

	normalized := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(attr slog.Attr) bool {
		normalized.AddAttrs(normalizeErrorAttr(attr))

		return true
	})

	return h.next.Handle(ctx, normalized)
}

func (h errorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if !h.grouped {
		normalized := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			normalized[i] = normalizeErrorAttr(attr)
		}
		attrs = normalized
	}

	return errorHandler{next: h.next.WithAttrs(attrs), grouped: h.grouped}
}

func (h errorHandler) WithGroup(name string) slog.Handler {
	if len(name) == 0 {
		return h
	}

	return errorHandler{next: h.next.WithGroup(name), grouped: true}
}

func hasErrorAttr(r slog.Record) bool {
	found := false
	r.Attrs(func(attr slog.Attr) bool {
		found = needsNormalization(attr)

		return !found
	})

	return found
}

// needsNormalization reports whether attr has to be rewritten, without
// resolving anything: a slog.LogValuer is resolved once, in normalizeErrorAttr,
// so a value that resolves differently each time cannot slip an "error" past
// this pass and then be written as a scalar.
func needsNormalization(attr slog.Attr) bool {
	if len(attr.Key) > 0 {
		return attr.Key == errorKey
	}

	switch attr.Value.Kind() {
	case slog.KindLogValuer:
		// It may resolve to a group holding an "error".
		return true
	case slog.KindGroup:
		return slices.ContainsFunc(
			attr.Value.Group(),
			needsNormalization,
		)
	default:
		return false
	}
}

// normalizeErrorAttr renames an "error" attribute, and is the only place that
// resolves a slog.LogValuer under an empty or "error" key: it hands the
// resolved value on so zapslog does not resolve it again.
func normalizeErrorAttr(attr slog.Attr) slog.Attr {
	if attr.Key != errorKey && len(attr.Key) != 0 {
		return attr
	}

	value := attr.Value.Resolve()

	if len(attr.Key) == 0 {
		if value.Kind() != slog.KindGroup {
			return slog.Attr{Value: value}
		}

		inlined := value.Group()
		normalized := make([]slog.Attr, len(inlined))
		for i, a := range inlined {
			normalized[i] = normalizeErrorAttr(a)
		}

		return slog.Attr{Value: slog.GroupValue(normalized...)}
	}

	if value.Kind() == slog.KindGroup {
		return slog.Attr{Key: errorKey, Value: value}
	}
	if err, ok := value.Any().(error); ok {
		return slog.String(errorMessageKey, errorMessage(err))
	}

	return slog.String(errorMessageKey, value.String())
}

// errorMessage calls Error the way log/slog does: a nil pointer reads "<nil>"
// and a panic is reported in place of the text rather than crashing the caller.
func errorMessage(err error) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			if v := reflect.ValueOf(err); v.Kind() == reflect.Pointer && v.IsNil() {
				msg = "<nil>"

				return
			}
			msg = fmt.Sprintf("!PANIC: %v", r)
		}
	}()

	return err.Error()
}
