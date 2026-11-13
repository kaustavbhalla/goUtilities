# gogrep & gowc — PRD

Nov 5, 2026 · @kaustav

## Overview & goals

Build two Unix tools in Go, `gowc` and `gogrep`, by hand from the docs, as a first serious Go project. The tools are deliberately small so that the effort goes into Go itself: streaming I/O, interfaces, errors, testing and profiling.

**Learning outcomes** (the real deliverable):

- Write idiomatic Go: modules, packages, exported names, error values, `gofmt`/`go vet` clean.
- Understand `io.Reader`/`io.Writer` composition well enough to explain why the tools never need to know whether input is a file, stdin or a test string.
- Use `bufio` correctly, including its buffer-size limits and the `Scanner` vs `Reader` trade-off.
- Handle text properly: bytes vs runes vs lines, UTF-8.
- Write table-driven tests, subtests, fuzz tests and benchmarks; read a CPU profile.

**Rule:** no AI-written code. Docs, the stdlib source and your own experiments only.

## Scope

V1 is two separate binaries in one Go module. They cover the commonly used subset of POSIX `wc` and `grep`, not GNU's full flag set.

**In scope**

- Reading from one or more files, or stdin when no file (or `-`) is given
- POSIX-style flags parsed with the stdlib `flag` package
- Correct exit codes and errors written to stderr
- Unit tests, fuzz tests and benchmarks for every core function
- A shared internal package for anything both tools need

**Out of scope for v1**

- Third-party libraries (stdlib only)
- GNU long options (`--count`), colour output, locale support beyond UTF-8
- Binary-file detection, compressed input, memory-mapped I/O
- Performance parity with GNU tools (measure it, don't chase it)

## gowc requirements

`gowc [-clmw] [file...]` prints newline, word and byte counts per file, plus a total line when given more than one file. Behaviour should match POSIX `wc`; diff against your system's `wc` to check.

| Flag | Counts     | Definition to implement                                           |
| ---- | ---------- | ----------------------------------------------------------------- |
| `-l` | Lines      | Number of `\n` bytes (a last line with no newline is not counted) |
| `-w` | Words      | Maximal runs of non-whitespace, using `unicode.IsSpace`           |
| `-c` | Bytes      | Raw byte count                                                    |
| `-m` | Characters | UTF-8 runes; invalid bytes count as one each                      |
| none | Default    | Same as `-lwc`, in that order                                     |

**Behaviour**

1. Output columns are right-aligned and always in the order lines, words, chars/bytes, regardless of flag order.
2. With no file arguments, or `-`, read stdin and print no filename.
3. A missing or unreadable file prints `gowc: <name>: <error>` to stderr, the tool continues with the other files, and exits with status 1.
4. Counting must be single-pass and streaming: memory use stays constant for a 10 GB file.
5. Words split across a buffer boundary must still count once. This is the main correctness trap.

## gogrep requirements

`gogrep [flags] PATTERN [file...]` prints the lines that match a regular expression. Patterns use Go's RE2 syntax via `regexp`, which guarantees linear-time matching. That means no backreferences, and you should be able to explain why.

| Flag                     | Behaviour                                                                                 |
| ------------------------ | ----------------------------------------------------------------------------------------- |
| `-i`                     | Case-insensitive match (hint: RE2's `(?i)` flag)                                          |
| `-v`                     | Print non-matching lines                                                                  |
| `-n`                     | Prefix each line with its 1-based line number                                             |
| `-c`                     | Print only the count of matching lines per file                                           |
| `-l`                     | Print only names of files with at least one match; stop reading a file at its first match |
| `-F`                     | Treat PATTERN as a fixed string, not a regex                                              |
| `-r`                     | Recurse into directories                                                                  |
| `-A n` / `-B n` / `-C n` | Print n lines of context after / before / around each match, with `--` between groups     |

**Behaviour**

1. With more than one file (or `-r`), prefix output lines with `filename:`.
2. Exit status 0 if any line matched, 1 if none matched, 2 on an error (bad regex, unreadable file). Match GNU's behaviour here.
3. Lines longer than the default `bufio.Scanner` buffer (64 KiB) must not crash or be silently dropped.
4. `-B` context needs a fixed-size ring buffer of previous lines, not a slice that grows.
5. `-r` walks with `filepath.WalkDir`, skips unreadable entries with a stderr warning, and does not follow symlinks by default.

## Architecture & design constraints

Core logic is a pure function from an `io.Reader` to results written to an `io.Writer`. `main` only parses flags, opens files and sets exit codes. That split is what makes everything testable without touching the filesystem.

**Suggested module layout**

- `go.mod` declaring one module
- `cmd/gowc/main.go` and `cmd/gogrep/main.go`: thin entry points
- `internal/wc/`: counting logic + its tests
- `internal/grep/`: matching, context buffer, output formatting + tests
- `internal/cli/`: shared helpers (open file or stdin, error printing)
- `testdata/`: fixture files (empty file, no trailing newline, CRLF, UTF-8, a very long line)

**Data flow for both tools**

1. Each file name (or stdin) becomes an `io.Reader`.
2. That reader is wrapped in `bufio` for buffered reading.
3. A core function consumes it and writes formatted results to an `io.Writer` (stdout in production, a `bytes.Buffer` in tests).
4. `main` collects errors, prints them to stderr, and picks the exit code.

**Constraints**

- Core functions accept interfaces (`io.Reader`, `io.Writer`), never `*os.File`.
- Wrap stdout in a `bufio.Writer` and remember to `Flush`. Measure the difference.
- Errors are values: wrap with `fmt.Errorf("...: %w", err)`, inspect with `errors.Is`. No panics for expected failures.
- Define a small options struct per tool instead of passing many booleans.
- Code passes `gofmt`, `go vet` and `go test -race` before every commit.

## Testing & benchmarking

Every core function ships with table-driven tests that feed it `strings.NewReader` input, so no test touches the filesystem except fixture tests.

**Required tests**

- Table-driven unit tests with named subtests (`t.Run`) covering: empty input, no trailing newline, only whitespace, CRLF, multibyte UTF-8, invalid UTF-8, a line over 64 KiB.
- A buffer-boundary test for `gowc`: wrap input in `iotest.OneByteReader` so every word straddles a read boundary.
- An error-path test using `iotest.ErrReader` to prove read errors are reported, not swallowed.
- Golden-file tests: compare output against `.golden` files in `testdata/`.
- At least one fuzz target per tool (`FuzzCount`, `FuzzMatch`). For example, check that `-v` output plus normal output always equals the input line count.

**Required benchmarks**

- `BenchmarkCount` and `BenchmarkGrep` over a generated 100 MB input, reporting MB/s with `b.SetBytes`.
- Compare `bufio.Scanner` vs `bufio.Reader.ReadSlice`, and buffered vs unbuffered stdout. Record the numbers in the README.
- Profile one benchmark with `-cpuprofile` and inspect it with `go tool pprof`. Write down the top hotspot and what you changed.
- Use `benchstat` to compare before/after runs instead of eyeballing single runs.

Target: at least 80% statement coverage (`go test -cover`) on `internal/`.

## Milestones & what to read for each

Work through six milestones in order, reading the linked docs before writing that milestone's code. Each one ends with something you can run and test.

- [x] **M0 — Go basics and project setup.** Read [A Tour of Go](https://go.dev/tour/), [How to Write Go Code](https://go.dev/doc/code), [Tutorial: Create a module](https://go.dev/doc/tutorial/create-module) and [Organizing a Go module](https://go.dev/doc/modules/layout). Done when `go run ./cmd/gowc` prints hello and `go vet ./...` is clean.
- [ ] **M1 — `gowc -c -l` from stdin.** Read [`io`](https://pkg.go.dev/io) (especially the `Reader` contract: n > 0 and err != nil can happen together), [`bufio`](https://pkg.go.dev/bufio), [`os`](https://pkg.go.dev/os) and [`flag`](https://pkg.go.dev/flag). Done when counts match `wc` on the `testdata/` fixtures.
- [ ] **M2 — Words and characters, plus tests.** Read [Strings, bytes, runes and characters in Go](https://go.dev/blog/strings), [`unicode/utf8`](https://pkg.go.dev/unicode/utf8), [`unicode`](https://pkg.go.dev/unicode), [`testing`](https://pkg.go.dev/testing), [Using Subtests and Sub-benchmarks](https://go.dev/blog/subtests), [Table-driven tests](https://go.dev/wiki/TableDrivenTests) and [`testing/iotest`](https://pkg.go.dev/testing/iotest). Done when the buffer-boundary test passes.
- [ ] **M3 — Multiple files, errors, exit codes.** Read [Error handling and Go](https://go.dev/blog/error-handling-and-go), [Errors are values](https://go.dev/blog/errors-are-values), [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors), [`errors`](https://pkg.go.dev/errors) and [`fmt`](https://pkg.go.dev/fmt) (verbs and width for alignment). Done when `gowc` is feature-complete.
- [ ] **M4 — `gogrep` core: `-i -v -n -c -l -F`.** Read [`regexp`](https://pkg.go.dev/regexp), [RE2 syntax](https://github.com/google/re2/wiki/Syntax), [`regexp/syntax`](https://pkg.go.dev/regexp/syntax), [`bytes`](https://pkg.go.dev/bytes) and [`strings`](https://pkg.go.dev/strings). Also read `bufio.Scanner.Buffer` and `bufio.Reader.ReadSlice` closely for the long-line requirement.
- [ ] **M5 — Context lines and recursion.** Read [Go Slices: usage and internals](https://go.dev/blog/slices-intro) (for the ring buffer), [`path/filepath`](https://pkg.go.dev/path/filepath) (`WalkDir`) and [`io/fs`](https://pkg.go.dev/io/fs). Optionally use [`testing/fstest`](https://pkg.go.dev/testing/fstest) to test recursion against an in-memory filesystem.
- [ ] **M6 — Fuzz, benchmark, profile.** Read [Tutorial: Getting started with fuzzing](https://go.dev/doc/tutorial/fuzz), [Go Fuzzing](https://go.dev/doc/security/fuzz), the [benchmarks section of `testing`](https://pkg.go.dev/testing#hdr-Benchmarks), [Profiling Go Programs](https://go.dev/blog/pprof), [Diagnostics](https://go.dev/doc/diagnostics) and [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat). Done when the README has before/after numbers.

## Deeper reading

These aren't tied to one milestone. Read them alongside the build to understand the _why_ behind the design.

| Read                                                                                                                                                                      | Why it matters here                                                                 |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [Effective Go](https://go.dev/doc/effective_go)                                                                                                                           | The idioms reviewers expect; reread after M3                                        |
| [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)                                                                                                         | Naming, error strings, receiver names                                               |
| [The Go Programming Language Specification](https://go.dev/ref/spec)                                                                                                      | Short and precise; useful given your interpreter work                               |
| [Go source: `bufio/scan.go`](https://cs.opensource.google/go/go/+/master:src/bufio/scan.go)                                                                               | See how `Scanner` and `ScanWords` actually work before writing your own split logic |
| [Go source: `io/io.go`](https://cs.opensource.google/go/go/+/master:src/io/io.go)                                                                                         | `io.Copy`, `MultiReader`, `TeeReader` show composition in about 700 lines           |
| [Regular Expression Matching Can Be Simple And Fast](https://swtch.com/~rsc/regexp/regexp1.html) (Russ Cox)                                                               | Why Go's `regexp` is linear-time and has no backreferences                          |
| [Why GNU grep is fast](https://lists.freebsd.org/pipermail/freebsd-current/2010-August/019310.html) (Mike Haertel)                                                        | Ideas to try for stretch-goal speedups                                              |
| [POSIX `wc`](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/wc.html) and [POSIX `grep`](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/grep.html) | The behaviour spec you're implementing                                              |
| [GNU grep manual](https://www.gnu.org/software/grep/manual/grep.html)                                                                                                     | Exact context-line and exit-status behaviour                                        |

Tip: `go doc bufio.Scanner.Buffer` in the terminal shows the same docs offline, and `go doc -src` shows the source.

## Acceptance criteria & stretch goals

V1 is done when all of the following hold:

- [ ] Output of `gowc` and `gogrep` matches the system `wc`/`grep` on every `testdata/` fixture, checked by a script that diffs them
- [ ] Exit codes match the requirements tables above
- [ ] A 1 GB file is processed with flat memory use (check with `/usr/bin/time -v`)
- [ ] `go test -race -cover ./...` passes with ≥ 80% coverage on `internal/`
- [ ] Fuzz targets run for 5 minutes each with no crash
- [ ] README records benchmark numbers and one profiling finding
- [ ] You can explain, without notes, how `io.Reader` composition makes the core testable

**Stretch goals** (each introduces one new Go concept)

- Parallel `gogrep -r` with a bounded worker pool, keeping output in file order (goroutines, channels, `sync.WaitGroup`)
- `-m` max-count and Ctrl+C handling via [`os/signal`](https://pkg.go.dev/os/signal) and [`context`](https://pkg.go.dev/context)
- A literal-prefix fast path using `bytes.Index` before running the regex, measured with `benchstat`
- Colour highlighting of matches using `FindAllIndex`, only when stdout is a terminal
