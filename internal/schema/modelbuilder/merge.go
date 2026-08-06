package modelbuilder

import (
	"fmt"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// BuildArguments mirrors BuildRendercvModelArguments
// (rendercv_model_builder.py:20-40). Every field is optional; an empty value
// means "not supplied" and is skipped, which is exactly upstream's truthiness
// rule for render-command overrides (spec §3.20, §5.17).
type BuildArguments struct {
	SettingsYaml string
	DesignYaml   string
	LocaleYaml   string

	OutputFolder         string
	TypstPath            string
	PdfPath              string
	MarkdownPath         string
	HtmlPath             string
	PngPath              string
	DontGenerateTypst    bool
	DontGenerateHtml     bool
	DontGenerateMarkdown bool
	DontGeneratePdf      bool
	DontGeneratePng      bool

	Overrides map[string]string
}

// BuildResult carries the merged document and, per spec §3.18, each overlay's
// own parsed document, so error coordinates can later be resolved against the
// file a value actually came from.
type BuildResult struct {
	Document       *yamldoc.Node
	OverlaySources map[schemaerr.OverlayKey]*yamldoc.Node
}

// overlayOrder is the fixed application order of spec §3.15
// (rendercv_model_builder.py:120-124).
var overlayOrder = []schemaerr.OverlayKey{
	schemaerr.OverlaySettings,
	schemaerr.OverlayDesign,
	schemaerr.OverlayLocale,
}

// BuildDictionary mirrors build_rendercv_dictionary
// (rendercv_model_builder.py:104-157).
func BuildDictionary(mainYaml string, args BuildArguments) (*BuildResult, error) {
	document, err := ReadYamlWithValidationErrors(mainYaml, schemaerr.SourceMain)
	if err != nil {
		return nil, err
	}

	// Spec §3.14: settings and settings.render_command are force-created as
	// empty mappings before anything else touches the document.
	settings := setDefaultMapping(document, "settings")
	renderCommand := setDefaultMapping(settings, "render_command")

	overlaySources := map[schemaerr.OverlayKey]*yamldoc.Node{}
	for _, key := range overlayOrder {
		content := overlayContent(args, key)
		if content == "" {
			continue
		}

		overlay, err := ReadYamlWithValidationErrors(content, schemaerr.OverlayToSource[key])
		if err != nil {
			return nil, err
		}

		// Spec §3.16: only the overlay's own top-level key is taken, and
		// spec §3.17: the assignment replaces, it does not merge.
		value, ok := mappingGet(overlay, string(key))
		if !ok {
			// Spec §5.18: upstream fails here with a raw key lookup rather
			// than a validation error.
			return nil, &schemaerr.InternalError{
				Message: fmt.Sprintf("The %s overlay does not have a `%s` key.", key, key),
			}
		}
		mappingSet(document, string(key), value)
		overlaySources[key] = overlay
	}

	for _, override := range renderCommandOverrides(args) {
		// Spec §3.20: only truthy values are written.
		if override.value == nil {
			continue
		}
		mappingSet(renderCommand, override.key, override.value)
	}

	// Spec §3.21: dotted-key overrides are applied last. Their semantics are
	// iteration 12's; only the ordering is fixed here.
	document = applyOverrides(document, args.Overrides)

	return &BuildResult{Document: document, OverlaySources: overlaySources}, nil
}

// applyOverrides is the ordering hook of spec §3.21.
//
// TODO(iteration-12): implement dotted-key override semantics
// (schema/overrides.py, applied at rendercv_model_builder.py:153-155).
func applyOverrides(document *yamldoc.Node, overrides map[string]string) *yamldoc.Node {
	_ = overrides
	return document
}

func overlayContent(args BuildArguments, key schemaerr.OverlayKey) string {
	switch key {
	case schemaerr.OverlaySettings:
		return args.SettingsYaml
	case schemaerr.OverlayDesign:
		return args.DesignYaml
	case schemaerr.OverlayLocale:
		return args.LocaleYaml
	default:
		return ""
	}
}

type renderOverride struct {
	key   string
	value *yamldoc.Node
}

// renderCommandOverrides lists the eleven keys of spec §3.19 in upstream's
// order (rendercv_model_builder.py:135-151), with falsy values already dropped
// per spec §3.20.
func renderCommandOverrides(args BuildArguments) []renderOverride {
	return []renderOverride{
		{key: "output_folder", value: stringOverride(args.OutputFolder)},
		{key: "typst_path", value: stringOverride(args.TypstPath)},
		{key: "pdf_path", value: stringOverride(args.PdfPath)},
		{key: "markdown_path", value: stringOverride(args.MarkdownPath)},
		{key: "html_path", value: stringOverride(args.HtmlPath)},
		{key: "png_path", value: stringOverride(args.PngPath)},
		{key: "dont_generate_typst", value: boolOverride(args.DontGenerateTypst)},
		{key: "dont_generate_html", value: boolOverride(args.DontGenerateHtml)},
		{key: "dont_generate_markdown", value: boolOverride(args.DontGenerateMarkdown)},
		{key: "dont_generate_pdf", value: boolOverride(args.DontGeneratePdf)},
		{key: "dont_generate_png", value: boolOverride(args.DontGeneratePng)},
	}
}

func stringOverride(value string) *yamldoc.Node {
	if value == "" {
		return nil
	}
	return &yamldoc.Node{Kind: yamldoc.KindString, Raw: value}
}

func boolOverride(value bool) *yamldoc.Node {
	if !value {
		return nil
	}
	return &yamldoc.Node{Kind: yamldoc.KindBool, Raw: "true"}
}

// setDefaultMapping mirrors dict.setdefault(key, {}) on a mapping node: it
// returns the existing value when the key is present and otherwise appends a
// fresh empty mapping, preserving key order.
func setDefaultMapping(node *yamldoc.Node, key string) *yamldoc.Node {
	if existing, ok := mappingGet(node, key); ok {
		return existing
	}
	value := &yamldoc.Node{Kind: yamldoc.KindMapping}
	mappingSet(node, key, value)
	return value
}

func mappingGet(node *yamldoc.Node, key string) (*yamldoc.Node, bool) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return nil, false
	}
	for _, item := range node.Items {
		if item.Key == key {
			return item.Value, true
		}
	}
	return nil, false
}

// mappingSet replaces the value of an existing key in place, keeping its
// position, and appends the key otherwise.
func mappingSet(node *yamldoc.Node, key string, value *yamldoc.Node) {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return
	}
	for i := range node.Items {
		if node.Items[i].Key == key {
			node.Items[i].Value = value
			return
		}
	}
	node.Items = append(node.Items, yamldoc.Item{Key: key, Value: value})
}
