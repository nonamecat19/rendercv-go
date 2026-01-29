package cli

import "github.com/nonamecat19/rendercv-go/internal/renderer/generate"

// Path resolution moved to `internal/renderer/generate` so the public API can
// reach it without importing the CLI (spec 016 §1.1). These aliases keep the
// names the CLI has always used; they are the same types and the same
// functions, not a second copy.

// Default output paths (`settings/render_command.py:22-55`). `OUTPUT_FOLDER` is
// itself a placeholder, resolved before the name ones.
const (
	DefaultOutputFolder = generate.DefaultOutputFolder
	DefaultTypstPath    = generate.DefaultTypstPath
	DefaultPDFPath      = generate.DefaultPDFPath
	DefaultMarkdownPath = generate.DefaultMarkdownPath
	DefaultHTMLPath     = generate.DefaultHTMLPath
	DefaultPNGPath      = generate.DefaultPNGPath
)

// PathInput is what `resolve_rendercv_file_path` reads besides the template.
type PathInput = generate.PathInput

// ResolvePath is `resolve_rendercv_file_path` (path_resolver.py:40-109).
var ResolvePath = generate.ResolvePath
