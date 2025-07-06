package locale

import "github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"

// Resolve reads a `locale` block into the catalog the renderer uses.
//
// The base is the named language's catalog and the block's own keys are applied
// over it, which is `create_variant_pydantic_model`'s shape for the locale
// models (spec 007 §1). An unknown or absent language resolves to English —
// validation has already rejected an unknown one, so the fallback is only ever
// reached by a caller that skipped it.
//
// **A locale variant supplies every field** (`require_all_fields=True`), so this
// is an override of a complete catalog rather than a merge of two partial ones,
// and the difference matters for the two month lists: a variant that supplied
// eleven months would leave the twelfth as English's rather than dropping it,
// which is not something a list assignment can express.
func Resolve(node *yamldoc.Node) Catalog {
	catalogs := Catalogs()
	catalog, known := catalogs[languageOf(node)]
	if !known {
		catalog = English()
	}
	if node == nil || node.Kind != yamldoc.KindMapping {
		return catalog
	}

	for _, item := range node.Items {
		if item.Value == nil || item.Value.Kind == yamldoc.KindNull {
			continue
		}
		switch item.Key {
		case "last_updated":
			catalog.LastUpdated = item.Value.Raw
		case "month":
			catalog.Month = item.Value.Raw
		case "months":
			catalog.Months = item.Value.Raw
		case "year":
			catalog.Year = item.Value.Raw
		case "years":
			catalog.Years = item.Value.Raw
		case "present":
			catalog.Present = item.Value.Raw
		case "month_abbreviations":
			catalog.MonthAbbreviations = stringsOf(item.Value, catalog.MonthAbbreviations)
		case "month_names":
			catalog.MonthNames = stringsOf(item.Value, catalog.MonthNames)
		case "phrases":
			catalog.DegreeWithArea = phraseOf(item.Value, catalog.DegreeWithArea)
		}
	}
	return catalog
}

// languageOf is the discriminator `locale` is a union on. The default is
// `english`, which is `EnglishLocale.language`'s own default rather than a
// choice made here.
func languageOf(node *yamldoc.Node) string {
	if node == nil || node.Kind != yamldoc.KindMapping {
		return englishLanguage
	}
	for _, item := range node.Items {
		if item.Key == "language" && item.Value != nil && item.Value.Kind != yamldoc.KindNull {
			return item.Value.Raw
		}
	}
	return englishLanguage
}

func stringsOf(node *yamldoc.Node, fallback []string) []string {
	if node.Kind != yamldoc.KindSequence {
		return fallback
	}
	out := make([]string, 0, len(node.Elems))
	for _, elem := range node.Elems {
		if elem == nil {
			continue
		}
		out = append(out, elem.Raw)
	}
	return out
}

// phraseOf reads `phrases`, which has exactly one member — `degree_with_area`
// (english_locale.py:10-18). The nesting is kept because the block is a model of
// its own upstream and gains members there, not here.
func phraseOf(node *yamldoc.Node, fallback string) string {
	if node.Kind != yamldoc.KindMapping {
		return fallback
	}
	for _, item := range node.Items {
		if item.Key == "degree_with_area" && item.Value != nil &&
			item.Value.Kind != yamldoc.KindNull {
			return item.Value.Raw
		}
	}
	return fallback
}
