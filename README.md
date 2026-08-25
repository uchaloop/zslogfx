# zslogfx

[![CI](https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml/badge.svg)](https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uchaloop/zslogfx.svg)](https://pkg.go.dev/github.com/uchaloop/zslogfx)
[![License: MIT](https://img.shields.io/badge/github/license/uchaloop/zslogfx)](LICENSE)

A JSON `log/slog` logger backed by zap, with global `slog.Default` installation
and Uber Fx lifecycle management.

## Installation

```bash
go get github.com/uchaloop/zslogfx
```

## Fx

```toml
[log]
level = "info"

[log.buffer]
enabled = false
```

```go
fx.New(
	confx.Module(),
	confx.Provide[zslogfx.Config]("log"),
	zslogfx.Module(),
).Run()
```

After installation, use the standard package-level API anywhere:

```go
slog.Info("service started", "port", 8080)
slog.Error("request failed", "error", err)
```

The same `*slog.Logger` is also available through Fx dependency injection.

Without a file:

```go
fx.New(
	fx.Supply(zslogfx.Config{Level: "info"}),
	zslogfx.Module(),
)
```


## Configuration

With the `log` section, supported variables are:

```text
LOG_LEVEL
LOG_CALLER_ENABLED
LOG_BUFFER_ENABLED
LOG_BUFFER_SIZE
LOG_BUFFER_FLUSH_INTERVAL
```

Caller information is enabled automatically at debug level unless explicitly
configured.

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

Static options can be supplied directly:

```go
fx.Supply(
	zslogfx.WithFields(
		slog.String("service.name", "orders"),
	),
)
```

Available options include:

- `WithLevel`
- `WithCaller`
- `WithStacktraceLevel`
- `WithFields`
- `WithWriteSyncer`
- `WithBuffer`
- `WithoutBuffer`

Without Fx:

```go
logger, err := zslogfx.Make(cfg, opts...)
if err != nil {
	return err
}
defer logger.Close()

logger.Slog.Info("service started")
```

## Output

Records are JSON with fields suitable for OpenSearch:

```text
@timestamp
log.level
log.logger
message
caller
error.stack_trace
```

Durations are encoded as milliseconds. Values implementing `slog.LogValuer`,
including `secret.Secret`, control their own representation.

## Buffering

Buffering is disabled by default. Enable it through configuration or:

```go
zslogfx.WithBuffer(4*1024*1024, time.Second)
```

Buffered records are flushed on graceful Fx shutdown. Abrupt termination can
lose records that have not yet been flushed, so enable buffering only when its
throughput benefit is needed.

## Acknowledgements

I am grateful to the authors of [zap](https://github.com/uber-go/zap), its
[`zapslog`](https://pkg.go.dev/go.uber.org/zap/exp/zapslog) bridge, and
[Uber Fx](https://github.com/uber-go/fx). Their work made this library possible.

## License

[MIT](LICENSE)
