package schemaerr

// YamlSource names the input document a value came from, so an error can be
// reported against the right file.
type YamlSource string

// The four source literals.
const (
	SourceMain     YamlSource = "main_yaml_file"
	SourceDesign   YamlSource = "design_yaml_file"
	SourceLocale   YamlSource = "locale_yaml_file"
	SourceSettings YamlSource = "settings_yaml_file"
)

// OverlayKey is the top-level key an overlay document contributes.
type OverlayKey string

// The three overlay keys.
const (
	OverlayDesign   OverlayKey = "design"
	OverlayLocale   OverlayKey = "locale"
	OverlaySettings OverlayKey = "settings"
)

// OverlayToSource maps an overlay key to the source literal its errors carry.
var OverlayToSource = map[OverlayKey]YamlSource{
	OverlayDesign:   SourceDesign,
	OverlayLocale:   SourceLocale,
	OverlaySettings: SourceSettings,
}
