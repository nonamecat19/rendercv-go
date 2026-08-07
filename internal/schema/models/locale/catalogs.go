package locale

// Catalogs returns every locale catalog, keyed by language.
//
// The data is Go source diffed against the submodule, matching the error
// dictionary of spec 004 §3.4 — the override files live in `third_party`, which
// is not present at runtime, so their content must be copied and something must
// check the copy.
//
// **`require_all_fields=True` makes that check strict for free** (spec 007 §1
// behavior 3): a locale variant must supply every field rather than inheriting
// defaults, so a field missing from the Go data is a difference the diff sees
// rather than a silent fallback to English.
//
// TODO(spec 007 T4-T25): the twenty-two catalogs, one commit each. English is
// here because its values are Python defaults rather than a YAML file; the
// twenty-one others follow, which is what keeps catalogs_conformance_test.go
// red until they do.
func Catalogs() map[string]Catalog {
	return map[string]Catalog{
		"english": English(),
	}
}
