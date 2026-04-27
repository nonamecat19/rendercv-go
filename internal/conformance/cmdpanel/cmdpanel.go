// Package cmdpanel canonicalizes the order of the entries in the Rich
// `Commands` panel of an upstream `--help` page.
//
// # Why a canonical order has to be imposed
//
// Upstream registers its CLI commands by walking its own package directory:
//
//	# src/rendercv/cli/app.py:142-151
//	cli_folder_path = pathlib.Path(__file__).parent
//	for file in cli_folder_path.rglob("*_command.py"):
//	    ...
//	    module = importlib.import_module(full_module)
//
// `pathlib.Path.rglob` yields raw `os.scandir` order — it is not sorted, unlike
// `sorted(rglob(...))` — and Typer lists subcommands in *registration* order
// rather than click's sorted one (`typer/core.py:816-820`: "Note that in Click's
// Group class, these are sorted. In Typer, we wish to maintain the original order
// of creation").
//
// The `Commands` panel of `rendercv --help` is therefore in filesystem readdir
// order: a property of the directory the interpreter happens to be reading, not
// of the release. Measured on the pinned submodule (`2eba248`), one interpreter,
// one command: the checkout prints `create-theme, new, render`, and a plain
// `cp -r` copy of the same source prints `render, new, create-theme`. Nothing
// else in the page differs — the column width is set by the longest command name,
// which the reordering does not change.
//
// There is consequently no upstream order to be byte-identical to; it is a coin
// flip per checkout. Both sides of every golden comparison are put into one
// canonical order — command name ascending — so a foreign checkout of the pinned
// upstream regenerates the same bytes. See `specs/divergences.md` D-018.
package cmdpanel

import (
	"sort"
	"strings"
)

// panelHeader opens the one Rich panel whose entries are reordered. `Options` and
// `Arguments` panels are left alone: their order is written in upstream's source,
// so it is contractual and a difference in it is a real defect.
const panelHeader = "╭─ Commands"

// Sort returns s with the entries of every `Commands` panel in ascending command
// name order.
//
// It is deliberately conservative. A panel it cannot parse — no bottom border, a
// body line that is not a panel row, a first row that is a continuation — is left
// exactly as found, because a normalizer that guesses at a shape it does not
// recognize hides more than it fixes.
func Sort(s string) string {
	if !strings.Contains(s, panelHeader) {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], panelHeader) {
			continue
		}
		end, ok := panelEnd(lines, i+1)
		if !ok {
			continue
		}
		sortEntries(lines[i+1 : end])
		i = end
	}
	return strings.Join(lines, "\n")
}

// panelEnd returns the index of the panel's bottom border, given the index of its
// first body line. It reports false for a panel with no body or no border.
func panelEnd(lines []string, start int) (int, bool) {
	for i := start; i < len(lines); i++ {
		switch {
		case strings.HasPrefix(lines[i], "╰"):
			return i, i > start
		case strings.HasPrefix(lines[i], "│"):
			continue
		default:
			return 0, false
		}
	}
	return 0, false
}

// sortEntries reorders body in place. One entry is the panel row that opens a
// command plus every continuation row wrapped underneath it, so the rows of an
// entry always move together.
func sortEntries(body []string) {
	type entry struct {
		name string
		rows []string
	}

	entries := make([]entry, 0, len(body))
	for _, row := range body {
		name, opens := commandName(row)
		if !opens {
			if len(entries) == 0 {
				// A continuation with nothing to continue: not a shape this
				// package claims to understand, so nothing moves.
				return
			}
			last := &entries[len(entries)-1]
			last.rows = append(last.rows, row)
			continue
		}
		entries = append(entries, entry{name: name, rows: []string{row}})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	at := 0
	for _, e := range entries {
		for _, row := range e.rows {
			body[at] = row
			at++
		}
	}
}

// commandName reports whether row opens a command entry, and names it. Rich pads
// a panel body with a single space before its first column, so the opening row is
// the one whose text starts exactly one space in from the border; a continuation
// is indented past the name column.
func commandName(row string) (string, bool) {
	text := strings.TrimPrefix(row, "│")
	if !strings.HasPrefix(text, " ") || strings.HasPrefix(text, "  ") {
		return "", false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}
