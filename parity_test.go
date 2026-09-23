package debug

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type setCase struct {
	NS    string   `json:"ns"`
	Names []string `json:"names"`
}

type colorCase struct {
	NS      string `json:"ns"`
	NColors int    `json:"n"`
}

// namespaces to probe across every set.
var probeNames = []string{
	"foo", "bar", "foo.bar", "foobar", "silly:value", "silly:1", "worker:1",
	"secret", "secret:key", "a:b", "x:y", "foo:nothing", "node:http",
	"connect:router", "foo:bar:baz", "worker", "", "silly",
}

var debugSets = []string{
	"",
	"foo",
	"foo,bar",
	"foo*",
	"silly:v*",
	"foo*,-bar",
	"*,-secret",
	"a:b, x:y",
	"worker:*",
	"-*",
	"*",
	"connect:*",
}

var colorCases = []colorCase{
	{"foo", 6}, {"foo:bar", 6}, {"worker", 6}, {"connect:router", 6},
	{"silly:value", 6}, {"a", 6},
}

// jscmd builds and runs the node driver, returning parsed out.
func jscmd(t *testing.T, sets []setCase, colors []colorCase) (enabled [][]bool, colorIdx []int) {
	t.Helper()
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping JS parity")
	}
	if _, err := os.Stat(filepath.Join("original", "src", "index.js")); err != nil {
		t.Fatalf("original debug not found: %v", err)
	}
	// The driver requires debug's own dependency (ms). node_modules is not
	// committed, so a checkout without `npm ci --omit=dev` in original/ skips —
	// a skip is never a pass. CI installs it from the lockfile.
	if _, err := os.Stat(filepath.Join("original", "node_modules", "ms")); err != nil {
		t.Skip("original/node_modules/ms missing: run `npm ci --omit=dev` in original/ to compare with real debug")
	}
	driver := `const dx=require(process.env.DBGO);
const data=JSON.parse(process.argv[1]);
const enabled=[];
for (const set of data.sets) {
  dx.enable(set.ns);
  const row=[];
  for (const name of set.names) row.push(!!dx.enabled(name));
  enabled.push(row);
}
const colorIdx=[];
for (const c of data.colors) {
  let hash=0;
  for (let i=0;i<c.ns.length;i++){ hash=((hash<<5)-hash)+c.ns.charCodeAt(i); hash|=0; }
  colorIdx.push(Math.abs(hash)%c.n);
}
process.stdout.write(JSON.stringify({enabled,colorIdx}));`
	payload, _ := json.Marshal(map[string]any{"sets": sets, "colors": colors})
	origAbs, _ := filepath.Abs("original")
	argv := []string{"-e", driver, string(payload)}
	cmd := exec.Command(nodeBin, argv...)
	cmd.Env = append(os.Environ(), "DBGO="+origAbs)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node driver failed: %v\n%s", err, out)
	}
	var res struct {
		Enabled [][]bool `json:"enabled"`
		Color   []int    `json:"colorIdx"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal node output: %v (%s)", err, out)
	}
	return res.Enabled, res.Color
}

func TestParity(t *testing.T) {
	var sets []setCase
	for _, ns := range debugSets {
		sets = append(sets, setCase{NS: ns, Names: probeNames})
	}
	jsEnabled, jsColor := jscmd(t, sets, colorCases)

	if len(jsEnabled) != len(sets) {
		t.Fatalf("js enabled rows %d != sets %d", len(jsEnabled), len(sets))
	}
	for i, set := range sets {
		m := NewMatcher()
		m.Enable(set.NS)
		for j, name := range set.Names {
			got := m.Enabled(name)
			want := jsEnabled[i][j]
			if got != want {
				t.Errorf("DEBUG=%q name=%q go=%v js=%v", set.NS, name, got, want)
			}
		}
	}
	t.Logf("enable/enabled parity OK: %d sets x %d names", len(sets), len(probeNames))

	for i, c := range colorCases {
		got := SelectColor(c.NS, c.NColors)
		if got != jsColor[i] {
			t.Errorf("SelectColor(%q,%d) go=%d js=%d", c.NS, c.NColors, got, jsColor[i])
		}
	}
	t.Logf("selectColor parity OK: %d cases", len(colorCases))
}

func TestEnabledWildcardTail(t *testing.T) {
	m := NewMatcher()
	m.Enable("foo")
	if !m.Enabled("anything*") {
		t.Error("a name ending in * should always be enabled")
	}
	if m.Enabled("foo") == false {
		t.Error("foo should be enabled")
	}
	if m.Enabled("bar") {
		t.Error("bar should not be enabled")
	}
}

func TestDisableRoundTrip(t *testing.T) {
	m := NewMatcher()
	m.Enable("foo,bar*,-baz")
	got := m.Disable()
	if m.Enabled("foo") {
		t.Error("after Disable, foo must be off")
	}
	if got == "" {
		t.Fatal("Disable should return the prior namespaces")
	}
	if !m.Enabled("foo") && len(m.names) != 0 && len(m.skips) != 0 {
		t.Errorf("matcher unexpectedly still has rules after Disable: %q", got)
	}
	if m.Enabled("bar") {
		t.Error("bar must be off after Disable")
	}
}
