// Command gentemplates rewrites upstream's Jinja templates into pongo2 source.
//
// **Template source may diverge; template output may not** (`AGENTS.md` §6.1,
// spec 008 plan §2). This is a build-time step, so what ships is Go-embedded
// text a reviewer can read beside upstream's file — not a runtime translation
// layer whose behavior is invisible.
//
// **What this tool cannot check.** Nothing here proves a rewrite preserves
// meaning; the byte diff against `testdata/golden` does, and until Wave C runs
// it, a wrong rule is undetected. That is the same limitation `tools/localeprobe`
// states, and it is worse here because the transform is *semantic* rather than a
// data copy — `localeprobe` can only mistranscribe, this can change a document.
//
// The rules are in transforms below, each with the upstream construct it
// rewrites and the count of sites it fires on. A rule that fires zero times is
// an error: it means the templates changed and the rule is stale.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	submodule    = "third_party/rendercv"
	sourceDir    = "src/rendercv/renderer/templater/templates"
	generatedDir = "internal/renderer/templater/templates"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gentemplates: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repositoryRoot()
	if err != nil {
		return err
	}

	source := filepath.Join(root, submodule, sourceDir)
	target := filepath.Join(root, generatedDir)

	if err := os.RemoveAll(target); err != nil {
		return err
	}

	counts := map[string]int{}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		out := filepath.Join(target, relative)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, []byte(transform(string(raw), counts)), 0o644)
	})
	if err != nil {
		return fmt.Errorf("walking %s (is the submodule initialized? `just setup`): %w", source, err)
	}

	var stale []string
	for _, rule := range transforms {
		if counts[rule.name] == 0 && rule.mustFire {
			stale = append(stale, rule.name)
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("these rules fired on nothing, so they are stale: %s",
			strings.Join(stale, ", "))
	}

	for _, rule := range transforms {
		fmt.Printf("%-24s %d\n", rule.name, counts[rule.name])
	}
	return nil
}

// rule is one rewrite.
type rule struct {
	name string
	// mustFire makes a zero count an error, which is how a rule that upstream
	// removed gets noticed instead of silently doing nothing.
	mustFire bool
	apply    func(string) (string, int)
}

var transforms = []rule{
	// **Whitespace first**, because the later rules change line contents and
	// `lstrip_blocks` cares about what precedes a tag on its line.
	{name: "lstrip_blocks", mustFire: true, apply: applyLstripBlocks},
	{name: "trim_blocks", mustFire: true, apply: applyTrimBlocks},

	{name: "as_rgb", mustFire: true, apply: replaceAll(`.as_rgb()`, ``)},
	{name: "splitlines", mustFire: true, apply: replaceAll(`.splitlines()`, `_lines`)},

	{name: "indent_args", mustFire: true, apply: regexpRule(
		regexp.MustCompile(`\|indent\((\d+)\)`), `|indent:$1`)},
	{name: "replace_args", mustFire: true, apply: applyReplaceArgs},
	{name: "slices", mustFire: true, apply: applySlices},
	{name: "in_list", mustFire: true, apply: applyInList},

	// **Last**, because it is about the file's end and every rule above can
	// change it.
	{name: "keep_trailing_newline", mustFire: true, apply: applyKeepTrailingNewline},
}

func transform(source string, counts map[string]int) string {
	for _, rule := range transforms {
		out, fired := rule.apply(source)
		counts[rule.name] += fired
		source = out
	}
	return source
}

// applyLstripBlocks strips whitespace from the start of a line up to a block
// tag, which is what Jinja's `lstrip_blocks=True` does (templater.py:43).
//
// **Only block tags**, not `{{ }}` — an expression keeps its indentation, which
// is what makes `    {{ line|indent:4 }}` produce four spaces plus four.
func applyLstripBlocks(source string) (string, int) {
	lines := strings.Split(source, "\n")
	fired := 0
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == line || !strings.HasPrefix(trimmed, "{%") {
			continue
		}
		lines[i] = trimmed
		fired++
	}
	return strings.Join(lines, "\n"), fired
}

// applyTrimBlocks removes the newline immediately after a block tag, which is
// Jinja's `trim_blocks=True` (`:42`).
//
// It runs after lstrip so the two compose the way Jinja composes them: a line
// containing only a block tag disappears entirely.
func applyTrimBlocks(source string) (string, int) {
	var out strings.Builder
	fired := 0

	for i := 0; i < len(source); i++ {
		out.WriteByte(source[i])
		if source[i] != '}' || i == 0 || source[i-1] != '%' {
			continue
		}
		if i+1 < len(source) && source[i+1] == '\n' {
			i++
			fired++
		}
	}
	return out.String(), fired
}

// applyReplaceArgs rewrites Jinja's two-argument `replace` into the port's
// sed-shaped single parameter (plan §2).
//
// The separator is chosen per call site: the first character not present in
// either argument. Being deterministic matters more than being pretty, because
// the generated file is reviewed against upstream's.
var replaceCallPattern = regexp.MustCompile(
	`\|replace\(\s*"([^"]*)"\s*,\s*"([^"]*)"\s*\)`)

func applyReplaceArgs(source string) (string, int) {
	fired := 0
	out := replaceCallPattern.ReplaceAllStringFunc(source, func(match string) string {
		groups := replaceCallPattern.FindStringSubmatch(match)
		from, to := groups[1], groups[2]

		separator := ""
		for _, candidate := range []string{"/", "|", "!", "~", "^"} {
			if !strings.Contains(from, candidate) && !strings.Contains(to, candidate) {
				separator = candidate
				break
			}
		}
		if separator == "" {
			panic(fmt.Sprintf("gentemplates: no separator available for replace(%q, %q)", from, to))
		}

		fired++
		return `|replace:"` + separator + from + separator + to + separator + `"`
	})
	return out, fired
}

// applySlices rewrites Python's four slice forms.
//
// **Not into pongo2's `slice` filter.** That takes a *string literal* — `":2"` —
// so a computed bound cannot be expressed: `|slice:":first_row_lines"` passes
// the variable's **name** as text. Two of the four sites have computed bounds
// (spec 008 §4F behavior 55), which is exactly the case plan §2's table assumed
// `slice` would cover.
//
// So the port registers `takefirst` and `dropfirst`, whose parameter is an
// ordinary expression and therefore may be a variable:
//
//	x[:n]  → x|takefirst:n
//	x[n:]  → x|dropfirst:n
//	x[1:]  → x|dropfirst:1
//	x[0]   → x.0        — an index, not a slice
var (
	sliceIndexPattern = regexp.MustCompile(`(\w+(?:\.\w+)*)\[(\d+)\]`)
	sliceRangePattern = regexp.MustCompile(`(\w+(?:\.\w+)*)\[([\w]*):([\w]*)\]`)
)

func applySlices(source string) (string, int) {
	fired := 0

	source = sliceRangePattern.ReplaceAllStringFunc(source, func(match string) string {
		groups := sliceRangePattern.FindStringSubmatch(match)
		start, stop := groups[2], groups[3]
		fired++
		switch {
		case start == "" && stop != "":
			return groups[1] + "|takefirst:" + stop
		case start != "" && stop == "":
			return groups[1] + "|dropfirst:" + start
		}
		panic(fmt.Sprintf("gentemplates: unhandled slice %q — only [:n] and [n:] appear upstream", match))
	})
	source = sliceIndexPattern.ReplaceAllStringFunc(source, func(match string) string {
		groups := sliceIndexPattern.FindStringSubmatch(match)
		fired++
		return groups[1] + "." + groups[2]
	})
	return source, fired
}

// applyKeepTrailingNewline strips one final newline from the template source,
// which is Jinja's `keep_trailing_newline=False` default — it happens at parse
// time and upstream never overrides it (templater.py:34-44).
//
// pongo2 keeps it. Missing this adds one `\n` to **every** fragment, and
// `Assemble` joins entries with `"\n\n"`, so the result is a blank line per
// entry and per section on every artifact case.
//
// It was found by a fragment-level differential — upstream's `.j2` through Jinja
// against the transformed source through pongo2 — which showed 55 of 85 renders
// differing and all 85 agreeing once one trailing newline came off. The rule is
// **exactly one** newline, not a trim.
func applyKeepTrailingNewline(source string) (string, int) {
	if !strings.HasSuffix(source, "\n") {
		return source, 0
	}
	return source[:len(source)-1], 1
}

// applyInList rewrites Jinja's `x in ["A", "B"]` into a disjunction.
//
// pongo2's parser has **no list literal**, so the membership test cannot be
// written at all. Both upstream sites have a single member, which makes the
// rewrite an equality — but the general form is emitted so a user override with
// two members still works, and so the rule does not quietly depend on the
// one-member coincidence.
var inListPattern = regexp.MustCompile(`(\w+(?:\.\w+)*) in \[([^\]]*)\]`)

func applyInList(source string) (string, int) {
	fired := 0
	out := inListPattern.ReplaceAllStringFunc(source, func(match string) string {
		groups := inListPattern.FindStringSubmatch(match)
		subject := groups[1]

		var comparisons []string
		for _, member := range strings.Split(groups[2], ",") {
			member = strings.TrimSpace(member)
			if member == "" {
				continue
			}
			comparisons = append(comparisons, subject+" == "+member)
		}
		fired++
		return strings.Join(comparisons, " or ")
	})
	return out, fired
}

func replaceAll(from, to string) func(string) (string, int) {
	return func(source string) (string, int) {
		count := strings.Count(source, from)
		return strings.ReplaceAll(source, from, to), count
	}
}

func regexpRule(pattern *regexp.Regexp, replacement string) func(string) (string, int) {
	return func(source string) (string, int) {
		count := len(pattern.FindAllString(source, -1))
		return pattern.ReplaceAllString(source, replacement), count
	}
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
