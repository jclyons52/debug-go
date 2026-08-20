# debug-go

Go port of the **namespace-matching core** of the npm package
[`debug`](https://www.npmjs.com/package/debug) (**v4.3.4** — the version in
ESLint 8.x's dependency graph). It reproduces the DEBUG / DEBUG_COLORS
enable/disable semantics from `src/common.js`: split a DEBUG string on
whitespace/commas, compile each token to an anchored regex (`*` → `.*?`),
`-`-prefixed tokens become skips, and `Enabled(name)` is true iff no skip
matches and some name matches (or the name ends in `*`).

This is intentionally **not** a logging library — it's the pure matching core
a logger (or a port consuming `debug`'s semantics) needs. The vendored
original lives in `original/`.

## API

```go
func NewMatcher() *Matcher
func (m *Matcher) Enable(namespaces string)   // DEBUG string → name/skip rules
func (m *Matcher) Enabled(name string) bool   // is a namespace enabled?
func (m *Matcher) Disable() string            // reset; return prior rule string
func SelectColor(namespace string, n int) int // debug's 32-bit color hash index
```

The Go regexp translation mirrors JS `new RegExp('^'+t+'$')` exactly (including
the quirk that regex metacharacters in a namespace are interpreted, as in the
original).

## Parity

`go test ./...` runs the Go matcher against the real `debug` under node across
12 DEBUG strings × 18 namespace probes (wildcards, negation, `-*`, multi-token,
spaces, empty) and 6 color-hash cases — all must match exactly. A vendored
`ms` stub satisfies debug's `humanize` require (unused on the matcher path).
