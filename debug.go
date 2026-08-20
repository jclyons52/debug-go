// Package debug implements the namespace-matching core of the npm package
// debug (v4.3.4) — the version in ESLint 8.x's dependency graph. It does NOT
// print logs; it reproduces the DEBUG / DEBUG_COLORS namespace enable/disable
// semantics (src/common.js): a DEBUG string is split on whitespace/commas,
// each token becomes an anchored regex (`*` → `.*?`), `-`-prefixed tokens are
// skips, and Enabled(name) is true iff no skip matches and some name matches
// (or the name ends in `*`).
//
// The Go regexp translation mirrors the JS `new RegExp('^'+t+'$')` behaviour
// exactly, including the quirk that regex metacharacters in a namespace are
// interpreted (as in the original). Vendor of the original (for parity) lives
// in original/.
package debug

import (
	"regexp"
	"strings"
)

// splitRe mirrors JS /[\s,]+/.
var splitRe = regexp.MustCompile(`[\s,]+`)

// Matcher holds the compiled name/skip patterns for a DEBUG string. It is the
// debug core's createDebug.names / createDebug.skips pair.
type Matcher struct {
	names []*regexp.Regexp
	skips []*regexp.Regexp
}

// NewMatcher returns an empty (all-disabled) matcher.
func NewMatcher() *Matcher { return &Matcher{} }

// Enable parses a DEBUG namespaces string into name/skip patterns (JS
// createDebug.enable).
func (m *Matcher) Enable(namespaces string) {
	m.names = nil
	m.skips = nil
	for _, tok := range splitRe.Split(namespaces, -1) {
		if tok == "" {
			continue
		}
		// Translate `*` → `.*?`, then anchor; a leading `-` (after the
		// translation) marks a skip. Exactly like JS `new RegExp('^'+t+'$')`.
		pat := strings.ReplaceAll(tok, "*", ".*?")
		if strings.HasPrefix(pat, "-") {
			if re, err := regexp.Compile("^" + pat[1:] + "$"); err == nil {
				m.skips = append(m.skips, re)
			}
		} else {
			if re, err := regexp.Compile("^" + pat + "$"); err == nil {
				m.names = append(m.names, re)
			}
		}
	}
}

// Enabled reports whether a namespace is enabled (JS createDebug.enabled).
func (m *Matcher) Enabled(name string) bool {
	if strings.HasSuffix(name, "*") {
		return true
	}
	for _, s := range m.skips {
		if s.MatchString(name) {
			return false
		}
	}
	for _, n := range m.names {
		if n.MatchString(name) {
			return true
		}
	}
	return false
}

// Disable resets the matcher and returns the namespaces string it was
// configured with (JS createDebug.disable).
func (m *Matcher) Disable() string {
	var parts []string
	for _, n := range m.names {
		parts = append(parts, toNamespace(n))
	}
	for _, s := range m.skips {
		parts = append(parts, "-"+toNamespace(s))
	}
	m.Enable("")
	return strings.Join(parts, ",")
}

// toNamespace converts a compiled pattern back to its namespace token (JS
// toNamespace): strip delimiters and turn a trailing `.*?` back into `*`.
func toNamespace(re *regexp.Regexp) string {
	s := re.String() // Go's String() has no ^$ (those were added by us? no — we compiled with them)
	// Go String() returns the source WITHOUT anchoring? It returns the pattern
	// source exactly as compiled (with ^ and $), and WITHOUT slashes. The JS
	// version strips the leading/trailing slash; Go has none. Trim a trailing
	// `$` and leading `^`, then restore `*`.
	s = strings.TrimPrefix(s, "^")
	s = strings.TrimSuffix(s, "$")
	if strings.HasSuffix(s, ".*?") {
		s = strings.TrimSuffix(s, ".*?") + "*"
	}
	return s
}

// SelectColor reproduces createDebug.selectColor's 32-bit hash modulo the
// color-table length (JS: hash = ((hash<<5)-hash)+charCode; hash|=0;
// colors[Math.abs(hash) % n]).
func SelectColor(namespace string, ncolors int) int {
	var hash int32
	for i := 0; i < len(namespace); i++ {
		hash = ((hash << 5) - hash) + int32(namespace[i])
	}
	abs := hash
	if abs < 0 {
		abs = -abs
	}
	if ncolors <= 0 {
		return 0
	}
	return int(abs % int32(ncolors))
}
