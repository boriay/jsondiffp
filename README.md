# jsondiffp

`jsondiffp` is a small command-line JSON diff tool written in Go. It parses both input files as JSON, compares objects and arrays recursively, and reports differences at the deepest useful path.

By default, the CLI prints a grouped text diff. It can also print machine-readable JSON with `-j`.

## Features

- Recursive JSON object and array comparison.
- JSONPath-like paths, for example `$.items[0].name`.
- Grouped text output by nearest object or array.
- JSON output mode for scripts and automation.
- Added, removed, changed, and type-changed entries.
- Optional numeric precision with exact decimal arithmetic.
- Linux install/uninstall targets in the Makefile.

## Build

```bash
make build
```

The binary is written to:

```text
bin/jsondiffp
```

## Usage

```bash
bin/jsondiffp [flags] <left.json> <right.json>
```

Flags:

```text
-j          Print diff as JSON instead of grouped text.
-p digits   Compare numbers after rounding to this many digits after the decimal point.
-r          Use direct rounding for -p. Without -r, accumulated rounding is used.
```

Examples:

```bash
bin/jsondiffp a.json b.json
bin/jsondiffp -j a.json b.json
bin/jsondiffp -p 1 a.json b.json
bin/jsondiffp -p 1 -r -j a.json b.json
```

## Text Output

For input like:

```json
{"items":[{"name":"a"}]}
```

and:

```json
{"items":[{"name":"b"}]}
```

`jsondiffp` prints:

```text
$.items[0]
  changed .name
    left: "a"
    right: "b"
```

If there are no differences:

```text
no differences
```

## JSON Output

Use `-j`:

```bash
bin/jsondiffp -j a.json b.json
```

Example output:

```json
[
  {
    "path": "$.items[0].name",
    "type": "changed",
    "left": "a",
    "right": "b"
  }
]
```

Diff entry types:

- `changed`: both sides exist, but values differ.
- `type_changed`: both sides exist, but JSON types differ.
- `added`: value exists only on the right side.
- `removed`: value exists only on the left side.

## Numeric Precision

Without `-p`, numbers are compared exactly as parsed JSON numbers.

With `-p`, numbers are rounded before comparison. The implementation uses exact decimal arithmetic based on `math/big.Rat`, so large JSON numbers are not collapsed through `float64`.

Default mode uses accumulated rounding by decimal digits:

```bash
bin/jsondiffp -p 1 -j a.json b.json
```

For example, `289.24826` is rounded step by step:

```text
289.24826 -> 289.2483 -> 289.248 -> 289.25 -> 289.3
```

Use `-r` for direct rounding to the requested precision:

```bash
bin/jsondiffp -p 1 -r -j a.json b.json
```

In direct mode, `289.24826` rounds directly to `289.2` at precision `1`.

## Makefile Targets

```bash
make fmt
make test
make check
make build
make run
make clean
```

Run with parameters:

```bash
make run LEFT=a.json RIGHT=b.json
make run LEFT=a.json RIGHT=b.json JSON=1
make run LEFT=a.json RIGHT=b.json PRECISION=1
make run LEFT=a.json RIGHT=b.json PRECISION=1 ROUND=1 JSON=1
```

## Linux and macOS Install

Install:

```bash
make install
```

Default install prefix:

- Linux: `/usr/local`
- macOS with Homebrew: `brew --prefix` (`/opt/homebrew` on Apple Silicon, usually `/usr/local` on Intel)
- macOS without Homebrew: `/usr/local`

The binary is installed to:

```text
$(PREFIX)/bin/jsondiffp
```

Uninstall:

```bash
make uninstall
```

Install under a different prefix:

```bash
make install PREFIX=$HOME/.local
make uninstall PREFIX=$HOME/.local
```

Apple Silicon Homebrew explicitly:

```bash
make install PREFIX=/opt/homebrew
make uninstall PREFIX=/opt/homebrew
```

Package-style install with `DESTDIR`:

```bash
make install DESTDIR=/tmp/pkgroot
make uninstall DESTDIR=/tmp/pkgroot
```

## Development

Run tests:

```bash
make test
```

Format and test:

```bash
make check
```

## License

MIT. See [LICENSE](LICENSE).
