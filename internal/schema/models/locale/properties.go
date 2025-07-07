package locale

// isoCodes is `language_iso_639_1` (english_locale.py:99-136), the table Typst's
// `text(lang: …)` and the HTML document's `lang` attribute are set from.
//
// **Twenty-three keys for twenty-two languages.** `indonesia` sits beside
// `indonesian` upstream, both mapping to `id`, and no language is named
// `indonesia` — so one row is unreachable. It is copied rather than corrected:
// this table is transcribed, and a transcription that fixes what it copies stops
// being checkable against the original.
var isoCodes = map[string]string{
	"danish":            "da",
	"dutch":             "nl",
	"english":           "en",
	"french":            "fr",
	"german":            "de",
	"hindi":             "hi",
	"italian":           "it",
	"indonesian":        "id",
	"japanese":          "ja",
	"korean":            "ko",
	"indonesia":         "id",
	"mandarin_chinese":  "zh",
	"norwegian_bokmål":  "nb",
	"norwegian_nynorsk": "nn",
	"portuguese":        "pt",
	"russian":           "ru",
	"spanish":           "es",
	"turkish":           "tr",
	"arabic":            "ar",
	"hebrew":            "he",
	"persian":           "fa",
	"vietnamese":        "vi",
	"hungarian":         "hu",
}

// rtlLanguages is `is_rtl`'s set (`:138-146`).
var rtlLanguages = map[string]struct{}{
	"arabic":  {},
	"hebrew":  {},
	"persian": {},
}

// ISOCode is the catalog's ISO 639-1 code, which the Typst preamble and the HTML
// document both carry.
//
// Upstream indexes the table directly and raises `KeyError` for a language it
// has no code for; every member of the union has one, so the miss is
// unreachable. The port returns `en` there rather than reporting, because a
// renderer that fails on a valid document is worse than one that hyphenates
// wrongly.
func (c Catalog) ISOCode() string {
	if code, known := isoCodes[c.Language]; known {
		return code
	}
	return "en"
}

// IsRTL reports whether the language is written right to left, which flips the
// whole document's text direction in the preamble.
func (c Catalog) IsRTL() bool {
	_, rtl := rtlLanguages[c.Language]
	return rtl
}
