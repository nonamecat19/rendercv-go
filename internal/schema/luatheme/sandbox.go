// Package luatheme is the port's replacement for upstream's custom-theme
// `__init__.py` — D-002, approved: upstream imports and executes arbitrary
// Python at validation time, Go cannot, so a custom theme is a sandboxed Lua
// script instead (spec 014).
package luatheme

import (
	"context"
	"errors"
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Budget is how long a theme script may run.
//
// **Without it, `while true do end` in a downloaded theme hangs the render
// forever** — measured by a verifier: `timeout 10 rendercv-go render` exited
// 124. A declaration needs microseconds, so the budget is generous by four
// orders of magnitude and still bounds the damage.
const Budget = 2 * time.Second

// maxMemoryMB bounds a script's process-wide memory footprint.
//
// **`Budget` and `maxDepth` bound *time* and *structure*; neither bounds
// *allocation*.** `string.rep("x", 3000000000)` is a single instruction —
// gopher-lua's context check between instructions never gets a chance to
// fire — and Go's runtime kills the whole process with an unrecoverable
// `fatal error: out of memory` and a multi-kilobyte stack trace on stderr,
// exit 2: exactly the failure `maxDepth`'s own comment names as the reason
// this sandbox exists ("unrecoverable, exit 2, from a file the user may
// have downloaded"). `SetMx` is generous — any real theme declaration is a
// few hundred bytes of strings and numbers — while still catching a
// multi-gigabyte allocation attempt within its 100ms poll interval, well
// short of `fatal error`'s ulimit-scale threshold. Found by a fresh-context
// verifier (iteration 14's Lua-slice sweep).
const maxMemoryMB = 512

// maxDepth bounds table nesting.
//
// **A cyclic table used to kill the process**: `local t={} t.self=t return t`
// sent the copy into unbounded recursion and Go's stack overflow is a `fatal
// error`, not a panic — unrecoverable, exit 2, from a file the user may have
// downloaded. The design tree is four levels deep at its deepest, so this is
// generous too.
const maxDepth = 32

// ErrTooDeep is a table nested past maxDepth, which is what a cycle looks like
// from inside the copy.
var ErrTooDeep = errors.New("this custom theme's options are nested too deeply (or contain a cycle)")

// **There is deliberately no `ErrSandboxed`.** An earlier version exported one,
// documented as "what a script gets for reaching outside the sandbox", and no
// code path ever returned it — `errors.Is` against it could never match. A
// blocked global is `nil`, so a script touching it fails with Lua's own
// "attempt to index a non-table object", which names the line. Inventing a
// wrapper would mean inventing user-facing wording, which spec 014 §2
// behavior 9 addresses.

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
//
// **`print` is on the list because CLI stdout is parity axis 2**: a script
// printing to the process's real stdout prepends a line to `render`'s result
// panel. Measured by a verifier.
var blocked = []string{
	"io", "os", "package", "require", "dofile", "loadfile", "debug",
	"print", "_printregs", "collectgarbage", "newproxy", "module", "channel",
	"load", "loadstring", "setfenv", "getfenv",
}

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
	// **`SetMx` polls the process's total allocation, not just this
	// state's** — it is gopher-lua's own mechanism, which calls `os.Exit`
	// directly rather than returning an error `Run` could turn into a
	// panel. That is still strictly better than the crash it replaces:
	// a bounded, deliberate exit instead of an unrecoverable Go `fatal
	// error` with a stack trace on stderr.
	state.SetMx(maxMemoryMB)
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

	// The budget is enforced by the state's context, which gopher-lua checks
	// between instructions — so a script that never yields is still stopped.
	ctx, cancel := context.WithTimeout(context.Background(), Budget)
	defer cancel()
	state.SetContext(ctx)

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
	copied, err := copyTable(table, 0)
	if err != nil {
		return nil, err
	}
	return copied, nil
}

func copyTable(source *lua.LTable, depth int) (*lua.LTable, error) {
	if depth > maxDepth {
		return nil, ErrTooDeep
	}

	out := &lua.LTable{}
	var failure error
	source.ForEach(func(key, value lua.LValue) {
		if failure != nil {
			return
		}
		nested, ok := value.(*lua.LTable)
		if !ok {
			out.RawSet(key, value)
			return
		}
		copied, err := copyTable(nested, depth+1)
		if err != nil {
			failure = err
			return
		}
		out.RawSet(key, copied)
	})
	return out, failure
}
