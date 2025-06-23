package templater

import (
	"errors"
	"strings"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
)

// registerFilters installs the seven filters the templates use.
//
// Five are Jinja builtins — `indent`, `length`, `lower`, `replace`, `string` —
// and two are upstream's own, `clean_url` and `strip`. **Neither custom filter
// appears in any template** (spec 008 §4F behavior 53); they are registered
// because upstream registers them, so a user override that uses one works.
//
// **pongo2 has no `indent` at all**, which is a better outcome than the clash
// the plan expected: there is nothing to override, so the filter below is simply
// Jinja's. pongo2 does ship `lower`, `length` and `replace`.
func registerFilters() error {
	// **Autoescaping off, and this is not a preference.** pongo2 defaults to
	// HTML-escaping every `{{ }}`, so `<`, `>`, `&` and `"` become entities. A
	// `.typ` is full of all four — `escape_typst_characters` emits `\"` and
	// `\<` deliberately — so leaving it on corrupts every Typst document at
	// every interpolation. Jinja's default is no autoescaping, which is what
	// upstream relies on.
	pongo2.SetAutoescape(false)

	if err := registerFilter("indent", filterIndent); err != nil {
		return err
	}
	if err := registerFilter("replace", filterReplace); err != nil {
		return err
	}
	if err := registerFilter("clean_url", filterCleanURL); err != nil {
		return err
	}
	return registerFilter("strip", filterStrip)
}

// filterIndent is Jinja's `indent(s, width=4, first=False, blank=False)`: it
// indents every line **except the first**, and skips blank lines.
//
// Both defaults are load-bearing. Four `|replace("    ", "")` sites in the
// templates undo an `indent(4)` and cancel exactly **only** because the first
// line was never indented (spec 008 §4F behavior 57) — an implementation that
// indented it would add four spaces at the front of every block and the
// `replace` would take four off somewhere else. That is a whitespace diff on
// most corpus cases, and `TestIndentAndReplaceCancel` is what catches it.
func filterIndent(in, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	width := 4
	if param != nil && param.IsInteger() {
		width = param.Integer()
	}

	lines := strings.Split(in.String(), "\n")
	indent := strings.Repeat(" ", width)
	for i := 1; i < len(lines); i++ {
		// Blank lines are left alone: Jinja's `blank` defaults to False.
		if lines[i] == "" {
			continue
		}
		lines[i] = indent + lines[i]
	}
	return pongo2.AsValue(strings.Join(lines, "\n")), nil
}

// filterReplace is Jinja's two-argument `replace`, which pongo2 has **no**
// equivalent for — its filters take exactly one parameter, and its `cut` only
// removes.
//
// The parameter is therefore `sed`-shaped: a separator character, the search
// string, the separator, the replacement, the separator. `|replace:"/-/ /"` is
// `replace("-", " ")`. The separator is the parameter's first character, so a
// pattern containing `/` picks another.
//
// This is a **template-source divergence and not an output one** (plan §2): the
// transform rewrites the eight call sites, and the byte diff against goldens is
// what proves they still mean the same thing.
func filterReplace(in, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	spec := param.String()
	if len(spec) < 3 {
		return nil, &pongo2.Error{Sender: "filter:replace", OrigError: errBadReplaceSpec}
	}

	separator := spec[:1]
	parts := strings.Split(spec, separator)
	// "", from, to, "" — a leading and a trailing empty field.
	if len(parts) != 4 {
		return nil, &pongo2.Error{Sender: "filter:replace", OrigError: errBadReplaceSpec}
	}
	return pongo2.AsValue(strings.ReplaceAll(in.String(), parts[1], parts[2])), nil
}

var errBadReplaceSpec = errors.New(
	`replace expects a sed-shaped parameter, as in |replace:"/-/ /"`)

func filterCleanURL(in, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(process.CleanURL(in.String())), nil
}

func filterStrip(in, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(process.Strip(in.String())), nil
}

// registerFilter wraps pongo2's global registry, which returns an error rather
// than panicking on a duplicate. Registration is idempotent because the registry
// is process-wide and the tests build several environments.
func registerFilter(name string, filter pongo2.FilterFunction) error {
	if pongo2.FilterExists(name) {
		return nil
	}
	return pongo2.RegisterFilter(name, filter)
}
