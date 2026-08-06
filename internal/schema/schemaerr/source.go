package schemaerr

type YamlSource string

const (
	SourceMain     YamlSource = "main_yaml_file"
	SourceDesign   YamlSource = "design_yaml_file"
	SourceLocale   YamlSource = "locale_yaml_file"
	SourceSettings YamlSource = "settings_yaml_file"
)

type OverlayKey string

const (
	OverlayDesign   OverlayKey = "design"
	OverlayLocale   OverlayKey = "locale"
	OverlaySettings OverlayKey = "settings"
)

var OverlayToSource = map[OverlayKey]YamlSource{
	OverlayDesign:   SourceDesign,
	OverlayLocale:   SourceLocale,
	OverlaySettings: SourceSettings,
}
