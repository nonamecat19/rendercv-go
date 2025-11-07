package conformance

import "testing"

// Unreachable records a corpus case that **cannot** pass, with the approved
// divergence that makes it so.
//
// Every entry here is a case whose golden the port is forbidden to reproduce,
// not one it has merely failed to reproduce yet. The distinction is the whole
// point: a case that is red because the work is unfinished must stay red and
// loud (`parity_test.go`'s package comment — "a skipped parity case is how a
// port convinces itself it is done"), while a case that is red because
// `specs/divergences.md` says it must be is a permanent, reviewed fact, and
// leaving it red means CI can never be green and so stops carrying information.
//
// `AssertUnreachable` therefore does **not** skip. It runs the comparison and
// requires it to fail, so an entry whose divergence is later closed — or wrongly
// added — fails the suite with "unexpectedly passes". The list can only shrink
// with a divergence, never quietly.
type Unreachable struct {
	// Case is the corpus case name, as `testdata/corpus.json` spells it.
	Case string
	// Divergence is the approved entry that forbids parity, e.g. "D-010".
	Divergence string
	// Why is one line naming the mechanism, for the failure message a wrongly
	// listed case produces.
	Why string
}

// unreachableCases is the whole list. Eight cases, three divergences.
//
// It is deliberately a Go literal rather than a field in `corpus.json`: the
// corpus is generated from the vendored upstream by `tools/gengolden`, and what
// the *port* cannot reproduce is not a property of upstream's behavior.
var unreachableCases = []Unreachable{
	// D-008 — `create-theme` writes port-native files. Both cases compare
	// template *source*, and the port's loader reads the pongo2 transform, not
	// upstream's Jinja.
	{Case: "create_theme", Divergence: "D-008", Why: "compares template source, which D-005 allows to differ"},
	{Case: "new_typst_templates", Divergence: "D-008", Why: "compares template source, which D-005 allows to differ"},

	// D-010 — the help pages' prose wraps around a longer binary name. The
	// pages are written and verified; `rendercv-go` is three characters longer
	// than `rendercv`, so Rich's wrap points move.
	{Case: "cli_help", Divergence: "D-010", Why: "the binary name is longer, so the prose wraps elsewhere"},
	{Case: "cli_help_short", Divergence: "D-010", Why: "the binary name is longer, so the prose wraps elsewhere"},
	{Case: "cli_render_help", Divergence: "D-010", Why: "the binary name is longer, so the prose wraps elsewhere"},
	{Case: "cli_new_help", Divergence: "D-010", Why: "the binary name is longer, so the prose wraps elsewhere"},

	// D-011 — both goldens are Rich-rendered Python tracebacks carrying the
	// generating machine's absolute paths and CPython frames.
	{Case: "err_missing_file", Divergence: "D-011", Why: "the golden is a Python traceback with absolute paths"},
	{Case: "err_bad_override_key", Divergence: "D-011", Why: "the golden is a Python traceback with absolute paths"},
}

// UnreachableFor returns the divergence forbidding a case's parity, if there is
// one.
func UnreachableFor(name string) (Unreachable, bool) {
	for _, entry := range unreachableCases {
		if entry.Case == name {
			return entry, true
		}
	}
	return Unreachable{}, false
}

// UnreachableCases returns the list, for a test that checks it names only cases
// the corpus actually has.
func UnreachableCases() []Unreachable {
	out := make([]Unreachable, len(unreachableCases))
	copy(out, unreachableCases)
	return out
}

// AssertUnreachable is the comparison a forbidden case gets instead of the
// byte-for-byte one.
//
// It asserts two things, and the second is the one that matters:
//
//  1. **the invocation still ran.** D-011's own note in `specs/divergences.md`
//     says the value of keeping these cases in the suite is that a crash, a
//     wrong stream or a bogus exit code would still be caught. So the port must
//     produce output or files, not nothing — `70`-with-no-output, the failure
//     mode an earlier audit found across six invocations, would fail here.
//  2. **the output still differs.** If a divergence is closed, or was recorded
//     for something that turned out to be reproducible, the case starts matching
//     and this fails with "unexpectedly passes". That is what keeps the list
//     from rotting into a mute button.
func AssertUnreachable(t *testing.T, entry Unreachable, golden Golden, got Result) {
	t.Helper()

	if got.Stdout == "" && got.Stderr == "" && len(got.Files) == 0 {
		t.Errorf("%s is unreachable under %s, but the port produced no output at all"+
			" — that is a crash, not a divergence", entry.Case, entry.Divergence)
		return
	}

	if Normalize(golden.Stdout) != Normalize(got.Stdout) {
		return
	}
	if !sameFileSet(golden.Files, got.Files) {
		return
	}
	t.Errorf("%s now matches upstream, but %s says it cannot (%s).\n"+
		"Either the divergence is closed — remove the entry from unreachableCases and"+
		" update specs/divergences.md — or it was recorded for something reproducible.",
		entry.Case, entry.Divergence, entry.Why)
}

func sameFileSet(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	seen := make(map[string]int, len(want))
	for _, name := range want {
		seen[name]++
	}
	for _, name := range got {
		seen[name]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
