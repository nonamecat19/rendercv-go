//go:build conformance && linux

package cli_test

import (
	"path/filepath"
	"testing"
)

// The documents the cases below are driven with. Both are **deterministic
// surfaces** — spec 012 delta §6.2.2. `render`'s progress panel is deliberately
// not here: upstream and the port take different times, and a four-digit
// `3240 ms` wraps where `0 ms` does not, so the panel differs in *line count*
// and normalizing `\d+ ms` cannot repair it once the token has wrapped.
const (
	invalidCV = "cv:\n  name: John Doe\nlocale:\n  language: klingon\n"
	// An empty document reaches the `Error` panel — a RenderCVUserError rather
	// than a validation table, and the second shape the styling has to get
	// right (`err_empty_yaml`).
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

// assertStylingIsStillPending is the **inverted assertion** this file is built
// on, and it is the reason the file can land while units C–G are outstanding.
//
// It is `conformance.AssertUnreachable`'s shape (`internal/conformance/unreachable.go`)
// applied to unfinished work rather than to an approved divergence: it does not
// skip, it *requires* the port to emit nothing and fails the moment it starts
// emitting something. So the entry cannot rot — the unit that attaches the
// styles fails this test on the way in and has to replace the inversion with a
// real inventory comparison.
//
// What is asserted positively, and stays asserted afterwards: upstream really
// does emit colour here, so a harness that stopped seeing it is caught; and the
// plain text agrees, so a styling unit that moves the geometry is caught by the
// same case that gates its colour.
func assertStylingIsStillPending(t *testing.T, unit, got, want string) {
	t.Helper()

	if len(sequences(want)) == 0 {
		t.Fatalf("upstream emitted no escape sequence at all; the case is not measuring"+
			" what it claims, or the harness has stopped seeing a terminal (unit %s)", unit)
	}
	if count := len(sequences(got)); count != 0 {
		t.Errorf("the port emitted %d escape sequences, where this case records that it emits"+
			" none. If unit %s has landed, delete this inversion and assert the inventory"+
			" against upstream instead:\n  port     %v\n  upstream %v",
			count, unit, inventory(got), inventory(want))
	}
}

// TestTerminalStylingIsPending records the finding of spec 012 delta §1 — on a
// terminal the port emits **no escape sequence of any kind** where upstream
// emits many — as an inverted assertion per surface.
//
// Each row names the unit of delta §8 that closes it and must then delete it.
func TestTerminalStylingIsPending(t *testing.T) {
	tests := []struct {
		name  string
		unit  string
		args  []string
		input string
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
		{name: "validation table", unit: "D", args: []string{"render", "CV.yaml"}, input: invalidCV},
		{name: "user error panel", unit: "D", args: []string{"render", "CV.yaml"}, input: emptyCV},
		{name: "new", unit: "E", args: []string{"new", "JohnDoe"}},
		{name: "help", unit: "F", args: []string{"render", "--help"}, wrapsOnTheBinaryName: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := normalize(runSide(t, dir, upstreamArgv(t, test.args), nil, test.input))
			got := normalize(runSide(t, dir, portArgv(t, test.args), nil, test.input))

			// The final frame, not the whole capture: upstream repaints a Live
			// panel once per step and the port paints once, which is the §5
			// decision and not a geometry defect. What must agree either way is
			// what the reader is left looking at.
			if !test.wrapsOnTheBinaryName && portText(got) != frameText(want) {
				t.Error("the final frame's plain text differs, which is a geometry defect" +
					" and not a colour one")
				diffLines(t, portText(got), frameText(want))
			}
			assertStylingIsStillPending(t, test.unit, got, want)
		})
	}
}

// TestTerminalDetection drives the environment matrix of delta §3. Each row is a
// rule the port has to match — **the rule, not merely "emit colour"** — and
// every expectation was measured through the vendored Python.
//
// The rows where upstream emits nothing are **asserted for real and pass
// today**: they are the configurations the port is already right about, and
// they are what stops unit B's detection from being wired up backwards later.
// The rest are inverted until the styles land.
func TestTerminalDetection(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		// colourless says upstream emits no SGR at all in this configuration,
		// so the port matches it by emitting none either, and the inventories
		// are compared directly.
		colourless bool
	}{
		// Upstream emits nothing: asserted, green.
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}, colourless: true},
		{
			name: "TERM=dumb beats FORCE_COLOR",
			env:  map[string]string{"TERM": "dumb", "FORCE_COLOR": "1"}, colourless: true,
		},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}, colourless: true},
		{name: "FORCE_COLOR set but empty", env: map[string]string{"FORCE_COLOR": ""}, colourless: true},

		// Upstream emits colour: inverted until unit C–F attach the styles, and
		// unit B's detection is what has to get each of these right.
		{name: "default tty"},
		{name: "NO_COLOR keeps bold", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR set but empty is ignored", env: map[string]string{"NO_COLOR": ""}},
		{name: "TTY_COMPATIBLE=1", env: map[string]string{"TTY_COMPATIBLE": "1"}},
		{name: "CI is not consulted", env: map[string]string{"CI": "true"}},
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)
			got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)

			if portText(got) != frameText(want) {
				t.Error("the final frame's plain text differs before any style is considered")
				diffLines(t, portText(got), frameText(want))
			}
			if test.colourless {
				if count := len(sequences(want)); count != 0 {
					t.Fatalf("row claims upstream emits nothing, but it emitted %d sequences", count)
				}
				assertInventory(t, got, want)
				return
			}
			assertStylingIsStillPending(t, "C-F", got, want)
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

// TestDumbTerminalWidth is delta §3.4 — the one part of this surface that was a
// **wrong** answer rather than a missing one: a dumb terminal lays out to 80 and
// ignores `COLUMNS` (`rich/console.py:1018-1019`), where the port used to honour
// `COLUMNS` unconditionally.
//
// Unit G closed it, so the inversion this case carried is gone and the two sides
// are compared as text. Both are colourless here, which is what lets a geometry
// rule be gated on a terminal while the styles are still outstanding — and it is
// why this is its own test rather than a row in the matrix above.
//
// The three rows are the rule and its two boundaries: `COLUMNS` ignored while
// dumb, `TTY_COMPATIBLE=0` taking the dumbness off again so `COLUMNS` wins, and
// an ordinary `TERM` where it always did.
func TestDumbTerminalWidth(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		width int
	}{
		{
			name:  "a dumb tty ignores COLUMNS",
			env:   map[string]string{"TERM": "dumb", "COLUMNS": "100"},
			width: 80,
		},
		{
			name:  "TTY_COMPATIBLE=0 makes it not a terminal, so COLUMNS wins",
			env:   map[string]string{"TERM": "dumb", "COLUMNS": "100", "TTY_COMPATIBLE": "0"},
			width: 100,
		},
		{
			name:  "an ordinary tty honours COLUMNS",
			env:   map[string]string{"TERM": "xterm-256color", "COLUMNS": "100"},
			width: 100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)
			got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)

			upstreamText, portFrame := frameText(want), portText(got)
			if upstreamWidth := firstLineWidth(upstreamText); upstreamWidth != test.width {
				t.Fatalf("upstream laid out to %d columns, want %d; the rule this test records"+
					" has changed", upstreamWidth, test.width)
			}
			if portWidth := firstLineWidth(portFrame); portWidth != test.width {
				t.Errorf("the port laid out to %d columns, want %d", portWidth, test.width)
			}
			if portFrame != upstreamText {
				t.Error("the final frame's plain text differs")
				diffLines(t, portFrame, upstreamText)
			}
		})
	}
}
