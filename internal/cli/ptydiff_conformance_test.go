//go:build conformance && linux

package cli_test

import (
	"path/filepath"
	"testing"
)

// The two documents every case below is driven with. Both are **deterministic
// surfaces** — spec 012 delta §6.2.2. `render`'s progress panel is deliberately
// not here: upstream and the port take different times, and a four-digit
// `3240 ms` wraps where `0 ms` does not, so the panel differs in *line count*
// and normalizing `\d+ ms` cannot repair it once the token has wrapped.
const (
	invalidCV = "cv:\n  name: John Doe\nlocale:\n  language: klingon\n"
	// An empty document reaches the `Error` panel — a RenderCVUserError
	// rather than a validation table, and the second shape the styling has
	// to get right (`err_empty_yaml`).
	emptyCV = "\n"
)

// TestPTYHarnessAgreesWhenColourIsOff is the **control case**, delta §6.1, and
// it runs before anything else in this file is believed.
//
// A colour differential produces enormous diffs by construction, so "the port
// emits no escapes" and "my capture is lying to me" look identical from
// outside. Four measurement runs have already been invalidated on this surface,
// two by a capture-path substitution and two by Rich buffering `Live` when
// stdout is not a tty. The protocol that avoids a fifth is to run a
// configuration in which both sides *must* agree and require byte-identity
// before measuring anything else.
//
// `TTY_COMPATIBLE=0` is the right switch: it turns *upstream* into the thing
// the port already is — not a terminal (`rich/console.py:937-984`) — so any
// residue is a real difference and not the finding under test.
func TestPTYHarnessAgreesWhenColourIsOff(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		input string
	}{
		{name: "validation errors", args: []string{"render", "CV.yaml"}, input: invalidCV},
		{name: "user error panel", args: []string{"render", "CV.yaml"}, input: emptyCV},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{"TTY_COMPATIBLE": "0"}
			dir := filepath.Join(t.TempDir(), "w")

			want := runSide(t, dir, upstreamArgv(t, test.args), env, test.input)
			got := runSide(t, dir, portArgv(t, test.args), env, test.input)

			if len(sequences(want)) != 0 {
				t.Fatalf("control is not a control: upstream emitted %d escape sequences with"+
					" TTY_COMPATIBLE=0", len(sequences(want)))
			}
			if got != want {
				t.Error("the harness cannot be trusted: the two sides differ with colour off")
				diffLines(t, got, want)
			}
		})
	}
}

// TestPTYHarnessSeesUpstreamColour is the second half of the control: the
// harness must be able to *observe* colour, or a later "the port emits none"
// would be a statement about the harness rather than about the port.
func TestPTYHarnessSeesUpstreamColour(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "w")
	capture := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), nil, invalidCV)

	counts := inventory(capture)
	for _, want := range []string{"ESC[1;31m", "ESC[36m", "ESC[35m", "ESC[38;5;94m", "ESC[0m"} {
		if counts[want] == 0 {
			t.Errorf("upstream emitted no %s on a pty; the harness is not seeing colour", want)
		}
	}
}

// TestTerminalColourDifferential is the finding, delta §1: on a terminal the
// port emits **no escape sequence of any kind** where upstream emits many.
//
// **This test is red until units C–F attach the styles**, and that is its
// purpose — it is the gate those units turn green. It asserts the two things
// that must both hold and that are independent of the repaint
// non-determinism of delta §5:
//
//  1. the *plain* text agrees, escapes removed — the geometry the 42 goldens
//     already pin must not move when styling lands;
//  2. the escape inventory agrees — the multiset of sequences and how often
//     each occurs.
func TestTerminalColourDifferential(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		input string
		env   map[string]string
		// wrapsOnTheBinaryName marks a surface whose *plain* text cannot be
		// compared after rebinding, because the port laid it out with the
		// longer name before the rebinding shortened it.
		//
		// `--help` is the one: the description "Details: rendercv-go render
		// --help" is three columns wider than upstream's while it is being
		// wrapped, so the break lands in a different place and no amount of
		// re-padding recovers it (D-009, and D-010 for the re-padding rule it
		// exceeds). The escape inventory is still compared, which is the whole
		// point of the case.
		wrapsOnTheBinaryName bool
	}{
		{name: "validation errors", args: []string{"render", "CV.yaml"}, input: invalidCV},
		{name: "user error panel", args: []string{"render", "CV.yaml"}, input: emptyCV},
		{name: "new", args: []string{"new", "JohnDoe"}},
		{name: "help", args: []string{"render", "--help"}, wrapsOnTheBinaryName: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := normalize(runSide(t, dir, upstreamArgv(t, test.args), test.env, test.input))
			got := normalize(runSide(t, dir, portArgv(t, test.args), test.env, test.input))

			// The final frame, not the whole capture: upstream repaints a Live
			// panel once per step and the port paints once, which is the §5
			// decision and not a geometry defect. What must agree either way is
			// what the reader is left looking at.
			if !test.wrapsOnTheBinaryName && portText(got) != frameText(want) {
				t.Error("the final frame's plain text differs, which is a geometry defect" +
					" and not a colour one")
				diffLines(t, portText(got), frameText(want))
			}
			assertInventory(t, got, want)
		})
	}
}

// TestTerminalDetectionDifferential drives the environment matrix of delta §3.
// Each row is a rule the port has to match — **the rule, not merely "emit
// colour"** — and every expectation was measured through the vendored Python.
//
// It is red for the same reason as the test above, and the two are separate
// because they fail for different reasons: this one fails when the *detection*
// is wrong even after the styles are attached.
func TestTerminalDetectionDifferential(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		// colourless says upstream emits no SGR colour at all in this
		// configuration, so the port matches it by emitting none either — the
		// rows that are already green.
		colourless bool
	}{
		{name: "default tty"},
		{name: "NO_COLOR set", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR empty", env: map[string]string{"NO_COLOR": ""}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}, colourless: true},
		{name: "TERM=dumb with FORCE_COLOR", env: map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, colourless: true},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}, colourless: true},
		{name: "TTY_COMPATIBLE=1", env: map[string]string{"TTY_COMPATIBLE": "1"}},
		{name: "FORCE_COLOR empty", env: map[string]string{"FORCE_COLOR": ""}, colourless: true},
		{name: "CI is not consulted", env: map[string]string{"CI": "true"}},
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		// Delta §3.4, and the one row here that is a **wrong** answer today
		// rather than a missing one: a dumb terminal lays out to 80 and ignores
		// COLUMNS (`rich/console.py:1011-1045`), where the port honours COLUMNS
		// unconditionally. Colourless on both sides, so it fails on geometry
		// alone — which is unit G's gate.
		{
			name: "dumb terminal ignores COLUMNS",
			env:  map[string]string{"TERM": "dumb", "COLUMNS": "100"}, colourless: true,
		},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)
			got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)

			if test.colourless && len(sequences(want)) != 0 {
				t.Errorf("row claims upstream emits nothing, but it emitted %d sequences",
					len(sequences(want)))
			}
			if portText(got) != frameText(want) {
				t.Error("the final frame's plain text differs before any style is considered")
				diffLines(t, portText(got), frameText(want))
			}
			assertInventory(t, got, want)
		})
	}
}

// assertInventory compares the multiset of escape sequences, which is what
// survives the repaint non-determinism of delta §5.
func assertInventory(t *testing.T, got, want string) {
	t.Helper()
	gotCounts, wantCounts := inventory(got), inventory(want)

	for sequence, wantCount := range wantCounts {
		if gotCounts[sequence] != wantCount {
			t.Errorf("%s: port emitted %d, upstream %d", sequence, gotCounts[sequence], wantCount)
		}
	}
	for sequence, gotCount := range gotCounts {
		if _, ok := wantCounts[sequence]; !ok {
			t.Errorf("%s: port emitted %d, upstream none", sequence, gotCount)
		}
	}
}
