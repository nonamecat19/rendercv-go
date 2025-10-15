package luatheme_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/luatheme"
)

// Spec 014 §4's fourth criterion: **the sandbox refuses filesystem and process
// access, asserted rather than assumed.**
//
// Upstream executes arbitrary Python during validation, on a file that may have
// arrived with a downloaded CV template. D-002's Lua replacement is only worth
// having if it is actually closed, so each blocked global is named here — a
// table-driven test, so removing one from the list fails loudly instead of
// quietly widening what a theme can do.
func TestSandboxBlocksEscapes(t *testing.T) {
	// **`dofile("/etc/passwd")` errors whether or not `dofile` is
	// blocked** — `/etc/passwd` is not valid Lua, so the test passed for
	// the wrong reason: removing `dofile` from `blocked` failed nothing.
	// A harmless, genuinely valid `.lua` file makes the *block* the only
	// thing that can raise an error. Found by a fresh-context verifier
	// (iteration 14's twentieth re-verification).
	harmless := filepath.Join(t.TempDir(), "harmless.lua")
	if err := os.WriteFile(harmless, []byte("return nil"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, script := range []struct{ name, body string }{
		{"io", `local f = io.open("/etc/passwd") return {}`},
		{"os execute", `os.execute("true") return {}`},
		{"os getenv", `local v = os.getenv("HOME") return {}`},
		{"require", `require("os") return {}`},
		{"dofile", fmt.Sprintf(`dofile(%q) return {}`, harmless)},
		{"loadfile", `loadfile("/etc/passwd") return {}`},
		{"package", `local p = package.path return {}`},
		{"debug", `debug.getinfo(1) return {}`},
		// **The other ten of the seventeen names `blocked` (sandbox.go)
		// lists**, previously verified only by reading the source, not by
		// a test — a fresh-context verifier's own transcript is not what
		// spec 014 §4's fourth criterion means by "asserted rather than
		// assumed": deleting one of these ten from `blocked` failed
		// nothing here before. Found by a fresh-context verifier
		// (iteration 14's nineteenth re-verification).
		{"print", `print("x") return {}`},
		{"_printregs", `_printregs() return {}`},
		{"collectgarbage", `collectgarbage("collect") return {}`},
		{"newproxy", `local p = newproxy() return {}`},
		{"module", `module("x") return {}`},
		{"channel", `local c = channel.make() return {}`},
		// **`load("return 1")` errors whether or not `load` is blocked
		// too** — gopher-lua's `load` is Lua 5.1's, which takes a chunk
		// *reader function*, not a string; unblocked, this call itself
		// raises "bad argument #1". `load(function() ... end)` is the
		// well-formed 5.1 call and would succeed if `load` were reachable.
		// Found by a fresh-context verifier (iteration 14's twentieth
		// re-verification).
		{"load", `local f = load(function() return "return 1" end) return {}`},
		{"loadstring", `local f = loadstring("return 1") return {}`},
		{"setfenv", `setfenv(1, {}) return {}`},
		{"getfenv", `local e = getfenv(1) return {}`},
	} {
		t.Run(script.name, func(t *testing.T) {
			if _, err := luatheme.Run(script.body); err == nil {
				t.Errorf("%s was allowed; the sandbox is open", script.name)
			}
		})
	}
}

// What a theme legitimately needs still works: it declares data, using the
// string, table and math libraries.
func TestSandboxAllowsADeclaration(t *testing.T) {
	table, err := luatheme.Run(`
		local options = {}
		options.name = string.lower("MyTheme")
		options.spacing = math.max(2, 3) .. "cm"
		options.colors = { name = "rgb(0, 0, 0)" }
		return options
	`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := table.RawGetString("name").String(); got != "mytheme" {
		t.Errorf("name = %q", got)
	}
	if got := table.RawGetString("spacing").String(); got != "3cm" {
		t.Errorf("spacing = %q", got)
	}
}

// A script that returns something other than a table is a reportable error, not
// a panic — upstream's equivalent is a module that defines no model.
func TestScriptMustReturnATable(t *testing.T) {
	_, err := luatheme.Run(`return 42`)
	if err == nil || !strings.Contains(err.Error(), "want a table") {
		t.Errorf("err = %v, want a table-shaped complaint", err)
	}
}
