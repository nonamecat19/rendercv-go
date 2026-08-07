package luatheme_test

import (
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
	for _, script := range []struct{ name, body string }{
		{"io", `local f = io.open("/etc/passwd") return {}`},
		{"os execute", `os.execute("true") return {}`},
		{"os getenv", `local v = os.getenv("HOME") return {}`},
		{"require", `require("os") return {}`},
		{"dofile", `dofile("/etc/passwd") return {}`},
		{"loadfile", `loadfile("/etc/passwd") return {}`},
		{"package", `local p = package.path return {}`},
		{"debug", `debug.getinfo(1) return {}`},
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
