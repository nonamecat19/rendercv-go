// Package typstdoc is `render_full_template` for the Typst format
// (templater.py:50-127): the orchestration that turns a validated model into a
// `.typ` string.
//
// **It ends at a string.** Compiling that string is iteration 10's, and the
// Markdown document — which shares every piece here — is iteration 11's.
package typstdoc

import (
	"unicode/utf8"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

// Options are the render's inputs beyond the document.
type Options struct {
	// InputDir is the input file's directory, which the loader searches before
	// the built-in templates so a user override wins (spec 008 §2).
	InputDir string
	// Registry is the entry-type registry the sections are read with.
	Registry *entries.Registry
}

// Render is `render_full_template(model, "typst")`.
//
// The order is upstream's and each step reads the one before it: the model is
// bridged, processed, and only then rendered — the header's connections are
// already formatted strings by the time the header template sees them.
//
// **The photo is not downloaded.** `download_photo_from_url` is the pipeline's
// only network access (spec 009 §4 behavior 15) and nothing here reaches the
// network; a render whose test suite can touch the network is worse than one
// that cannot. The CLI supplies the download in iteration 12.
func Render(document bridge.Document, options Options) (string, error) {
	registry := options.Registry
	if registry == nil {
		registry = entries.Default()
	}

	model, err := bridge.Model(document, registry)
	if err != nil {
		return "", err
	}
	processed := process.Run(model, process.FormatTypst)

	environment, err := templater.NewEnvironment(
		options.InputDir, templater.BuiltinTemplates(), themeOf(document))
	if err != nil {
		return "", err
	}

	context := contextOf(processed, document)

	preamble, err := environment.Render(templater.FormatTypst,
		templater.FragmentPreamble, context, nil)
	if err != nil {
		return "", err
	}
	header, err := environment.Render(templater.FormatTypst,
		templater.FragmentHeader, context, nil)
	if err != nil {
		return "", err
	}

	sections := make([]templater.RenderedSection, 0, len(processed.Sections))
	for _, section := range processed.Sections {
		rendered, err := renderSection(environment, context, section)
		if err != nil {
			return "", err
		}
		sections = append(sections, rendered)
	}

	return templater.Assemble(preamble, header, sections, true), nil
}

// renderSection is the per-section body of `:99-124`: a beginning, one fragment
// per entry, an ending. The three extra context names are upstream's keyword
// arguments, and `entry_type` reaches both the beginning and the ending because
// each branches on it.
func renderSection(
	environment *templater.Environment,
	context pongo2.Context,
	section process.Section,
) (templater.RenderedSection, error) {
	beginning, err := environment.Render(templater.FormatTypst,
		templater.FragmentSectionBeginning, context, pongo2.Context{
			"section_title":            section.Title,
			"snake_case_section_title": section.SnakeCaseTitle,
			"entry_type":               section.EntryType,
		})
	if err != nil {
		return templater.RenderedSection{}, err
	}

	ending, err := environment.Render(templater.FormatTypst,
		templater.FragmentSectionEnding, context, pongo2.Context{
			"entry_type": section.EntryType,
		})
	if err != nil {
		return templater.RenderedSection{}, err
	}

	name := "entries/" + section.EntryType
	rendered := make([]string, 0, len(section.Entries))
	for _, entry := range section.Entries {
		out, err := environment.Render(templater.FormatTypst, name, context,
			pongo2.Context{"entry": entryContext(entry)})
		if err != nil {
			return templater.RenderedSection{}, err
		}
		rendered = append(rendered, out)
	}

	return templater.RenderedSection{
		Beginning: beginning,
		Entries:   rendered,
		Ending:    ending,
	}, nil
}

// entryContext is the entry as a template sees it.
//
// **Every string field gains a `…_lines` companion**, which is the transform's
// `splitlines` rule (`tools/gentemplates`): upstream's Jinja calls
// `entry.main_column.splitlines()` and pongo2 cannot, so the split happens here.
// A `TextEntry` is a bare string instead, because its template is `{{entry}}`.
func entryContext(entry process.Entry) any {
	if entry.IsText {
		return entry.Text
	}

	out := make(pongo2.Context, len(entry.Fields)*2)
	for name, value := range entry.Fields {
		out[name] = value
		if text, isText := value.(string); isText {
			out[name+"_lines"] = splitLines(text)
		}
	}
	return out
}

// lineBoundaries is every character Python's `str.splitlines()` breaks on
// (CPython's `STRINGLIB(splitlines)`), beyond the `\r\n` pair it treats as one.
//
// **Splitting on `\n` alone is not the same function.** A `summary` written with
// Windows line endings leaves a bare carriage return inside the rendered Typst —
// `#summary[one\rtwo]` — and a ` ` produces one line where upstream
// produces two. Both were measured; neither is in the corpus, which is why the
// byte differential could not find them.
var lineBoundaries = []rune{
	'\n', '\v', '\f', '\r',
	'', '', '', // file, group and record separators
	'',      // next line
	' ', ' ', // line and paragraph separators
}

// splitLines is Python's `str.splitlines()`.
//
// **The empty string splits to nothing**, not to one empty line, and
// `EducationEntry` branches on that length being zero — so a split that returned
// `[""]` would take the wrong branch and produce a first row of one blank line.
//
// A trailing boundary produces no final empty element, which is the same rule:
// `"a\n"` is one line, not two.
func splitLines(text string) []string {
	out := []string{}
	start := 0

	for index := 0; index < len(text); {
		width := 1
		boundary := false
		for _, candidate := range lineBoundaries {
			if size := runeAt(text, index, candidate); size > 0 {
				width, boundary = size, true
				break
			}
		}
		if !boundary {
			_, size := utf8.DecodeRuneInString(text[index:])
			index += size
			continue
		}

		// `\r\n` is **one** boundary, not two — otherwise every Windows line
		// gains an empty line after it.
		if text[index] == '\r' && index+1 < len(text) && text[index+1] == '\n' {
			width = 2
		}
		out = append(out, text[start:index])
		index += width
		start = index
	}

	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

// runeAt reports the width of `candidate` at `index`, or 0 when it is not there.
func runeAt(text string, index int, candidate rune) int {
	decoded, size := utf8.DecodeRuneInString(text[index:])
	if decoded != candidate {
		return 0
	}
	return size
}

// contextOf is the four names `render_single_template` always passes
// (`:209-214`), shaped as the templates address them.
func contextOf(model process.Model, document bridge.Document) pongo2.Context {
	current := model.CurrentDate

	return pongo2.Context{
		"cv": pongo2.Context{
			"name":     model.Name,
			"headline": model.Headline,
			// The three underscore-prefixed names are upstream's private
			// attributes, which its templates read directly.
			"_plain_name":  model.PlainName,
			"_connections": model.Connections,
			"_top_note":    model.TopNote,
			"_footer":      model.Footer,
			// The photo is falsy until the download of §4 behavior 15 is wired
			// in; the header branches on it.
			"photo": "",
		},
		"design": document.Design,
		"locale": pongo2.Context{
			"language_iso_639_1": document.Locale.ISOCode(),
			"is_rtl":             document.Locale.IsRTL(),
		},
		"settings": pongo2.Context{
			"pdf_title": model.PDFTitle,
			"_resolved_current_date": pongo2.Context{
				"year":  current.Year(),
				"month": int(current.Month()),
				"day":   current.Day(),
			},
		},
	}
}

func themeOf(document bridge.Document) string {
	if theme, ok := document.Design["theme"].(string); ok && theme != "" {
		return theme
	}
	return "classic"
}
