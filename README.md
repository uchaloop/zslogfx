<p align="center">
  <a href="https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml"><img src="https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://pkg.go.dev/github.com/uchaloop/zslogfx"><img src="https://pkg.go.dev/badge/github.com/uchaloop/zslogfx.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/uchaloop/zslogfx" alt="License: MIT"></a>
</p>

# zslogfx

A JSON `log/slog` logger backed by zap, with global `slog.Default` installation
and Uber Fx lifecycle management.

## Installation

```bash
go get github.com/uchaloop/zslogfx
```

## Fx

```go
fx.New(
	confx.Module(),
	confx.Provide[zslogfx.Config]("log"),
	zslogfx.Module(),
).Run()
```

`Config` names its default instance `log`, which gives the prefix, so it reads
`LOG_LEVEL` and the rest of `LOG_*`:

```text
LOG_LEVEL=info
LOG_CALLER_ENABLED=false
```

After installation, use the standard package-level API anywhere:

```go
slog.Info("service started", "port", 8080)
slog.Error("request failed", "error", err)
```

The same `*slog.Logger` is also available through Fx dependency injection.

Fx logs through it too: its own events at `debug`, which `LOG_LEVEL=info` keeps
out of the way, and the failures of a provide, a hook or a rollback at `error`.
`Module` goes at the `fx.New` root, and one application at a time owns
`slog.Default`.

A `Config` assembled in Go works just as well, for a tool that reads no
environment at all:

```go
fx.New(
	fx.Supply(zslogfx.Config{Level: "info"}),
	zslogfx.Module(),
)
```

## Configuration

Under the `log` instance name:

| Variable | Type | When unset |
|---|---|---|
| `LOG_LEVEL` | `string` (`debug`, `info`, `warn`/`warning`, `error`) | `info` |
| `LOG_CALLER_ENABLED` | `*bool` | on at debug level, off otherwise |

An explicitly empty `LOG_CALLER_ENABLED` is invalid; set `true` or `false`,
or leave it unset for automatic behavior.

Nothing is required: a logger with no configuration at all is a working logger.
`confmaker.Manifest[zslogfx.Config]("log")` lists the same set from the type itself.

## Fields and options

Add fields resolved from the Fx container:

```go
zslogfx.AsOptions(
	func(build BuildInfo) zslogfx.Option {
		return zslogfx.WithFields(
			slog.String("service.version", build.Version),
		)
	},
)
```

Options that need nothing from the graph go through `SupplyOptions`. A plain
`fx.Supply` does not reach the module, which reads a value group:

```go
zslogfx.SupplyOptions(
	zslogfx.WithFields(
		slog.String("service.name", "orders"),
	),
)
```

The Options of one `SupplyOptions` call are applied in the order given. A value
group has none, so the order between separate `AsOptions` and `SupplyOptions`
calls is undefined: do not contribute two Options that set the same thing, such
as two `WithLevel` or two `WithWriteSyncer`.

The options:

| Option | Effect |
|---|---|
| `WithLevel` | overrides `Config.Level` |
| `WithCaller` | overrides `Config.Caller.Enabled` |
| `WithStacktraceLevel` | adds `error.stack_trace` from that level up; off unless set |
| `WithFields` | attributes added to every record |
| `WithWriteSyncer` | a destination other than stdout |

Without Fx:

```go
logger, err := zslogfx.Make(cfg, opts...)
if err != nil {
	return err
}
defer logger.Sync()

logger.Slog.Info("service started")
```

## Output

Records are JSON with fields suitable for OpenSearch:

```text
@timestamp            always
log.level             always
message               always
caller                with caller annotations
error.message         with an error attribute
error.stack_trace     with WithStacktraceLevel
```

A top-level `error` attribute is written as `error.message` with the error
text, because ECS makes `error` an object and a scalar `error` would conflict
with `error.stack_trace`. This covers the errors Fx logs itself. An `error`
group is written as an object, and an `error` attribute inside a group keeps
its key.

Timestamps are RFC 3339 with nanoseconds. Durations are encoded as
nanoseconds, the unit ECS gives `event.duration`, so name a field of your own
without a unit suffix. Values implementing `slog.LogValuer`, including
`secret.Secret`, control their own representation.

The library adds no write buffer: every logging call hands the record to the
destination synchronously, so a crash does not take a queue of records with it.
`Sync` flushes what the destination itself buffers, may be called as often as
the application likes, and leaves the logger usable. The logger serializes
writes, so a destination given to `WithWriteSyncer` does not have to be safe
for concurrent use.

## OpenSearch mapping

Dynamic mapping reads these records, but it gives the stack trace a `keyword`
sub-field it will never use, and it rounds `@timestamp` to milliseconds. An
index template settles both:

```json
{
  "mappings": {
    "properties": {
      "@timestamp": { "type": "date_nanos" },
      "log": { "properties": { "level": { "type": "keyword" } } },
      "message": { "type": "text" },
      "caller": { "type": "keyword" },
      "error": {
        "properties": {
          "message": { "type": "text" },
          "stack_trace": { "type": "text" }
        }
      }
    }
  }
}
```

A `date` field parses the timestamps as well, it just keeps milliseconds of
them. Where the cluster offers them, `match_only_text` for `message` and
`error.message` and `wildcard` for `error.stack_trace` index the same content
for less.

## Acknowledgements

I am grateful to the authors of [zap](https://github.com/uber-go/zap), its
[`zapslog`](https://pkg.go.dev/go.uber.org/zap/exp/zapslog) bridge, and
[Uber Fx](https://github.com/uber-go/fx). Their work made this library possible.

## License

[MIT](LICENSE)
