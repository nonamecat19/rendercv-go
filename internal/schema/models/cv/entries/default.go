package entries

// Default is the registry section discrimination runs against: the eight entry
// models in the order of the `EntryModel` union
// (third_party/rendercv/src/rendercv/schema/models/cv/section.py:24-33).
//
// The union order is the discrimination order, because
// `get_entry_type_name_and_section_model` iterates `characteristic_entry_fields`
// and takes the first type whose characteristic fields intersect the entry's keys
// (`section.py:148-154`). It is deliberately **not** the import order at
// `section.py:11-18`, which is alphabetical by module filename — porting that
// order instead would change which type an ambiguous entry resolves to.
//
// The descriptors are listed literally rather than collected through per-file
// `init()` registration: `init()` runs in filename order within a package, which
// is exactly the alphabetical order this must not be (plan §8).
func Default() *Registry {
	return NewRegistry(
		OneLineDescriptor(),
		NormalDescriptor(),
		ExperienceDescriptor(),
		EducationDescriptor(),
		PublicationDescriptor(),
		BulletDescriptor(),
		NumberedDescriptor(),
		ReversedNumberedDescriptor(),
	)
}
