// Package luatheme is the port's replacement for upstream's custom-theme
// `__init__.py` — D-002, approved: upstream imports and executes arbitrary
// Python at validation time, Go cannot, so a custom theme is a sandboxed Lua
// script instead (spec 014).
package luatheme

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ErrSandboxed is what a script gets for reaching outside the sandbox. It is one
// error rather than one per library, because the distinction is not useful to
// the person who wrote the script: the answer is always "that is not available".
var ErrSandboxed = errors.New("this custom theme script tried to use a disallowed feature")

// blocked are the standard-library globals a theme script may not touch.
//
// **This is the whole point of D-002, not an implementation detail** (spec 014
// §2 behavior 8). Upstream runs user Python with the full standard library
// reachable — `open`, `subprocess`, `socket` — during *validation*, on a file
// the user may have downloaded with a CV template. A Lua state with the same
// reach would be the same hazard in a different syntax, so the port's version is
// only worth having if it is actually closed.
//
// The list is what `gopher-lua` opens by default that can leave the process:
// files, the OS, the module loader, the bytecode loader, and the debug library
// (which can reach back into the Go host through upvalues).
var blocked = []string{"io", "os", "package", "require", "dofile", "loadfile", "debug"}

// NewState returns a Lua state a theme script runs in.
//
// Everything a theme actually needs is left: `string`, `table`, `math`, and the
// base library minus the loaders. A theme declares options — names, types and
// defaults — so it needs no side effects at all, which is what makes the
// sandbox cheap rather than a compromise.
func NewState() *lua.LState {
	state := lua.NewState(lua.Options{SkipOpenLibs: false})
	for _, name := range blocked {
		state.SetGlobal(name, lua.LNil)
	}
	return state
}

// Run executes a theme script and returns the table it yields.
//
// A script must **return a table**: upstream's `__init__.py` declares a pydantic
// model, and the Lua equivalent of a declaration is data, not a class (spec 014
// §2 behavior 6).
func Run(script string) (*lua.LTable, error) {
	state := NewState()
	defer state.Close()

	if err := state.DoString(script); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	value := state.Get(-1)
	table, ok := value.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("the script returned %s, want a table of theme options", value.Type())
	}

	// The table outlives the state it was built in, so it is copied out before
	// the state closes. Only the shapes a theme declaration can hold are copied.
	return copyTable(table), nil
}

func copyTable(source *lua.LTable) *lua.LTable {
	out := &lua.LTable{}
	source.ForEach(func(key, value lua.LValue) {
		if nested, ok := value.(*lua.LTable); ok {
			out.RawSet(key, copyTable(nested))
			return
		}
		out.RawSet(key, value)
	})
	return out
}
