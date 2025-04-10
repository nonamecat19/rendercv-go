package entries

// Default is the registry section discrimination runs against: the eight entry
// models in the order of the `EntryModel` union
// (third_party/rendercv/src/rendercv/schema/models/cv/section.py:24-33).
//
// The union order is the discrimination order, because
// `get_entry_type_name_and_section_model` iterates `characteristic_entry_fields`
// and takes the first type whose characteristic fields intersect the entry's keys
// (`section.py:148-154`). It is deliberately **not** the import order at
// `section.py:11-18`, which is alphabetical by module filename.
//
// TODO(iteration-3): T17 fills this in. It is empty on purpose so that T6's
// conformance test lands red against the generated fixture, per tasks 003 T6.
func Default() *Registry {
	return NewRegistry()
}
