// Package document is `render_full_template` (templater.py:50-127): the
// orchestration that turns a validated model into a rendered document.
//
// **One function, two formats**, which is upstream's shape — `file_type` is a
// parameter there, not a second function. The Typst and Markdown documents share
// the model, the processors and the assembly and differ in three places: the
// template directory, the string-processor chain, and whether a preamble is
// rendered at all.
//
// **It ends at a string.** Compiling the Typst is iteration 10's.
package document

import (
	"errors"
	"path/filepath"
	"unicode/utf8"

	"github.com/flosch/pongo2/v6"

	"github.com/nonamecat19/rendercv-go/internal/renderer/bridge"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater"
	"github.com/nonamecat19/rendercv-go/internal/renderer/templater/process"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
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

// Render is `render_full_template(model, file_type)` — one function, both
// formats, because that is upstream's own shape.
//
// The three format-dependent decisions are §1 behavior 3's: the template
// directory, the string-processor chain (`process.Run` picks it from the same
// format), and whether a preamble exists at all. Everything else — the bridge,
// the entry expansion, the section loop, the separators — is shared, which is
// why the Markdown document needed no new pipeline.
//
// The order is upstream's and each step reads the one before it: the model is
// bridged, processed, and only then rendered — the header's connections are
// already formatted strings by the time the header template sees them.
//
// **A URL photo is reported, not rendered.** `download_photo_from_url` is the
// pipeline's only network access (spec 009 §4 behavior 15) and nothing here
// reaches the network. A *local* photo needs no download and is rendered; a URL
// one returns `ErrPhotoDownloadUnsupported`, because the alternative — which
// this code did until a verifier measured it — is a header silently missing its
// whole `#grid`, with exit 0 and no warning.
func Render(document bridge.Document, format templater.Format, options Options) (string, error) {
	registry := options.Registry
	if registry == nil {
		registry = entries.Default()
	}

	model, err := bridge.Model(document, registry)
	if err != nil {
		return "", err
	}
	typst := format == templater.FormatTypst
	processed := process.Run(model, processFormat(format))

	environment, err := templater.NewEnvironment(
		options.InputDir, templater.BuiltinTemplates(), themeOf(document))
	if err != nil {
		return "", err
	}

	photo, err := photoOf(document)
	if err != nil {
		return "", err
	}

	context := contextOf(processed, document, photo)
	if !typst {
		// **The Markdown header reads the fields, not the connections**
		// (spec 011 §2). They are added rather than substituted because the
		// Typst names stay valid — one context serves both templates, as
		// upstream's one model does.
		if block, ok := context["cv"].(pongo2.Context); ok {
			for name, value := range bridge.MarkdownFields(document) {
				block[name] = value
			}
		}
	}

	// **Only the Typst document has a preamble** (`:82-89`); the Markdown one
	// opens with its header, and `Assemble` is told which shape to build.
	preamble := ""
	if typst {
		preamble, err = environment.Render(format, templater.FragmentPreamble, context, nil)
		if err != nil {
			return "", err
		}
	}

	header, err := environment.Render(format, templater.FragmentHeader, context, nil)
	if err != nil {
		return "", err
	}

	sections := make([]templater.RenderedSection, 0, len(processed.Sections))
	for _, section := range processed.Sections {
		rendered, err := renderSection(environment, format, context, section)
		if err != nil {
			return "", err
		}
		sections = append(sections, rendered)
	}

	return templater.Assemble(preamble, header, sections, typst), nil
}

// RenderHTML is `render_html` (templater.py:130-155): the Markdown document
// converted to an HTML body, then wrapped by the single `Full.html` fragment.
//
// **It takes the Markdown document's bytes, not the model** (`html.py:31-33`),
// which is why the two artifacts are ordered rather than independent — a wrong
// `.md` is a wrong `.html`, and upstream disables the HTML entirely when the
// Markdown was not generated.
func RenderHTML(doc bridge.Document, markdown string, options Options) (string, error) {
	body, err := process.MarkdownToHTML(markdown)
	if err != nil {
		return "", err
	}

	environment, err := templater.NewEnvironment(
		options.InputDir, templater.BuiltinTemplates(), themeOf(doc))
	if err != nil {
		return "", err
	}

	model, err := bridge.Model(doc, registryOf(options))
	if err != nil {
		return "", err
	}
	processed := process.Run(model, process.FormatMarkdown)

	photo, err := photoOf(doc)
	if err != nil {
		return "", err
	}

	// **`Full.html` reads a `title` nobody binds.** `render_html` passes only
	// `html_body` (`:153`), so Jinja renders the `<title>` empty — measured, and
	// present in every corpus `.html` as a blank line between the tags. Binding
	// `settings.pdf_title` there would be an improvement, and an artifact diff.
	context := contextOf(processed, doc, photo)
	return environment.Render(templater.FormatHTML, templater.FragmentFull, context, pongo2.Context{
		"html_body": body,
	})
}

func registryOf(options Options) *entries.Registry {
	if options.Registry != nil {
		return options.Registry
	}
	return entries.Default()
}

// processFormat maps the template directory onto the processor chain. They are
// two types because they are two decisions upstream spells with one string, and
// there is no `html` processor chain — the HTML is converted from the finished
// Markdown rather than processed again (spec 008 §3 behavior 14).
func processFormat(format templater.Format) process.Format {
	if format == templater.FormatTypst {
		return process.FormatTypst
	}
	return process.FormatMarkdown
}

// renderSection is the per-section body of `:99-124`: a beginning, one fragment
// per entry, an ending. The three extra context names are upstream's keyword
// arguments, and `entry_type` reaches both the beginning and the ending because
// each branches on it.
func renderSection(
	environment *templater.Environment,
	format templater.Format,
	context pongo2.Context,
	section process.Section,
) (templater.RenderedSection, error) {
	beginning, err := environment.Render(format,
		templater.FragmentSectionBeginning, context, pongo2.Context{
			"section_title":            section.Title,
			"snake_case_section_title": section.SnakeCaseTitle,
			"entry_type":               section.EntryType,
		})
	if err != nil {
		return templater.RenderedSection{}, err
	}

	ending, err := environment.Render(format,
		templater.FragmentSectionEnding, context, pongo2.Context{
			"entry_type": section.EntryType,
		})
	if err != nil {
		return templater.RenderedSection{}, err
	}

	name := "entries/" + section.EntryType
	rendered := make([]string, 0, len(section.Entries))
	for _, entry := range section.Entries {
		out, err := environment.Render(format, name, context,
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
func contextOf(model process.Model, document bridge.Document, photo any) pongo2.Context {
	current := model.CurrentDate

	return pongo2.Context{
		"cv": pongo2.Context{
			"name":     model.Name,
			"headline": model.Headline,
			// The three underscore-prefixed names are upstream's private
			// attributes, which its templates read directly.
			"_plain_name":  model.PlainNameForTemplate(),
			"_connections": model.Connections,
			"_top_note":    model.TopNote,
			"_footer":      model.Footer,
			// **Falsy when there is none**, which is what the header branches
			// on — `{% if cv.photo %}`.
			"photo": photo,
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

// ErrPhotoDownloadUnsupported is what a URL photo gets instead of a wrong
// document. It is not a divergence in output — no document renders differently
// — it is a feature this iteration does not implement, and saying so is the
// difference between a missing feature and a silent corruption.
var ErrPhotoDownloadUnsupported = errors.New(
	"cv.photo is a URL: downloading it is not implemented yet (spec 009 §4 behavior 15)")

// photoOf is the header's `cv.photo`.
//
// Upstream's templates read two things from it: its truthiness, and `.name` —
// `pathlib.Path.name`, the basename, because the Typst compiler resolves the
// image relative to the output directory the file was copied into. So a mapping
// with a `name` key is exactly the surface the template uses.
func photoOf(document bridge.Document) (any, error) {
	model := document.Model
	if model == nil || model.CvModel == nil || model.CvModel.PhotoValue == nil {
		return nil, nil
	}

	photo := model.CvModel.PhotoValue
	if photo.Kind == cv.PhotoKindURL {
		return nil, ErrPhotoDownloadUnsupported
	}
	return pongo2.Context{"name": filepath.Base(photo.Path.Value)}, nil
}

func themeOf(document bridge.Document) string {
	if theme, ok := document.Design["theme"].(string); ok && theme != "" {
		return theme
	}
	return "classic"
}
