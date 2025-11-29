package conformance

import (
	"os"
	"strconv"
)

// AllowMissingInputEnvVar is the one explicit opt-out that turns an absent test
// input back into a skip.
//
// It is the same convention `internal/clidiff`'s `RENDERCV_DIFF_ALLOW_SKIP`
// established for the differential gate, generalised to the other inputs that
// can go missing: **absent is fatal by default, and only an environment
// variable set on purpose makes it a skip.** One name serves every site, so a
// developer without the submodule sets one thing and a CI job that sets it by
// accident is visible in one grep.
//
// The default is a failure for the reason a skipped parity case is: a test that
// vanishes with its input prints `ok`, and `ok` is what a reader takes for
// evidence. `AssertUnreachable`'s doc comment makes the same argument about
// forbidden cases — this is that argument applied to missing files.
const AllowMissingInputEnvVar = "RENDERCV_ALLOW_MISSING_INPUT"

// TB is the part of `*testing.T` RequireInput uses. It is an interface so the
// guard's own three branches can be tested — a fatal that cannot be observed
// failing is exactly the hole this guard exists to close.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// RequireInput reads a file a test cannot assert anything without.
//
// An unreadable path fails the test, naming what stops running and how to
// produce the file, unless AllowMissingInputEnvVar says otherwise — in which
// case the test skips and says which variable let it.
//
// assertion names what would silently not run; remedy is the command or action
// that produces the input.
func RequireInput(t TB, path, assertion, remedy string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err == nil {
		return content
	}
	if AllowMissingInput(t) {
		t.Skipf("%s is absent and %s is set, so %s does not run (%v)",
			path, AllowMissingInputEnvVar, assertion, err)
		return nil
	}
	t.Fatalf("%s is absent, so %s would silently not run — %s, or set %s=1 to skip it "+
		"on purpose (%v)", path, assertion, remedy, AllowMissingInputEnvVar, err)
	return nil
}

// AllowMissingInput reads AllowMissingInputEnvVar.
//
// **An unparseable value is fatal rather than a silent false.** A typo in the
// opt-out must not read as "run the gate" any more than it reads as "skip it";
// either reading turns a misspelling into a result nobody chose.
func AllowMissingInput(t TB) bool {
	t.Helper()

	raw, ok := os.LookupEnv(AllowMissingInputEnvVar)
	if !ok || raw == "" {
		return false
	}
	allow, err := strconv.ParseBool(raw)
	if err != nil {
		t.Fatalf("%s=%q is not a boolean: %v", AllowMissingInputEnvVar, raw, err)
		return false
	}
	return allow
}
