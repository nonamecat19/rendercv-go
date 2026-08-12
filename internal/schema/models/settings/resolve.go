package settings

import (
	"time"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Resolved is the three settings fields the renderer reads
// (settings.py:11-52). It is a read of a validated block, not a second
// validation: every value here has a declared default, so a document with no
// `settings` block at all resolves to the defaults.
type Resolved struct {
	// CurrentDate is `_resolved_current_date` (`:72-76`): the declared date, or
	// today when the value is the literal `today`.
	CurrentDate time.Time
	// BoldKeywords is `settings.bold_keywords`, deduplicated.
	BoldKeywords []string
	// PDFTitle is `settings.pdf_title`, before its placeholders are substituted.
	PDFTitle string
	// RenderCommand is `settings.render_command`, **after** the CLI's own flags
	// have been merged into it. The CLI writes its flags here rather than
	// reading them twice, which is upstream's shape too: every generator gates
	// on the merged model (`renderer/pdf_png.py:33,63`, `typst.py:23`,
	// `markdown.py:22`, `html.py:26`), never on the flag.
	RenderCommand RenderCommand
}

// RenderCommand is the resolved `settings.render_command` block.
//
// **Reading these back is what makes a document's own switches work.** The port
// used to gate generation on the CLI flag alone, so a CV — or a `--settings`
// overlay — asking for no PDF still got a PDF and both PNGs.
type RenderCommand struct {
	// OutputFolder and the five path templates are `render_command`'s path
	// keys (`settings/render_command.py:22-55`). They are read here for the
	// same reason the switches are: upstream passes
	// `model.settings.render_command.<x>_path` to `resolve_rendercv_file_path`
	// (`renderer/typst.py:26`, `pdf_png.py:36`, and the rest), so a document
	// that names its own output folder gets it.
	//
	// **They were not resolved at all before**, so `settings.render_command.
	// output_folder: from_doc` in an ordinary CV was silently ignored and every
	// artifact went to `rendercv_output`. The CLI could only ever see its own
	// flags, which is why nothing caught it.
	//
	// An empty string means the key was absent, and the format's default
	// applies. The CLI's flags are merged into this block before it is
	// resolved, so a flag already wins over the document here.
	OutputFolder string
	TypstPath    string
	PDFPath      string
	PNGPath      string
	MarkdownPath string
	HTMLPath     string

	DontGenerateTypst    bool
	DontGeneratePDF      bool
	DontGeneratePNG      bool
	DontGenerateMarkdown bool
	DontGenerateHTML     bool
}

// DefaultPDFTitle is `pdf_title`'s declared default (`:32-33`).
const DefaultPDFTitle = "NAME - CV"

// Resolve reads a `settings` block, filling in the declared defaults.
//
// `now` is what the literal `today` resolves to, passed in rather than read from
// the clock so a render is reproducible — it is the same reference date
// validation used, and the CLI's `--current-date` reaches this through it.
func Resolve(node *yamldoc.Node, now time.Time) Resolved {
	out := Resolved{CurrentDate: now, PDFTitle: DefaultPDFTitle}
	if node == nil || node.Kind != yamldoc.KindMapping {
		return out
	}

	for _, item := range node.Items {
		if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
			continue
		}
		switch item.Key {
		case "render_command":
			out.RenderCommand = resolveRenderCommand(item.Value)
		case "current_date":
			if item.Value.Raw == "today" {
				continue
			}
			if parsed, err := time.Parse("2006-01-02", item.Value.Raw); err == nil {
				out.CurrentDate = parsed
			}
		case "bold_keywords":
			out.BoldKeywords = uniqueKeywords(item.Value)
		case "pdf_title":
			out.PDFTitle = item.Value.Raw
		}
	}
	return out
}

// uniqueKeywords is `keep_unique_keywords` (`:54-69`), whose body is
// `list(set(value))`.
//
// **Upstream's result order is a Python set's**, which is hash-dependent and, with
// the default randomized string hashing, differs between runs. This port
// deduplicates in input order instead, because a renderer whose output depends
// on the interpreter's hash seed cannot be diffed against anything.
//
// It mostly does not show: `build_keyword_matcher_pattern` sorts the keywords by
// descending length, so the order only survives between keywords of *equal*
// length, and two equal-length keywords both matching at the same position is
// what it would take to see a difference. Recorded here rather than in
// `divergences.md` because upstream has no fixed behavior to diverge from.
func uniqueKeywords(node *yamldoc.Node) []string {
	if node.Kind != yamldoc.KindSequence {
		return nil
	}

	seen := make(map[string]struct{}, len(node.Elems))
	out := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil || elem.Kind == yamldoc.KindNull {
			continue
		}
		if _, already := seen[elem.Raw]; already {
			continue
		}
		seen[elem.Raw] = struct{}{}
		out = append(out, elem.Raw)
	}
	return out
}

// resolveRenderCommand reads the five generation switches. Anything that is not
// the boolean `true` leaves its switch off, which is upstream's truthiness rule
// for this block (spec 004 §3.20).
func resolveRenderCommand(node *yamldoc.Node) RenderCommand {
	var out RenderCommand
	if node == nil || node.Kind != yamldoc.KindMapping {
		return out
	}
	for _, item := range node.Items {
		if item.Value == nil {
			continue
		}
		text := ""
		if item.Value.Kind == yamldoc.KindString {
			text = item.Value.Raw
		}
		on := item.Value.Kind == yamldoc.KindBool && yamldoc.BoolIsTrue(item.Value.Raw)
		switch item.Key {
		case "output_folder":
			out.OutputFolder = text
		case "typst_path":
			out.TypstPath = text
		case "pdf_path":
			out.PDFPath = text
		case "png_path":
			out.PNGPath = text
		case "markdown_path":
			out.MarkdownPath = text
		case "html_path":
			out.HTMLPath = text
		case "dont_generate_typst":
			out.DontGenerateTypst = on
		case "dont_generate_pdf":
			out.DontGeneratePDF = on
		case "dont_generate_png":
			out.DontGeneratePNG = on
		case "dont_generate_markdown":
			out.DontGenerateMarkdown = on
		case "dont_generate_html":
			out.DontGenerateHTML = on
		}
	}
	return out
}
