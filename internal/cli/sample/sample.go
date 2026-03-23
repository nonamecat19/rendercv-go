// Package sample generates the starter CV `rendercv-go new` writes.
//
// It mirrors `schema/sample_generator.py`'s `create_sample_yaml_input_file`
// (`:97-200`), which is four string transforms over one pydantic dump:
//
//	dump ──▶ nested-bullet regex ──▶ schema hint ──▶ comment surgery ──▶ file
//
// The transforms are here. The dump is not: it is `RenderCVModel`'s
// `model_dump_json(exclude_none=False, by_alias=True)` reloaded through
// `json.loads` (`:217-221`) and then written by ruamel, which is a pydantic
// serializer and a YAML emitter the port does not have. It is captured instead,
// by tools/sampleprobe, and stored decomposed: one `cv` block, nine `design`
// blocks, twenty-two `locale` blocks, one `settings` block.
//
// **Why 33 blocks and not 198 documents.** Spec 013 §3.1 behavior 14: the theme
// and the locale are the document's only two axes and they are independent —
// `create_sample_rendercv_pydantic_model` passes each to its own adapter and
// nothing else reads either (`:75-77`). The probe asserts it for every pair
// rather than trusting it.
//
// **What holds the capture to upstream.** Nothing here derives the dump, so a
// field pydantic adds, drops or reorders would go unnoticed if the blocks were
// only ever compared against a fixture. They are not:
// upstream_conformance_test.go generates all 198 documents from the vendored
// Python on every run and diffs them against this package, so a submodule bump
// that moves the dump turns red on the next `just test-parity` — rather than
// waiting for someone to rerun the probe.
//
// **What is still generated rather than captured.** The name — the one value a
// user supplies — is emitted here (see scalar.go), and so are all four
// transforms, so a name that changes the document's *shape* (a multi-line name
// becomes a literal block, and its continuation lines can themselves match the
// nested-bullet regex) comes out right rather than being pasted in.
package sample

import (
	"embed"
	"fmt"
	"regexp"
	"strings"

	"github.com/nonamecat19/rendercv-go/internal/version"
)

// blocks are the raw dumps tools/sampleprobe captured: what
// `dictionary_to_yaml` returns for each of the model's four top-level keys,
// before any of the transforms below. GENERATED — see tools/sampleprobe. Data,
// not an expectation: never hand-tuned to make a test pass, and what they must
// be is upstream_conformance_test.go's differential, not this file.
//
//go:embed blocks
var blocks embed.FS

// Generate is `create_sample_yaml_input_file` (`:97-200`).
//
// The two `RenderCVUserError`s upstream raises here for an unknown theme or
// locale (`:128-141`, spec 013 §4.3) are **not** reproduced: `new` checks both
// first and with different wording (`cli/new_command/new_command.py:65-77`), so
// upstream's text is unreachable from any CLI invocation and inventing a Go
// rendering of a Python list repr would put a string in the binary that no run
// can print. The error below is the port's own, and no command surfaces it.
func Generate(name, theme, locale string) (string, error) {
	cv, err := block("cv.yaml")
	if err != nil {
		return "", err
	}
	design, err := block("design/" + theme + ".yaml")
	if err != nil {
		return "", fmt.Errorf("no design block for theme %q: %w", theme, err)
	}
	localeBlock, err := block("locale/" + locale + ".yaml")
	if err != nil {
		return "", fmt.Errorf("no locale block for locale %q: %w", locale, err)
	}
	settings, err := block("settings.yaml")
	if err != nil {
		return "", err
	}

	// The name is substituted into the *model*, before the dump
	// (`:73`), so it is substituted before the transforms here too. It matters:
	// a multi-line name's continuation lines go through the nested-bullet
	// regex like any other line.
	cv, err = withName(cv, name)
	if err != nil {
		return "", err
	}

	document := cv + design + localeBlock + settings
	document = splitNestedBullets(document)
	document = schemaHint + document
	return commentDesignAndLocale(document, theme, locale)
}

// FileName is `new`'s output path (`cli/new_command/new_command.py:81`): every
// space becomes an underscore and **nothing else is sanitized** — not a slash,
// not a newline, not a leading dot.
func FileName(name string) string {
	return strings.ReplaceAll(name, " ", "_") + "_CV.yaml"
}

// schemaHint is line 1 of every starter CV (`:161-166`).
//
// **The URL keeps `rendercv/rendercv`.** It addresses upstream's repository and
// upstream's published schema, so the binary-name divergence does not reach it
// (spec 013 §4.4).
const schemaHint = "# yaml-language-server:" +
	" $schema=https://raw.githubusercontent.com/rendercv/rendercv/refs/tags/v" +
	version.RenderCV + "/schema.json\n"

// namePrefix is the second line of the captured `cv` block: `name` is `Cv`'s
// first field, so its line is the first under `cv:` and the probe refuses to
// write a block where it is not.
const namePrefix = "  name: "

// withName replaces the captured block's name line with the user's, rendered
// the way ruamel would have rendered it had it been in the model (`:73`).
func withName(cv, name string) (string, error) {
	const head = "cv:\n" + namePrefix
	if !strings.HasPrefix(cv, head) {
		return "", fmt.Errorf("the cv block does not start with %q", head)
	}
	_, rest, found := strings.Cut(cv[len("cv:\n"):], "\n")
	if !found {
		return "", fmt.Errorf("the cv block's name line is not terminated")
	}
	return "cv:\n" + nameLine(name) + rest, nil
}

// listItem matches a line that is a YAML list item, whatever its indentation —
// `re.match(r"\s+- ", line)` (`:156`). Python's `\s` for a `str` pattern is the
// Unicode set, which is why this is not just a space.
var listItem = regexp.MustCompile(`^[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{2028}\x{2029}\p{Zs}]+- `)

// nestedBulletIndent is the literal twelve spaces `:154` inserts, **regardless
// of the line's own indentation** (spec 013 §3.1 behavior 8). A bullet at any
// other depth is re-indented to twelve; that is the rule, not a bug to fix.
const nestedBulletIndent = 12

// splitNestedBullets is `:151-159`.
//
// `sample_content.yaml:75-78` writes a nested bullet list as a plain multi-line
// scalar, YAML folds it onto one line, and `width = 9999` keeps it there. This
// splits it back apart: inside a list-item line, every ` - ` that is neither
// preceded nor followed by a space becomes a line break and a fresh bullet.
func splitNestedBullets(document string) string {
	lines := strings.Split(document, "\n")
	for i, line := range lines {
		if !listItem.MatchString(line) {
			continue
		}
		lines[i] = replaceBulletSeparators(line)
	}
	return strings.Join(lines, "\n")
}

// replaceBulletSeparators is `re.sub(r"(?<! ) - (?! )", …, line)`. Go's regexp
// has no look-around, so the two guards are spelled out; the scan is
// left-to-right and non-overlapping, as `re.sub`'s is.
func replaceBulletSeparators(line string) string {
	const separator = " - "
	replacement := "\n" + strings.Repeat(" ", nestedBulletIndent) + "- "

	var out strings.Builder
	for at := 0; ; {
		next := strings.Index(line[at:], separator)
		if next < 0 {
			out.WriteString(line[at:])
			return out.String()
		}
		start := at + next
		end := start + len(separator)
		precededBySpace := start > 0 && line[start-1] == ' '
		followedBySpace := end < len(line) && line[end] == ' '
		out.WriteString(line[at:start])
		if precededBySpace || followedBySpace {
			out.WriteString(separator)
		} else {
			out.WriteString(replacement)
		}
		at = end
	}
}

// commentDesignAndLocale is `:168-195`: the design and locale blocks are
// commented out **by string surgery on the dump**, not by re-serializing
// anything, and the settings block is left alone (`:191`).
//
// The three cuts are literal: `design:\n  theme: <theme>\n`,
// `locale:\n  language: <locale>\n` and `settings:\n`. `str.split` is
// unanchored and the code indexes `[0]` and `[1]` unconditionally, so a
// document whose *content* contains one of the three markers — a name of
// `"settings:\nx"`, say — is cut at the first occurrence. Spec 013 §5.3.3 calls
// that a latent upstream defect; reproducing the split rule rather than a
// correct one is the point.
func commentDesignAndLocale(document, theme, locale string) (string, error) {
	designMarker := "design:\n  theme: " + theme + "\n"
	localeMarker := "locale:\n  language: " + locale + "\n"
	const settingsMarker = "settings:\n"

	parts := strings.Split(document, designMarker)
	if len(parts) < 2 {
		return "", fmt.Errorf("the dump has no %q", designMarker)
	}
	cvField := parts[0]

	parts = strings.Split(parts[1], localeMarker)
	if len(parts) < 2 {
		return "", fmt.Errorf("the dump has no %q", localeMarker)
	}
	designField := designMarker + commentBlock(parts[0]) + "\n"

	parts = strings.Split(parts[1], settingsMarker)
	if len(parts) < 2 {
		return "", fmt.Errorf("the dump has no %q", settingsMarker)
	}
	localeField := localeMarker + commentBlock(parts[0]) + "\n"
	settingsField := settingsMarker + parts[1]

	return cvField + designField + localeField + settingsField, nil
}

// commentBlock is `f"  {line.replace('  ', '# ', 1)}"` over
// `splitlines(keepends=False)`, joined with `\n` (`:175-179`).
//
// The rule is mechanical and its consequences are the spec's §6.1: the first
// two-space run becomes `# `, then two spaces go in front, so a depth-1 key
// `  page:` becomes `  # page:` and a depth-2 key `    size: us-letter` becomes
// `  #   size: us-letter`. A line with no two-space run keeps its text and gains
// the two-space prefix; an empty line becomes exactly two spaces.
func commentBlock(body string) string {
	lines := splitLines([]rune(body))
	for i, line := range lines {
		lines[i] = "  " + strings.Replace(line, "  ", "# ", 1)
	}
	return strings.Join(lines, "\n")
}

// block reads one captured block.
func block(name string) (string, error) {
	content, err := blocks.ReadFile("blocks/" + name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
