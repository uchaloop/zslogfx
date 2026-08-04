# zslogfx

[![CI](https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml/badge.svg)](https://github.com/uchaloop/zslogfx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uchaloop/zslogfx.svg)](https://pkg.go.dev/github.com/uchaloop/zslogfx)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`zslogfx` configures a zap-backed standard `log/slog` logger and owns it for the
lifetime of an Uber Fx application. Services use a plain `*slog.Logger`; zap is
only the fast JSON backend, through Uber's official
[`go.uber.org/zap/exp/zapslog`](https://pkg.go.dev/go.uber.org/zap/exp/zapslog)
handler. Writes are unbuffered by default; buffering is explicit.

## Install

```bash
go get github.com/uchaloop/zslogfx
```

## Quick start

Load a config, install the logger, and use `slog` anywhere - including inside
`fx.Invoke` - without passing a logger through function arguments.

```toml
# config.toml
[log]
level = "info"
```

```go
package main

import (
	"log/slog"

	"github.com/uchaloop/confmaker/confx"
	"github.com/uchaloop/zslogfx"
	"go.uber.org/fx"
)

func main() {
    fx.New(
        confx.LoadModule("config.toml"),
        confx.ProvideDefault[zslogfx.Config]("log"),
        zslogfx.Module(),

        fx.Invoke(
            func() {
                // slog.Default is already the zap-backed JSON logger.
                slog.Info("service started", "port", 8080)
            },
        ),
    ).Run()
}
```

Without a config file, supply `Config` directly instead of the two `confx` lines:

```go
fx.Supply(zslogfx.Config{Level: "info"}),
zslogfx.Module(),
```

Or build the same config entirely from environment variables:

```go
confx.ProvideNoFileDefault[zslogfx.Config]("log"),
zslogfx.Module(),
```

## Configuration

`Config` carries passive `koanf` (file) and `env` (environment) tags; zslogfx
reads no source itself - the application chooses one (`confmaker/confx` above, any
other loader, or plain Go). With the `[log]` section, the environment variables
are `LOG_LEVEL`, `LOG_CALLER_ENABLED`, `LOG_BUFFER_ENABLED`, `LOG_BUFFER_SIZE`,
and `LOG_BUFFER_FLUSH_INTERVAL`; env overrides file through `confmaker/confx`.

`Module` installs the logger as `slog.Default` and also provides `*slog.Logger`
(and the `*Logger` that owns it) for code that prefers explicit injection. The
logger is built during `fx.New` - `fx.WithLogger` and an eager invoke both depend
on it - so `slog.Default` is set **before any component runs**: the logging
sequence is deterministic. Pass `Module()` at the `fx.New` root.

### Dynamic fields

To stamp a value resolved from the container (a build version, environment name,
...) onto every record, provide an `Option` constructor through `AsOptions` - it
can depend on anything in the Fx graph:

```go
// application-defined; not part of zslogfx
type BuildInfo struct{ Version string }

fx.New(
    fx.Supply(BuildInfo{Version: "1.2.3"}),
    confx.LoadModule("config.toml"),
    confx.ProvideDefault[zslogfx.Config]("log"),
    zslogfx.Module(),
    zslogfx.AsOptions(func(b BuildInfo) zslogfx.Option {
        return zslogfx.WithFields(slog.String("service.version", b.Version))
    }),
)
```

For static fields no constructor is needed - supply the option directly:
`fx.Supply(zslogfx.WithFields(slog.String("service.name", "orders")))`.
Fields are plain `slog.Attr`, so application code never imports zap.

## Format

One JSON format, ECS-aligned for OpenSearch: `@timestamp`, `log.level`,
`log.logger`, `message`, `error.stack_trace`, a scalar `caller` (kept out of the
ECS `log.origin` object so index mappings never conflict), and durations as
numeric milliseconds so they aggregate. The format is fixed - there is no encoder
override. Because logging goes through slog, a value implementing
`slog.LogValuer` (such as
[`secret/v2.Secret`](https://pkg.go.dev/github.com/uchaloop/secret/v2)) is masked
automatically.

## Buffering

Buffering is off by default. Enable it in config or with an option:

```go
zslogfx.WithBuffer(4*1024*1024, time.Second)
```

At shutdown the module flushes: with buffering it calls `BufferedWriteSyncer.Stop`
(stops the background goroutine and drains pending records); without buffering it
calls the core's `Sync`.

> **Buffering trades durability for throughput.** Records are flushed only on a
> graceful shutdown (Fx `OnStop`) or a graph-build error. A crash, `os.Exit`,
> panic, OOM-kill, or `SIGKILL` after the grace period loses up to `Size` (or one
> `FlushInterval`) of records - exactly the logs just before the failure. Leave
> buffering off unless log volume makes direct stdout writes a bottleneck (e.g. a
> busy Kubernetes pod), and make sure the app terminates through Fx within
> `terminationGracePeriodSeconds`.

## Acknowledgements

zslogfx stands on [uber-go/zap](https://github.com/uber-go/zap) (and its
`zap/exp/zapslog` bridge) and [uber-go/fx](https://github.com/uber-go/fx). Thanks
to their authors and maintainers.

## License

[MIT](LICENSE).
