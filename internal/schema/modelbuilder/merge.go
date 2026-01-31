package modelbuilder

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

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

	OutputFolder string
	TypstPath    string
	PdfPath      string
	MarkdownPath string
	HtmlPath     string
	PngPath      string
	// **An explicit false is indistinguishable from an absent key**, and that
	// is upstream's behavior, not a limitation of this type: the override loop
	// is `if value:` (`rendercv_model_builder.py:149`), which drops False along
	// with None and "". Measured — passing dont_generate_pdf=False to
	// build_rendercv_dictionary leaves the key absent from the merged
	// dictionary.
	//
	// So there is no third state to represent. A *bool here would let this port
	// express something upstream cannot, which is a divergence rather than a
	// feature.
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
	setDefaultMapping(settings, "render_command")

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

	// **The mapping has to be looked up again, after the overlays.** Upstream
	// writes `input_dict["settings"]["render_command"][key] = value`
	// (`rendercv_model_builder.py:149-151`) — a fresh subscript every time — and
	// a settings overlay **replaces** the whole `settings` value at `:132`. The
	// port captured `render_command` before that replacement, so with a
	// `--settings` overlay present every render-command override was written
	// into a node no longer attached to the document and silently lost.
	//
	// Measured: `render cv.yaml --settings s.yaml --dont-generate-typst
	// --dont-generate-pdf --dont-generate-png` generated all five formats here
	// and two upstream.
	renderCommand := setDefaultMapping(setDefaultMapping(document, "settings"), "render_command")

	for _, override := range renderCommandOverrides(args) {
		// Spec §3.20: only truthy values are written.
		if override.value == nil {
			continue
		}
		mappingSet(renderCommand, override.key, override.value)
	}

	// Spec §3.21: dotted-key overrides are applied last. Their semantics are
	// iteration 12's; only the ordering is fixed here.
	document, err = applyOverrides(document, args.Overrides)
	if err != nil {
		return nil, err
	}

	return &BuildResult{Document: document, OverlaySources: overlaySources}, nil
}

// applyOverrides applies the CLI's dotted-key overrides
// (override_dictionary.py:91-121, applied at rendercv_model_builder.py:153-155).
//
// **This was a stub returning the document unchanged**, which meant every
// `--cv.phone`, `--design.theme` and `--settings.current_date` was silently
// discarded — four corpus cases could never pass, and the pinned date the
// goldens are now generated with had no effect on the port.
//
// The keys are applied in sorted order rather than map order. Upstream iterates
// a dict, which is insertion-ordered from the CLI; Go's map order is random, and
// two overrides that touch the same path must not resolve differently between
// runs.
func applyOverrides(
	document *yamldoc.Node,
	overrides map[string]string,
) (*yamldoc.Node, error) {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := setByLocation(document, strings.Split(key, "."), overrides[key], key); err != nil {
			return nil, err
		}
	}
	return document, nil
}

// setByLocation is `update_value_by_location` (`:5-89`), walking one dotted path.
//
// **A missing mapping key is created; a missing list index is an error.** That
// asymmetry is upstream's: a dict grows to meet the path, a list does not
// (`:74-76` against `:61-65`).
func setByLocation(node *yamldoc.Node, path []string, value, fullKey string) error {
	if len(path) == 0 {
		return nil
	}
	previous := strings.Join(strings.Split(fullKey, ".")[:len(strings.Split(fullKey, "."))-len(path)], ".")

	// Every kind that is neither a list nor a mapping ends the walk with the
	// same message, and the arms name them one by one rather than leaning on a
	// `default`, so a new kind cannot join that set without a decision
	// (kindguard).
	unwalkable := func() error {
		return &schemaerr.UserError{Message: fmt.Sprintf(
			"It seems like there's something wrong with `%s`, but we don't know what it is.",
			fullKey)}
	}
	if node == nil {
		return unwalkable()
	}

	switch node.Kind {
	case yamldoc.KindSequence:
		index, err := strconv.Atoi(path[0])
		if err != nil {
			return &schemaerr.UserError{Message: fmt.Sprintf(
				"`%s` corresponds to a list, but `%s` is not an integer.", previous, path[0])}
		}
		if index < 0 || index >= len(node.Elems) {
			return &schemaerr.UserError{Message: fmt.Sprintf(
				"Index %d is out of range for the list `%s`.", index, previous)}
		}
		if len(path) == 1 {
			node.Elems[index] = stringOverride(value)
			return nil
		}
		return setByLocation(node.Elems[index], path[1:], value, fullKey)

	case yamldoc.KindMapping:
		if len(path) == 1 {
			setMappingValue(node, path[0], stringOverride(value))
			return nil
		}
		child := setDefaultMapping(node, path[0])
		return setByLocation(child, path[1:], value, fullKey)

	case yamldoc.KindNull, yamldoc.KindBool, yamldoc.KindInt,
		yamldoc.KindFloat, yamldoc.KindString, yamldoc.KindTagged:
		return unwalkable()
	}

	return unwalkable()
}

// setMappingValue replaces a key's value, appending the key when it is absent.
func setMappingValue(node *yamldoc.Node, key string, value *yamldoc.Node) {
	for i := range node.Items {
		if node.Items[i].Key == key {
			node.Items[i].Value = value
			return
		}
	}
	node.Items = append(node.Items, yamldoc.Item{Key: key, Value: value})
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

// boolOverride is upstream's `if value:` for a bool: only true contributes an
// override, and false is dropped exactly as an absent key is
// (`rendercv_model_builder.py:149`).
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
