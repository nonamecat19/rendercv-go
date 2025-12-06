package conformance

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

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

	verdict := evaluateUnreachable(golden, got)
	switch verdict.kind {
	case verdictSilent:
		t.Errorf("%s is unreachable under %s, but the port produced no output at all"+
			" — that is a crash, not a divergence", entry.Case, entry.Divergence)
	case verdictUnreadable:
		t.Errorf("%s is unreachable under %s, but the comparison could not be made: %s",
			entry.Case, entry.Divergence, verdict.detail)
	case verdictDiffers:
		t.Logf("%s differs from upstream as %s requires: %s",
			entry.Case, entry.Divergence, verdict.detail)
	case verdictMatches:
		t.Errorf("%s now matches upstream, but %s says it cannot (%s).\n"+
			"Either the divergence is closed — remove the entry from unreachableCases and"+
			" update specs/divergences.md — or it was recorded for something reproducible.",
			entry.Case, entry.Divergence, entry.Why)
	}
}

// verdictKind is what the inverted comparison found.
type verdictKind int

const (
	// verdictSilent: the port produced nothing — a crash, not a divergence.
	verdictSilent verdictKind = iota
	// verdictDiffers: the port differs from upstream, as the divergence requires.
	verdictDiffers
	// verdictMatches: the port reproduced the golden, which the divergence forbids.
	verdictMatches
	// verdictUnreadable: an artifact the comparison needs could not be read.
	verdictUnreadable
)

// unreachableVerdict is a kind plus one line naming the evidence behind it, so a
// case that stays red says *which* difference held it up rather than only that
// one existed.
type unreachableVerdict struct {
	kind   verdictKind
	detail string
}

// evaluateUnreachable performs the inverted comparison. It is separate from
// AssertUnreachable, and pure, because a guard whose verdict cannot be observed
// without failing a test is a guard nothing pins.
//
// **It compares what `parity_test.go` compares, on the same terms.** Two of those
// terms were missing until iteration 13, and together they let `new_typst_templates`
// stay red for a divergence other than its own:
//
//   - the port's name is rebound first, as the normal path does. Without that,
//     D-009's rename counted as evidence of D-008, and closing D-008 would not
//     have failed the suite.
//   - artifact *contents* are compared, not only their names. D-008 is entirely
//     about what is inside the thirteen template files the case writes; a guard
//     reading only the file list could not see the divergence it was guarding.
//
// Exit codes are deliberately **not** evidence: no recorded divergence is about
// one, so a differing exit code is a defect that should surface as a matching
// verdict's loud failure rather than quietly hold a case red.
// It reports **every** dimension that differs rather than the first, because the
// short-circuit is what hid the defect: a case whose only listed evidence is a
// dimension its own divergence is not about is a case borrowing another's reason,
// and that is visible in the log only if the whole list is there.
func evaluateUnreachable(golden Golden, got Result) unreachableVerdict {
	if got.Stdout == "" && got.Stderr == "" && len(got.Files) == 0 {
		return unreachableVerdict{kind: verdictSilent}
	}

	var differences []string
	if Normalize(golden.Stdout) != Normalize(RebindBinaryName(got.Stdout)) {
		differences = append(differences, "stdout")
	}
	if Normalize(golden.Stderr) != Normalize(RebindBinaryName(got.Stderr)) {
		differences = append(differences, "stderr")
	}
	if !sameFileSet(golden.Files, got.Files) {
		differences = append(differences, "the set of files written")
	}

	for _, rel := range golden.Files {
		// A file only one side has is already reported by the set difference;
		// reading it would say "missing", which is the same fact twice.
		if !contentComparable(rel) || !contains(got.Files, rel) {
			continue
		}
		want, err := golden.artifactBytes(rel)
		if err != nil {
			return unreachableVerdict{kind: verdictUnreadable, detail: "golden artifact " + rel + ": " + err.Error()}
		}
		have, err := got.artifactBytes(rel)
		if err != nil {
			return unreachableVerdict{kind: verdictUnreadable, detail: "produced artifact " + rel + ": " + err.Error()}
		}
		if !bytes.Equal(want, have) {
			differences = append(differences, "the contents of "+rel)
		}
	}

	if len(differences) == 0 {
		return unreachableVerdict{kind: verdictMatches}
	}
	return unreachableVerdict{kind: verdictDiffers, detail: strings.Join(differences, "; ")}
}

// contentComparable is `parity_test.go`'s isByteComparable, reachable from the
// package itself: PDF and PNG bytes carry a creation timestamp, a document ID and
// a different Typst build, so a difference in them is never evidence of anything.
func contentComparable(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".pdf", ".png":
		return false
	default:
		return true
	}
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
