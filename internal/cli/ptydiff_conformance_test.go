//go:build conformance && linux

package cli_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/conformance"
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
		// The `user error panel`, `new` and `validation table` rows are **gone,
		// not disabled**: those surfaces are styled now and are compared line by
		// line by `TestErrorPanelColour`, `TestNewCommandColour` and
		// `TestValidationTableColour` below. `--help` is the last one left, and
		// it is unit F.
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

// TestErrorPanelColour is delta §8's unit D on its **deterministic half**: the
// one-message `Error` box, compared byte for byte against upstream on a pty.
//
// **Bytes, not an inventory.** This surface is the one place on the styled
// terminal where a byte comparison is honest: the panel is printed by
// `rich.print` from the `@handle_user_errors` decorator, never through `Live`,
// so there is no repaint, no cursor hide/show and no timing in it — measured, a
// capture of it carries nothing but the panel. Every other styled surface has
// to fall back to the style inventory (§5), which cannot see a run opened
// around the wrong characters. Here a misplaced run fails the case.
//
// The environment rows are §3's rules applied to this surface: the palette-free
// `bold red` is the same sequence on every colour system, so what they actually
// pin is *whether* a sequence is emitted at all and whether `NO_COLOR` keeps the
// bold.
func TestErrorPanelColour(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "default tty"},
		{name: "NO_COLOR keeps bold", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR set but empty is ignored", env: map[string]string{"NO_COLOR": ""}},
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), test.env, emptyCV)
			got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), test.env, emptyCV)

			if got != want {
				t.Error("the styled Error panel differs from upstream's")
				diffLines(t, got, want)
			}
		})
	}
}

// TestValidationTableColour is delta §8's unit D on its **other half**: the
// `There are validation errors!` box and the three-column table inside it,
// compared against upstream on a pty.
//
// **The final frame, because this panel is a `Live` update** — upstream clears
// the progress panel and repaints this one in its place (`progress_panel.py:160`)
// — but everything inside that frame is compared as bytes. There is no timing
// in it and no repaint that survives the collapse, so the comparison is exact:
// a run opened around the wrong characters fails the case.
//
// What the rows pin, beyond the panel's own `bold red`:
//
//   - the three column styles, `cyan`, `magenta` and `orange4`
//     (`progress_panel.py:149-151`), and that the header carries **none** of
//     them — it is `bold` alone, which is Rich's `table.header`;
//   - `orange4` is a palette entry (94), so the eight-colour row is the one that
//     catches a port hard-coding `ESC[38;5;94m` where upstream downgrades to
//     `ESC[33m` (delta §2.2);
//   - the box characters of the table are unstyled while the panel's border is
//     not, which a misplaced run would break on every line.
func TestValidationTableColour(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "default tty"},
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
		{name: "NO_COLOR keeps bold", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR set but empty is ignored", env: map[string]string{"NO_COLOR": ""}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)
			got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), test.env, invalidCV)

			// **Non-vacuity.** `orange4`'s palette form is the sequence only this
			// surface emits, so seeing it is what says the capture is of the
			// table and not of an empty run.
			if len(test.env) == 0 && !strings.Contains(want, "\x1b[38;5;94m") {
				t.Fatal("upstream emitted no orange4 on a pty; the harness is not seeing the" +
					" surface this case is about")
			}

			if styledFrame(got) != styledFrame(want) {
				t.Error("the styled validation table differs from upstream's")
				diffLines(t, styledFrame(got), styledFrame(want))
			}
		})
	}
}

// TestProgressPanelColour is delta §8's unit C: the box `render` leaves the
// reader looking at, compared against upstream's on a pty.
//
// **The final frame, and only the final frame.** This surface *is* the `Live`
// display of §5 — upstream hides the cursor, repaints the whole panel once per
// completed step at up to 4 Hz, and shows the cursor again — and the number of
// repaints varies between two runs of the same binary. So the comparison
// collapses the repaint stream to the frame that survives it and drops the
// cursor sequences, which are the §5 decision rather than anything unit C
// attaches. What is compared inside that frame is bytes: every run, around
// exactly the characters upstream puts it around.
//
// **The document is rendered to Typst and no further** (`-nopdf -nopng -nomd
// -nohtml`). One step is enough to carry all four styles, and it keeps both
// sides off the PDF path, where the port's compile time is an order of magnitude
// from upstream's and a slow enough step would push the timing field past the
// eight columns `normalizeTimings` can absorb.
func TestProgressPanelColour(t *testing.T) {
	// A document small enough to render in single-digit milliseconds on both
	// sides, so the timing field never outgrows its column.
	const validCV = "cv:\n  name: John Doe\n"

	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "default tty"},
		// `purple` is the panel's one palette colour, and this is the row that
		// catches a port which hard-codes `ESC[38;5;129m`: on eight colours
		// upstream writes `ESC[35m` (delta §2.2).
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
		{name: "NO_COLOR keeps bold", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR set but empty is ignored", env: map[string]string{"NO_COLOR": ""}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}},
	}

	args := []string{"render", "CV.yaml", "-nopdf", "-nopng", "-nomd", "-nohtml"}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, args), test.env, validCV)
			got := runSide(t, dir, portArgv(t, args), test.env, validCV)

			if styledFrame(got) != styledFrame(want) {
				t.Error("the styled progress panel differs from upstream's")
				diffLines(t, styledFrame(got), styledFrame(want))
			}
		})
	}
}

// TestNewCommandColour is delta §8's unit E on `new`: the greeting, the
// `Useful Links` panel with its **OSC 8 hyperlinks**, and the `Get started`
// panel, compared line by line against upstream on a pty.
//
// **Lines, not one string, because exactly one line may differ** — the
// `2. Run: rendercv-go render …` instruction, which is the sanctioned binary
// name (D-009). It is compared as its plain text after `RebindBinaryName` and
// as its **run structure**, so a port that dropped the `cyan` from it would
// still fail. Every other line is compared as bytes, escape sequences included.
//
// `new` never builds a `Live`, so there is no repaint to collapse and no
// timing to normalize: the only thing normalized is the OSC 8 id, which is
// `randint(0, 999999)` upstream and cannot be compared at all (delta §2.5).
func TestNewCommandColour(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "default tty"},
		// `dodger_blue3` and `purple` are palette entries and this is the row
		// that catches a port hard-coding `ESC[38;5;26m`: on eight colours
		// upstream writes `ESC[94m` (delta §2.2).
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
		// `Style.without_color` keeps the link (`rich/style.py:474-490`), so
		// this row pins that `NO_COLOR=1` drops the colour and leaves both the
		// bold and the hyperlink standing.
		{name: "NO_COLOR keeps bold and the links", env: map[string]string{"NO_COLOR": "1"}},
		{name: "NO_COLOR set but empty is ignored", env: map[string]string{"NO_COLOR": ""}},
		// No colour system means no OSC 8 either: `Style.render` returns before
		// the hyperlink branch (`rich/style.py:706`).
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, []string{"new", "JohnDoe"}), test.env, "")
			got := runSide(t, dir, portArgv(t, []string{"new", "JohnDoe"}), test.env, "")

			// **Non-vacuity.** Two empty captures compare equal, so the row
			// that is supposed to carry the whole surface has to be seen
			// carrying it before any agreement means anything.
			if len(test.env) == 0 && !strings.Contains(want, "\x1b]8;") {
				t.Fatal("upstream emitted no OSC 8 hyperlink on a pty; the harness is not" +
					" seeing the surface this case is about")
			}

			assertStyledLines(t, got, want, func(gotLine, wantLine string) bool {
				// D-009: the port names the binary it is. The row is re-padded
				// by the established helper rather than by substituting the
				// captured bytes, which is the phantom of delta §6.3.
				return conformance.RebindBinaryName(plain(gotLine)) == plain(wantLine)
			})
		})
	}
}

// TestCreateThemeColour is unit E's other half: the `Theme created` panel,
// whose markup is the awkward one — three tags upstream never closes, so the
// `purple` opened on the `1. Modify` line runs over five more and the two
// `[cyan]` tags override its colour without ending it (delta §2.1).
//
// **One line cannot match and must not**: D-008 writes `init.lua` where
// upstream writes `__init__.py`, because the port cannot execute the Python
// upstream's `__init__.py` is (D-002). That line is compared as its run
// structure — three separately opened `purple` runs either side — and the rest
// of the panel as bytes.
func TestCreateThemeColour(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "default tty"},
		{name: "eight colour TERM", env: map[string]string{"TERM": "xterm", "COLORTERM": ""}},
		{name: "256 colour TERM", env: map[string]string{"TERM": "xterm-256color", "COLORTERM": ""}},
		{name: "NO_COLOR keeps bold", env: map[string]string{"NO_COLOR": "1"}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "TTY_COMPATIBLE=0", env: map[string]string{"TTY_COMPATIBLE": "0"}},
	}

	args := []string{"create-theme", "mytheme"}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "w")
			want := runSide(t, dir, upstreamArgv(t, args), test.env, "")
			got := runSide(t, dir, portArgv(t, args), test.env, "")

			if len(test.env) == 0 && !strings.Contains(want, "\x1b[38;5;129m") {
				t.Fatal("upstream emitted no purple on a pty; the harness is not seeing the" +
					" surface this case is about")
			}

			assertStyledLines(t, got, want, func(gotLine, wantLine string) bool {
				return strings.Contains(gotLine, "/init.lua") &&
					strings.Contains(wantLine, "/__init__.py")
			})
		})
	}
}

// assertStyledLines compares two captures line by line with their escape
// sequences kept.
//
// A line the two sides are **allowed** to differ on — a sanctioned divergence,
// and nothing else — still has to carry the same runs in the same order, which
// is what `sequences` compares. That is the part of a styled line a divergence
// in its *text* cannot excuse: it catches a run opened around the wrong
// characters, or dropped altogether, on exactly the line whose bytes cannot be
// compared.
func assertStyledLines(t *testing.T, got, want string, sanctioned func(gotLine, wantLine string) bool) {
	t.Helper()

	gotLines, wantLines := styledLines(got), styledLines(want)
	if len(gotLines) != len(wantLines) {
		t.Fatalf("the port wrote %d lines, upstream %d:\n  port     %q\n  upstream %q",
			len(gotLines), len(wantLines), readable(got), readable(want))
	}

	for i := range wantLines {
		if gotLines[i] == wantLines[i] {
			continue
		}
		if !sanctioned(gotLines[i], wantLines[i]) {
			t.Errorf("line %d differs:\n  port     %q\n  upstream %q",
				i+1, readable(gotLines[i]), readable(wantLines[i]))
			continue
		}
		if !slices.Equal(sequences(gotLines[i]), sequences(wantLines[i])) {
			t.Errorf("line %d is a sanctioned divergence but its runs differ:"+
				"\n  port     %q\n  upstream %q",
				i+1, readable(gotLines[i]), readable(wantLines[i]))
		}
	}
}

// styledLines is a capture as lines, escape sequences kept and the OSC 8 id
// normalized. The `\r` is the pty's own ONLCR translation and is neither
// side's doing.
func styledLines(capture string) []string {
	text := strings.ReplaceAll(normalize(capture), "\r\n", "\n")
	return strings.Split(strings.TrimRight(text, "\r\n"), "\n")
}

// styledFrame is the final frame with its escape sequences **kept** — the
// counterpart of `frameText`, for the one comparison that is about them.
//
// Three normalizations, each a thing neither side controls:
//
//   - the cursor hide/show `Live` brackets its run with (delta §5, deliberately
//     not reproduced by the port);
//   - the pty's own ONLCR `\r`, and the extra line `Live.stop` writes on a
//     terminal;
//   - the timing, which is different on the two sides by construction.
func styledFrame(capture string) string {
	frame := cursorVisibility.ReplaceAllString(finalFrame(capture), "")
	frame = strings.ReplaceAll(frame, "\r\n", "\n")
	return normalizeTimings(strings.TrimRight(frame, "\r\n"))
}

// cursorVisibility matches `Live`'s `ESC[?25l` / `ESC[?25h`.
var cursorVisibility = regexp.MustCompile(`\x1b\[\?25[lh]`)

// timingField matches the duration and whatever padding follows it.
//
// **The replacement is the same width as what it replaced**, which is what lets
// two sides that took different times be compared column for column: the field
// is laid out to eight columns and `<timing>` is eight characters, so a match of
// `n` columns becomes `<timing>` plus `n-8` spaces. That holds as long as the
// duration itself fits the field — a step slow enough to print nine digits of
// milliseconds would shift the row, which is delta §6.2.2's warning and the
// reason this case renders Typst only.
var timingField = regexp.MustCompile(`\d+ ms +`)

func normalizeTimings(frame string) string {
	return timingField.ReplaceAllStringFunc(frame, func(field string) string {
		const token = "<timing>"
		if len(field) < len(token) {
			return field
		}
		return token + strings.Repeat(" ", len(field)-len(token))
	})
}

// TestTerminalDetection drives the environment matrix of delta §3. Each row is a
// rule the port has to match — **the rule, not merely "emit colour"** — and
// every expectation was measured through the vendored Python.
//
// **Every row is asserted for real now.** The surface it drives is the
// validation panel, which unit D styled, so the rows where upstream emits
// colour are compared as the final frame's bytes rather than inverted — a port
// that read `NO_COLOR` as "no styling", or that took `CI` for a colour hint,
// fails here on the exact sequence it got wrong. The colourless rows keep their
// own shape: they assert that upstream emits nothing and that the port matches.
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

		// Upstream emits colour, and so must the port: unit B's detection is
		// what has to get each of these right, and unit D is what attached the
		// styles this surface carries.
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
			if len(sequences(want)) == 0 {
				t.Fatalf("row claims upstream emits colour here and it emitted none;" +
					" the harness has stopped seeing a terminal")
			}
			if styledFrame(got) != styledFrame(want) {
				t.Error("the styled frame differs from upstream's")
				diffLines(t, styledFrame(got), styledFrame(want))
			}
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

// TestDumbTerminalWidthIsPending is delta §3.4, and the one part of this
// surface that is a **wrong** answer today rather than a missing one: a dumb
// terminal lays out to 80 and ignores `COLUMNS` (`rich/console.py:1011-1045`),
// where the port honours `COLUMNS` unconditionally
// (`internal/cli/panel.go:24-31`).
//
// Inverted like the rest, and it is unit G's gate. Both sides are colourless
// here, so it fails on geometry alone — which is why it is its own test rather
// than a row in the matrix above.
func TestDumbTerminalWidthIsPending(t *testing.T) {
	env := map[string]string{"TERM": "dumb", "COLUMNS": "100"}
	dir := filepath.Join(t.TempDir(), "w")

	want := runSide(t, dir, upstreamArgv(t, []string{"render", "CV.yaml"}), env, invalidCV)
	got := runSide(t, dir, portArgv(t, []string{"render", "CV.yaml"}), env, invalidCV)

	upstreamWidth, portWidth := firstLineWidth(frameText(want)), firstLineWidth(portText(got))
	if upstreamWidth != 80 {
		t.Fatalf("upstream laid out to %d columns under TERM=dumb COLUMNS=100, want 80;"+
			" the rule this test records has changed", upstreamWidth)
	}
	if portWidth == upstreamWidth {
		t.Errorf("the port laid out to %d columns, matching upstream. If unit G has landed,"+
			" replace this inversion with a plain-text comparison", portWidth)
	} else if portWidth != 100 {
		t.Errorf("the port laid out to %d columns, want the 100 it takes from COLUMNS;"+
			" the divergence recorded here is not the one being measured", portWidth)
	}
}
