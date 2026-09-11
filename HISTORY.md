# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Fri 11 Sep 2026 | v0.2.0

The Go 1.27 release, plus the fixes a pre-release review of the whole package
turned up. Most changes follow from the toolchain bump, either because Go 1.27
is now required to build the package at all, or because it made something golog
already did possible to do faster or more directly.

### Breaking

- **The minimum Go version is 1.27.** `go.mod`'s `go` directive moved from 1.25
  to 1.27, which `encoding/json/v2` and the standard library `uuid` package both
  require. This is a hard floor on the whole build graph: a module that requires
  golog must declare `go 1.27` or later itself. A consumer on an older toolchain
  gets a build error, not a warning.
- **golog no longer builds under `GOEXPERIMENT=nojsonv2`.** That setting selects
  the pre-v2 `encoding/json` and excludes `encoding/json/v2` and
  `encoding/json/jsontext` from the standard library, which golog now imports
  directly. `nojsonv2` is a transitional opt-out the Go team expects to remove.
- **JSON output no longer escapes `<`, `>` and `&`.** Those three characters
  used to be written as their six-character Unicode escapes and are now written
  literally, so a message containing `<b>` now reads as `<b>` in the log. It is
  still valid JSON and every parser reads it the same way; the escaping only ever
  existed because `encoding/json` v1 assumed its output might be embedded in a
  web page. `U+2028` and `U+2029` are still escaped. This is the **only**
  intentional change to the bytes the JSON formatter writes. Every other v1
  rendering is preserved deliberately, including `time.Duration` as nanoseconds,
  `omitempty` on a caller's zero number or `false`, `nil` slices and maps as
  `null`, and `[N]byte` as an array.

### Added

- **Log identifiers, from the standard library's new `uuid` package.** `Log.ID`
  is a `uuid.UUID`, and `Logger.SetIDGenerator` (plus the package-level
  `golog.SetIDGenerator`) installs the generator that fills it. It is nil by
  default, so `Log.ID` stays the zero UUID and no `id` appears in any output
  until you ask for one.

  ```go
  golog.SetIDGenerator(uuid.NewV7) // a fresh, time-sortable id per entry
  ```

  A generator that returns a fixed value tags every line of a child logger with
  the same identifier, which is the shape a per-request logger wants. The id is
  written to the plain output, to the JSON formatter as `"id"`, and to an
  installed `log/slog` logger as an `id` attribute. See
  [_examples/correlation](_examples/correlation/main.go).
- **`Fields.Get[T]` and `Log.Field[T]`**, generic methods that do the type
  assertion a `Handler` or `Formatter` author would otherwise write by hand:
  `username, ok := log.Field[string]("username")`. Go 1.27's generic methods are
  what make these methods rather than package-level functions.
- **`printer.RichText`**, `Rich` without the `os.Stdout` check, for callers that
  already know whether their destination renders color, and **`printer.AppendRich`**,
  the same into a caller's buffer.
- **`RegisterLevel` and `ReloadLevels`.** Logging no longer reads the `Levels`
  map on its hot path; it reads a snapshot taken from it. `RegisterLevel` adds or
  replaces a level and refreshes that snapshot at once, so it is safe to call
  while other goroutines are logging. Writing to `Levels` directly still works
  before logging starts (a lookup miss picks a new level up on first use), and
  `ReloadLevels` publishes a change made to an existing entry's fields by hand;
  `SetText` calls it for you.
- `"none"` and `"off"` are accepted as names of the disable level, next to
  `"disable"` and `"disabled"`.
- **`printer.WriteRichLine`** writes a colored-or-plain title followed by a body
  as exactly one `Write` per writer, and **`printer.ScanLines`** is the
  line-splitting loop behind both `Scan` methods.
- A first test suite. The library had none, so CI had never executed a single
  assertion against it.
- CI now runs `-race`, `go vet`, `gofmt`, golangci-lint and `govulncheck`,
  builds `_examples` and `_benchmarks`, tests on Windows and macOS as well as
  Linux, and cross-compiles for every target `go tool dist list` reports.

### Changed

- **The JSON formatter runs on `encoding/json/v2`.** It used to cache one
  `encoding/json.Encoder` per level behind an `RWMutex` and serialize every
  `Encode` call through a second, global mutex, because a v1 encoder is bound
  for life to the writer it was built with. `jsontext.Encoder.Reset` rebinds, so
  a single pool replaces the map and both mutexes. Serial throughput is
  unchanged (896 vs 914 ns/op, same allocation count) and concurrent throughput
  is **5.2x higher** (269 vs 1398 ns/op).
- Modernized for Go 1.27: `strings.CutLast` and `strings.Cut` in the stacktrace
  parser, `maps.Copy`, `bytes.Clone`, `slices` in the child-logger registry, and
  `testing.B.Loop` in the benchmarks. `go fix -diff ./...` is empty.
- `Level` gained `MarshalJSONTo`, which writes the level name straight into the
  encoder instead of allocating a quoted `[]byte`. `MarshalJSON` stays for
  callers that invoke it directly and for third-party encoders.
- `golang.org/x/sys` updated to v0.47.0, eight minor versions on. Dependabot had
  been silently skipping this module: `.github/dependabot.yml` carried an
  `exclude-paths` key, which is not valid for a `gomod` update entry.
- **A plain-text line is now built in full and written with a single `Write`.**
  It used to go out as six to nine separate writes (title, space, time, space,
  id, prefix, message, one per field, newline), each a syscall on an unbuffered
  file. The plain path drops from 11 to 5 allocations per three lines and is
  about a third faster on a nop writer; on a real file the difference is the
  syscall count. See Fixed for what the multiple writes did to concurrent
  loggers.
- **Fields in the plain output are sorted by key.** They were written in map
  order, which shuffled them from one line to the next. The JSON formatter was
  already deterministic.
- **`Scan` goes through the logger.** Each scanned line is printed as `Println`
  would print it: with the time prefix, the logger's prefix, and past its
  handlers and formatters, so scanned lines now reach an installed `slog` or
  `logrus` logger and the JSON formatter. Before, it wrote `time + line` straight
  to the Printer. A trailing `\r` is trimmed, for CRLF output from Windows
  children.
- **The logger's lock is no longer held while writing.** Configuration lives
  under a read-write lock that a log line takes once, briefly, to copy what it
  needs. The Printer serializes its own writes; a per-level writer set with
  `SetLevelOutput` is written under a separate lock that a logger shares with
  its clones, since `Clone` copies `LevelOutput` and a parent and child then
  share the writer. A `Formatter` may therefore call any getter on its
  `Logger`; before, `log.Logger.GetLevelOutput(...)` from inside `Format`
  deadlocked. The `Formatter` doc now says it must not log through the same
  logger.
- `LevelMetadata.Text(true)` (and `GetTextForLevel(level, true)`) always emits
  the escape codes now. It used to consult `os.Stdout` as well, so a caller that
  had checked its own writer got plain text whenever the process's stdout was
  redirected. Pass the answer of `printer.SupportsColor(w)` for the writer you
  are about to use.
- `Printer.Print` and `Printer.Println` render any value the way `fmt.Sprint`
  does; anything but a string or `[]byte` used to print as nothing.

### Fixed

- **Lines from a parent logger and its children could interleave mid-line.**
  `Child` clones the Printer, so a parent and a child held different mutexes
  over the same `os.Stdout`, and with a line split into several writes the
  pieces could cross. Iris makes every application logger a child of
  `golog.Default`, so this was reachable from any Iris app that also logged
  through the default logger. One write per line closes it.
- **`Fatal` and `Fatalf` did not exit when the logger's level was `disable`.**
  They went through `Log`, which gates on the level before reaching the exit;
  the docs on `Fatal`, `FatalLevel` and the README all promised an exit "no
  matter the level". They now exit unconditionally and print only when the level
  allows it.
- **`Clone` shared the handler slice's backing array with the original**, so a
  `Handle` on the parent and a `Handle` on a child that both landed in spare
  capacity overwrote each other. `Handle` also read the slice without the lock
  it is written under.
- **`Scan` stopped silently on a line longer than 64KB**, `bufio.Scanner`'s
  default cap, and on any read error. Lines of any length are read now, capped
  at 1MB with a ` [truncated]` marker, and a read error other than the end of the
  input is logged at the error level (`Printer.Scan` writes it as a line).
- **A value the JSON formatter could not marshal turned that line into plain
  text** in the middle of a JSON stream: a `NaN`, a channel, a function in
  `Fields`. The fields are now replaced by a single `golog_error` field carrying
  the encoder's message, and the line stays JSON.
- **Stacktraces were empty for projects under a GOPATH-style `go/src/` path.**
  The filter that dropped the standard library's frames matched on that
  substring in the file path. It now matches on the function's package path.
- **Windows enabled virtual-terminal processing on stderr only**, while the
  default output is stdout, so with stderr redirected under a legacy console the
  level titles printed as literal escape codes. Both handles are tried now, and
  the existing console flags are kept rather than overwritten.
- `Printer.Printf` with no arguments wrote the format verbatim, so `%%` stayed
  `%%`. `Printer.WriteString` stopped at the first failing writer where `Write`
  continues; the two behave the same now.
- `printer.Rich` cached whether `os.Stdout` is a terminal for the life of the
  process, and `os.Stdout` is an assignable variable that test helpers swap.
- `SetLevelOutput` decides once whether its writer renders color. It used to be
  asked on every line, which for an `*os.File` is a syscall per line.
- A recycled `Log` is now fully reset when it goes back to the pool, so it can
  neither carry a previous entry's id or fields nor pin a caller's `Fields`
  until the slot is reused.
- The correlation-id example, README and package docs derived the per-request
  logger with `Child(route)`, which caches one logger per key: two concurrent
  requests on the same route were setting each other's id. They use `Clone` now.

- **golog now compiles for every `GOOS`/`GOARCH` the toolchain supports.**
  `js/wasm`, `wasip1/wasm`, `plan9` and `aix` all failed to build:
  `printer/terminal` defined `IsTerminal` only for linux, bsd, solaris and
  windows, while `printer` called it unconditionally. The package was
  restructured around a single `//go:build unix` implementation with a
  false-returning fallback, and solaris and aix gained working detection along
  the way. The dead `appengine` build tag and the legacy `// +build` lines are
  gone.
- **Colors were dropped for terminal writers whenever the process's own stdout
  was redirected.** `printer.Rich` consulted `os.Stdout` even when called for a
  writer the caller had already checked, and did so with a syscall on every
  colored line.
- **`Printer.WriteRich` read the writer set without holding the lock** that
  `AddOutput` and `SetOutput` take to replace it. A concurrent `AddOutput` was a
  data race on a map, which is a runtime throw rather than a detector complaint.
- **Removing a child logger scrambled the order of the remaining ones.**
  Registration order was kept in a `map[int]any` and rebuilt by ranging over it,
  and Go randomizes map iteration order, so the one operation that existed to
  preserve the order was the one that destroyed it. `LastChild` read the result.
- **`SetLevelFormat` applied to the wrong entries.** The per-level formatter was
  looked up by the logger's own threshold rather than by the level of the entry
  being written, so `SetLevelFormat("info", "json")` formatted everything as
  JSON while the threshold sat at info, and nothing when it sat anywhere else.
- **`SetLevelOutput` was ignored after the first log at that level**, because
  the cached JSON encoder kept the writer it had been built with.
- A recycled `Log` could report the previous entry's `Timestamp` when
  `TimeFormat` was empty.
- `Logger.Clone` read the formatter maps without the lock that
  `RegisterFormatter` and `SetLevelFormat` take to write them.
- The Windows initializer no longer launches `cmd.exe` to read the OS build
  number. It cost a process start in every binary that imported golog and reads
  to antivirus heuristics like a program hiding a shell; `RtlGetVersion` returns
  the same information directly.
- `_benchmarks` compiles again. It still imported `kataras/pio`, dropped in
  v0.1.14, and because the directory name starts with an underscore and it had
  no `go.mod`, nothing had compiled it since. Both `_examples` and `_benchmarks`
  are now modules that CI builds.
- CI's "test the examples" step had never run anything: the script it called
  globbed a path two levels above the checkout, matched nothing and exited 0,
  and the step that followed was masked by `continue-on-error`.
- `doc.go` opened a second `/*` inside the license block. Go comments do not
  nest, so the entire BSD license was part of the package documentation and
  rendered above it on pkg.go.dev.

## Sun 24 Aug 2025 | v0.1.15

Tagged at the time but never recorded here; documented retroactively.

### Changed
- Expose the internal package as `golog/printer`, so implementations that need
  its `Printer` or its color variables can reach them.
- `Printer` gained `Clone`, so `Logger.Clone` makes a deeper copy.
- `Printer` gained `TerminalOrStdout`, `TerminalOrStderr`, `Printf` and
  `WriteString`; `Printer.Terminal` returns a `*Printer` rather than an
  `io.Writer`.
- Use `golang.org/x/sys/windows` for the console mode call instead of a manual
  syscall.

## Sun 24 Aug 2025 | v0.1.14

### Added
- **Memory Management for Child Loggers**: New methods to prevent memory leaks in long-running applications
  - `RemoveChild(key any) bool` - Remove specific child logger by key
  - `ClearChildren()` - Remove all child loggers at once
  - `ChildCount() int` - Get current number of child loggers
  - `ListChildKeys() []any` - List all child logger keys
- **Enhanced Scan Functionality**: Complete implementation of the `Scan()` method
  - Time-formatted input scanning with `bufio.Scanner`
  - Goroutine-based processing for non-blocking operation
  - Proper resource cleanup and error handling

### Changed
- **Removed External Dependencies**: Eliminated `github.com/kataras/pio` dependency
  - Replaced with lightweight internal printer system
  - Maintained 100% API compatibility
  - Reduced binary size and simplified dependency management
- **Improved Performance**: Streamlined internal architecture
  - Direct formatting via `formatLog()` method instead of complex hijacking
  - Atomic operations for thread-safe multi-writer support
  - More efficient color and styling system

## Wed 24 April 2024 | v0.1.12

Accept `slog.Attr` among with `golog.Fields` for extra log data. 

## Wed 11 November 2023 | v0.1.11

- Add integration for `"log/slog"` package:

```go
import "log/slog"

// [...]
myLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
golog.Install(myLogger) // OR just  golog.Install(slog.Default())
```

- The `InstallStd` method was removed, use the `Install` method instead, it receives `any` instead of just `ExternalLogger` now.

## Sat 29 October 2022 | v0.1.8

Add `golog.Now` variable so 3rd-parties can customize the time.Now functionality used in golog.Log.Time and Timestamp.

## Wed 09 Decemember | v0.1.6

Fix `Clone` not inherite the parent's formatters field (fixes `SetLevelFormat` on childs).

## Mo 07 September | v0.1.5

Introduce the [Formatter](https://github.com/kataras/golog/blob/main/formatter.go) interface. [Example](https://github.com/kataras/golog/tree/main/_examples/customize-output).

- Add `Logger.RegisterFormatter(Formatter)` to register a custom `Formatter`.
- Add `Logger.SetFormat(formatter string, opts ...any)` to set the default formatter for all log levels.
- Add `Logger.SetLevelFormat(levelName string, formatter string, opts ...any)` to change the output format for the given "levelName".

## Su 06 September | v0.1.3 and v0.1.4

- Add `Logger.SetLevelOutput(levelName string, w io.Writer)` to customize the writer per level.
- Add `Logger.GetLevelOutput(levelName string) io.Writer` to get the leveled output or the default one.

## Sa 15 August | v0.1.2

- `Logger.Child` accepts an `any` instead of `string`. This way you can register children for pointers without forcing to naming them. If the key is string or completes the `fmt.Stringer` interface, then it's used as prefix (like always did).

## Fr 14 August 2020 | v0.0.19

- Use locks on hijacker.

## Tu 09 June 2020 | v0.0.18

- New `SetStacktraceLimit` method. If 0 (default) all debug stacktrace will be logged, if negative the field is disabled.
- If `TimeFormat` field is empty then `timestamp` field is disabled. 

## Sa 06 June 2020 | v0.0.16

- New `Fields` type that can be passed to `Logf/Debugf/Infof/Warnf/Errorf/Fatalf` functions and set the `Log.Fields` data field (which can be retrieved through a custom `LogHandler`).
- Add `Log.Stacktrace` of new `Frame` type which holds the callers stack trace when `Debug/Debugf`.
- Add `json` struct fields to the `Log` structure.
- Update the [customize-output](_examples/customize-output) example.

## Su 17 May 2020 | v0.0.14

Add a `Warningf` method to complete the [dgraph-io/badger.Logger](https://github.com/dgraph-io/badger/blob/ef28ef36b5923f12ffe3a1702bdfa6b479db6637/logger.go#L27) interface **and set the `Prefix` text right before the log's actual message instead of the beginning of the log line.**

## Tu 28 April 2020 | v0.0.12

This release provides support for colorized log level per registered output. Log's level will be colorful for registered `io.Writer`(via `AddOutput`) that supports colors, even when the rest of the writers (e.g. files) don't.

**Breaking changes on the `Levels` map**. See the corresponding updated example for migration.

## Th 12 December 2019 | v0.0.10

- Update [pio dependency](https://github.com/kataras/pio) to version 0.0.2 as it contains a small but important bugfix for GUI apps.

## We 16 October 2019 | v0.0.9

- Set the Logger's `NewLine` on `Clone` method which `golog` makes use inside its `Child("...")` method.
- Go module (v0.0.9). 

## We 02 August 2017 | v0.0.8

Add `fatal` level and `Fatal/Fatalf` funcs.

### v0.0.6 

## Su 30 July 2017 | v0.0.6 && v0.0.7

### v0.0.6 

Add a `SetPrefix(string)` and `Child(string) *Logger`.

Default package-level logger
```go
// automatically prefixed as "Router: "
golog.Child("Router").Errorf("Route %s already exists", "/mypath")
// able to prefix the main logger or child 
golog.Child("Server").SetPrefix("HTTP Server: ").Infof("Server is running at %s", ":8080")
// Child does return a new *Logger based on its parent
srvLogger := golog.Child("Server")
```

Same for independent instances
```go
log := golog.New()
log.SetPrefix("App#1: ")

routerLogger := log.Child("Router")
routerLogger.Errorf("Route %s already exists", "/mypath")
```

Example can be found [here](_examples/child/main.go).

### v0.0.7

Users are now able to add custom or modify existing levels with an easy to remember API.

```go
package main

import (
	"github.com/kataras/golog"
)

func main() {
	// Let's add a custom level,
	//
	// It should be starting from level index 6,
	// because we have 6 built'n levels  (0 is the start index):
	// disable,
	// fatal,
	// error,
	// warn,
	// info
	// debug

	// First we create our level to a golog.Level
	// in order to be used in the Log functions.
	var SuccessLevel golog.Level = 6
	// Register our level, just three fields.
	golog.Levels[SuccessLevel] = &golog.LevelMetadata{
		Name:    "success",
		RawText: "[SUCC]",
		// ColorfulText (Green Color[SUCC])
		ColorfulText: "\x1b[32m[SUCC]\x1b[0m",
	}

	// create a new golog logger
	myLogger := golog.New()

	// set its level to the higher in order to see it
	// ("success" is the name we gave to our level)
	myLogger.SetLevel("success")

	// and finally print a log message with our custom level
	myLogger.Logf(SuccessLevel, "This is a success log message with green color")
}
```

Example can be found [here](_examples/customize-levels/new-level/main.go).

## Sa 29 July 2017 | v0.0.4 & v0.0.5

### v0.0.4
- Fix an issue occurred by previous chnages, which [pio](https://github.com/kataras/pio) appends a trailing new line.

- Add a new method `golog#NewLine` which can override the default line breaker chars "\n".

### v0.0.5

Default output is `os.Stdout` instead of `os.Stderr` now, you can change it by `golog#SetOutput`.

Users are now able to customize both raw and colorful leveled log messages' prefix, example below.

Example Code:

```go
package main

import (
    "github.com/kataras/golog"
)

func main() {

    // First argument is the raw text for outputs
    // that are not support colors,
    // second argument is the full colorful text (yes it can be different if you wish to).
    //
    // If the second argument is empty then golog will update the colorful text to the
    // default color (i.e red on ErrorText) based on the first argument.

    // Default is "[ERRO]"
    golog.ErrorText("|ERROR|", "")
    // Default is "[WARN]"
    golog.WarnText("|WARN|", "")
    // Default is "[INFO]"
    golog.InfoText("|INFO|", "")
    // Default is "[DBUG]"
    golog.DebugText("|DEBUG|", "")

    // Business as usual...
    golog.SetLevel("debug")

    golog.Println("This is a raw message, no levels, no colors.")
    golog.Info("This is an info message, with colors (if the output is terminal)")
    golog.Warn("This is a warning message")
    golog.Error("This is an error message")
    golog.Debug("This is a debug message")
}
```

> This feature has been implemented after @carlca 's suggestion, [here](https://github.com/kataras/golog/issues/2).


## Th 27 July 2017 | v0.0.3

Increase the logger's performance by reducing the use of buffers on the [pio library](https://github.com/kataras/pio)