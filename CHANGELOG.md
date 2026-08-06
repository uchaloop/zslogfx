# Changelog

All notable changes to this module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this module adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/uchaloop/zslogfx/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/uchaloop/zslogfx/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/uchaloop/zslogfx/releases/tag/v0.1.1
[0.1.0]: https://github.com/uchaloop/zslogfx/releases/tag/v0.1.0
