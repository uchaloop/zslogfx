# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2026-09-18

### Added

- `SupplyOptions(opts...)` for Options that need nothing from the Fx graph; they
  travel as one value group entry and apply in the order given. A plain
  `fx.Supply` never reached `Module`, and Fx reports no unused value, so the
  option was dropped in silence - as the README's own example was.

### Changed

- `Logger.Close` is now `Logger.Sync`: it flushes what the destination buffers,
  may be called repeatedly, and leaves the logger usable. Restoring
  `slog.Default` belongs to `Module`.
- A top-level `error` attribute is written as `error.message` with the error
  text, Fx's own errors included. A scalar `error` conflicted with
  `error.stack_trace` in OpenSearch, where dotted names become objects; zap's
  `errorVerbose` and `errorCauses` are gone with it.
- `@timestamp` is RFC 3339 with nanoseconds, so the zone offset reads `+02:00`
  as ECS spells it, not `+0200`.
- Durations are encoded in nanoseconds, the unit ECS gives `event.duration`.
  Name a duration field of your own without a unit suffix.
- `Config.Level` and `WithLevel` take `debug`, `info`, `warn` (or `warning`) and
  `error`. `dpanic`, `panic` and `fatal` passed validation and then silenced the
  logger, since zapslog raises no record above zap's error level. Empty still
  means `info`.
- `Make` validates the level Config and Options add up to, so an Option
  overrides an invalid `Config.Level` instead of failing next to it.
  `Config.Validate` stays for confmaker.

### Removed

- Write buffering: `Config.Buffer`, `BufferConfig`, `WithBuffer`,
  `WithoutBuffer` and the `LOG_BUFFER_*` variables. It saved about a microsecond
  per record and cost the last records of a crash.
- `log.logger` from the documented output; nothing ever set a logger name.
- The dependency on `github.com/uchaloop/validate`, which `Config.Validate` no
  longer needs for its one check.

### Fixed

- A failed provide, supply, decorate or replace left the logger installed as
  `slog.Default`: Fx calls no error hook for those.
- A failed shutdown is reported after the last OnStop hook; that record is now
  synced too. A shutdown that succeeds still syncs once.
- Writes to a destination given to `WithWriteSyncer` are serialized, so it does
  not have to be safe for concurrent use.
- A sync error is ignored only for an `*os.File` that is not a regular file.
  `EINVAL` and `ENOTTY` used to be ignored whatever the destination, and stdout
  on macOS returns neither: it returns `EBADF` for a pipe and `ENODEV` for
  `/dev/null`.
- An `error` attribute inside a group with an empty key is normalized as well,
  because such a group is inlined.
- A `slog.LogValuer` under the `error` key, or under an empty key, is resolved
  once and handed on resolved instead of being resolved again by zapslog.
- No stack trace is collected when `WithStacktraceLevel` is not set. One was
  collected for every error record and then dropped: 1982 ns and 2 allocations
  against 490 ns and 0.
- The README's license badge, which pointed at a malformed shields.io path.

## [0.3.1] - 2026-09-17

### Added

- `Config.ConfigName` returns `log`, the default instance name for confmaker
  v0.6.2 and later: `confx.Provide[zslogfx.Config]()` reads `LOG_*` without
  naming the instance.

### Changed

- The README shows the confmaker v0.6.2 API: `confx.Provide[zslogfx.Config]()`
  and `confmaker.Manifest[zslogfx.Config]()` instead of a positional instance name.

## [0.3.0] - 2026-09-01

### Changed

- `Config.Validate` accumulates through `github.com/uchaloop/validate` instead of
  a hand-rolled slice and `errors.Join`. The messages and their order are
  unchanged, and `errors` and `fmt` are no longer imported here.
- The module is built with Go 1.27, which the new dependency requires. A module
  that depends on this one has to declare 1.27 as well.

## [0.2.1] - 2026-08-25

### Fixed

- The README described configuration as a TOML table and named a setting by its
  file key. 0.2.0 stopped reading files; the documentation had not caught up.

## [0.2.0] - 2026-08-25

### Changed

- `Validate` reports every problem at once instead of the first.

### Removed

- The `koanf` struct tags. Configuration is read from the environment only.

## [0.1.3] - 2026-08-07

### Fixed

- Ignore `EBADF` returned while syncing the default `os.Stdout` during
  shutdown. Custom `WriteSyncer` implementations still return this error so
  closed files, sockets, and other destinations are not hidden.

## [0.1.2] - 2026-08-06

### Changed

- Reworked the README as concise, user-focused documentation.

## [0.1.1] - 2026-08-05

### Changed

- Nested config tags now use structural `CALLER_` and `BUFFER_` env prefixes
  while preserving the existing `LOG_*` environment-variable names.
- Documented no-file loading through
  `confmaker/confx.ProvideNoFileDefault` and linked the opaque
  `github.com/uchaloop/secret/v2.Secret` logging example.

## [0.1.0] - 2026-08-04

### Added

- A standard `log/slog` logger backed by zap and wired into Uber Fx. Services use
  a plain `*slog.Logger`; zap is only the fast JSON backend, through Uber's
  official `go.uber.org/zap/exp/zapslog` handler.
- `Make(cfg, opts...)`: build a `*Logger` (owning the `*slog.Logger`) without Fx.
- `Module()`: install the logger as `slog.Default`, provide `*Logger` and
  `*slog.Logger`, route Fx events through it, and close it on shutdown. The logger
  is built during `fx.New`, before any component runs, so the logging sequence is
  deterministic.
- `Config` with passive `koanf`/`env` tags and `Validate` (`Level`, `Caller`,
  `Buffer`); zslogfx reads no config source itself - fill it via `confmaker/confx`
  or plain Go.
- Runtime `Option`s: `WithLevel`, `WithCaller`, `WithStacktraceLevel`,
  `WithFields`, `WithWriteSyncer`, `WithBuffer`, `WithoutBuffer`; `AsOptions`
  injects Option constructors through an Fx value group.
- One fixed JSON format, ECS-aligned for OpenSearch (`@timestamp`, `log.level`,
  `log.logger`, `message`, `error.stack_trace`, a scalar `caller`), with numeric
  millisecond durations. Values implementing `slog.LogValuer` (such as
  `secret.Secret`) are masked automatically.
- Buffering is off by default and opt-in (for high-volume pods), flushed on
  graceful shutdown.

[Unreleased]: https://github.com/uchaloop/zslogfx/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/uchaloop/zslogfx/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/uchaloop/zslogfx/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/uchaloop/zslogfx/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/uchaloop/zslogfx/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/uchaloop/zslogfx/compare/v0.1.3...v0.2.0
[0.1.3]: https://github.com/uchaloop/zslogfx/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/uchaloop/zslogfx/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/uchaloop/zslogfx/releases/tag/v0.1.1
[0.1.0]: https://github.com/uchaloop/zslogfx/releases/tag/v0.1.0
