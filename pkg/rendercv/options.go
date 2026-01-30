package rendercv

// BuildOptions mirrors BuildRendercvModelArguments
// (schema/rendercv_model_builder.py:24-39), a TypedDict(total=False) in which
// every key is optional. The zero value is "no options", exactly as an empty
// kwargs dict is upstream.
type BuildOptions struct {
	// InputFilePath is the path the document was read from. Upstream declares
	// it as a keyword-only argument beside the kwargs
	// (rendercv_model_builder.py:192-210) rather than as one of them.
	//
	// It is not merely informational: every relative path in a document
	// resolves against this file's directory rather than the working
	// directory, and PDF and PNG generation need it to find a photo. An empty
	// value resolves paths against the working directory.
	InputFilePath string

	// DesignYAML, LocaleYAML and SettingsYAML are the *contents* of the three
	// overlay files, not their paths — mirroring design_yaml_file,
	// locale_yaml_file and settings_yaml_file, which upstream types as str.
	DesignYAML   string
	LocaleYAML   string
	SettingsYAML string

	// The output folder and the five path templates. An empty string means the
	// key is absent, so the document's own settings decide. Placeholders such
	// as OUTPUT_FOLDER and NAME_IN_SNAKE_CASE are substituted at render time
	// (path_resolver.py:40-109).
	OutputFolder string
	TypstPath    string
	PDFPath      string
	PNGPath      string
	MarkdownPath string
	HTMLPath     string

	// The five dont_generate_* flags are tri-state, mirroring upstream's
	// `bool | None`:
	//
	//   nil          the key is absent, so settings.yaml decides
	//   &true        do not generate this format
	//   &false       generate it, overriding a settings file that switched it off
	//
	// A plain bool cannot express the third case, and its zero value would
	// silently mean the first — which is why these are pointers even though
	// most callers will leave them nil.
	DontGenerateTypst    *bool
	DontGeneratePDF      *bool
	DontGeneratePNG      *bool
	DontGenerateMarkdown *bool
	DontGenerateHTML     *bool

	// Overrides are the dotted key/value pairs the CLI parses from
	// `--key=value`, mirroring the overrides dict. A library caller supplies
	// them already parsed.
	Overrides map[string]string
}

// No is a convenience for the common tri-state case: No() is a *bool meaning
// "do not generate this format" — the True of upstream's `bool | None`
// (rendercv_model_builder.py:24-39).
//
// It exists because &true is not expressible as a literal in Go, and
// BuildOptions{DontGeneratePDF: rendercv.No()} reads better than declaring a
// variable at every call site.
func No() *bool {
	value := true
	return &value
}

// Yes is the opposite of [No]: a *bool meaning "generate this format even if a
// settings file switched it off" — the False of upstream's `bool | None`
// (rendercv_model_builder.py:24-39), and the case a plain bool could not reach.
func Yes() *bool {
	value := false
	return &value
}
