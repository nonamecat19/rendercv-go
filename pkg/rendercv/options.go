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

	// OutputFolder, TypstPath, PDFPath, PNGPath, MarkdownPath and HTMLPath are
	// the output folder and the five path templates. An empty string means the
	// key is absent, so the document's own settings decide. All six resolve
	// against the input file's directory, not the working directory
	// (path.py:37-41). Placeholders such
	// as OUTPUT_FOLDER and NAME_IN_SNAKE_CASE are substituted at render time
	// (path_resolver.py:40-109).
	OutputFolder string
	TypstPath    string
	PDFPath      string
	PNGPath      string
	MarkdownPath string
	HTMLPath     string

	// DontGenerateTypst, DontGeneratePDF, DontGeneratePNG,
	// DontGenerateMarkdown and DontGenerateHTML are the five dont_generate_*
	// flags. true suppresses a format; false is the same as leaving it out, so
	// the document's own settings decide.
	//
	// Upstream types these `bool | None`, which looks like three states, but
	// its override loop is `if value:` (rendercv_model_builder.py:149) — so an
	// explicit False is dropped exactly as an absent key is, and there is no
	// third state to represent. Measured against the vendored Python: passing
	// dont_generate_pdf=False leaves the key absent from the merged dictionary.
	//
	// A *bool here would let a caller express something upstream cannot, which
	// would be a divergence rather than a convenience.
	DontGenerateTypst    bool
	DontGeneratePDF      bool
	DontGeneratePNG      bool
	DontGenerateMarkdown bool
	DontGenerateHTML     bool

	// Overrides are the dotted key/value pairs the CLI parses from
	// `--key=value`, mirroring the overrides dict. A library caller supplies
	// them already parsed.
	Overrides map[string]string
}
