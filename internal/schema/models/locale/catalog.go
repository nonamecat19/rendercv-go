package locale

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// MonthCount is the exact length both month lists must have
// (english_locale.py:60, :79 — `at.Len(min_length=12, max_length=12)`).
const MonthCount = 12

// catalogFields is EnglishLocale's declaration order (english_locale.py:21-...).
//
// Every field has a default, so none is Required: a locale block naming only its
// language inherits the rest. That is upstream's model; the *variants* are
// stricter, and spec 007 §1 behavior 3 explains why — they are generated with
// `require_all_fields=True`, which constrains the data rather than the schema.
var catalogFields = []binder.Field{
	{Name: "language", Value: binder.ValueString},
	{Name: "last_updated", Value: binder.ValueString},
	{Name: "month", Value: binder.ValueString},
	{Name: "months", Value: binder.ValueString},
	{Name: "year", Value: binder.ValueString},
	{Name: "years", Value: binder.ValueString},
	{Name: "present", Value: binder.ValueString},
	{Name: "phrases"},
	{Name: "month_abbreviations"},
	{Name: "month_names"},
}

// FieldNames returns the catalog's declaration order.
func FieldNames() []string {
	names := make([]string, 0, len(catalogFields))
	for _, field := range catalogFields {
		names = append(names, field.Name)
	}
	return names
}

// Catalog is EnglishLocale (english_locale.py:21-...), the model every locale
// variant is generated from.
type Catalog struct {
	Language           string
	LastUpdated        string
	Month              string
	Months             string
	Year               string
	Years              string
	Present            string
	DegreeWithArea     string
	MonthAbbreviations []string
	MonthNames         []string
}

// English is the base catalog, whose values are Python defaults rather than a
// YAML file (spec 007 §2 behaviors 5 and 7).
//
// **The abbreviations are not truncations.** They come from Yale's cataloguing
// table, which `english_locale.py:59` cites in a comment, so `June`, `July` and
// `Sept` are four letters rather than three. A port that generated this list by
// slicing would get three of twelve wrong and would look right in review.
func English() Catalog {
	return Catalog{
		Language:       "english",
		LastUpdated:    "Last updated in",
		Month:          "month",
		Months:         "months",
		Year:           "year",
		Years:          "years",
		Present:        "present",
		DegreeWithArea: "DEGREE in AREA",
		MonthAbbreviations: []string{
			"Jan", "Feb", "Mar", "Apr", "May", "June",
			"July", "Aug", "Sept", "Oct", "Nov", "Dec",
		},
		MonthNames: []string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
	}
}

// ValidateCatalog binds a `locale` block. Unknown keys are rejected — every
// locale model is a `BaseModelWithoutExtraKeys` (spec 007 §3 behavior 8).
//
// The language discriminator itself is ValidateLanguage's, which runs first and
// stands alone when it fails.
func ValidateCatalog(
	node *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	result, errs := binder.Bind(
		node,
		binder.Spec{Fields: catalogFields, Policy: binder.ForbidExtra, Model: "EnglishLocale"},
		location,
		source,
	)

	for _, field := range []string{"month_abbreviations", "month_names"} {
		value, present := result.Value(field)
		if !present || value == nil || value.Kind == yamldoc.KindNull {
			continue
		}
		errs = append(errs, validateMonthList(value, locationOf(location, field), source)...)
	}

	return append(errs, result.ExtraErrors...)
}

func locationOf(location []string, field string) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, field)
}
