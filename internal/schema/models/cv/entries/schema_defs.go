package entries

import "github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"

// entriesModule is the Python package the five colliding entry classes live in
// (spec 005 §3.3 behavior 11).
//
// Five of the nine entry types need the qualified `$defs` name and four do not.
// Which is which is upstream's, not a choice: a class name is qualified when it
// collides somewhere in the tree, and the differential is what checks the answer.
const entriesModule = "rendercv.schema.models.cv.entries"

// The property metadata the two bases contribute, so it is written once rather
// than once per entry type — which is also how upstream gets it
// (entry_with_date.py:42-50, entry_with_complex_fields.py:93-132).
func dateProperty() jsonschema.Property {
	return jsonschema.Property{
		Name: "date", Ref: "ArbitraryDate", Optional: true,
		Description: "The date of this event in YYYY-MM-DD, YYYY-MM, or YYYY format," +
			" or any custom text like 'Fall 2023'. Use this for single-day or" +
			" imprecise dates. For date ranges, use `start_date` and `end_date` instead.",
		Examples: []any{"2020-09-24", "2020-09", "2020", "Fall 2023", "Summer 2020"},
	}
}

// complexProperties is the five fields of BaseEntryWithComplexFields, in
// declaration order after the inherited `date` (spec 003 §3.79).
func complexProperties() []jsonschema.Property {
	return []jsonschema.Property{
		{
			Name: "start_date", Ref: "ExactDate", Optional: true,
			Description: "The start date in YYYY-MM-DD, YYYY-MM, or YYYY format.",
			Examples:    []any{"2020-09-24", "2020-09", "2020"},
		},
		{
			Name: "end_date", Optional: true,
			// The third arm is what gives this field a title where `date` and
			// `start_date` have none (jsonschema.Property.title).
			Arms: []any{
				jsonschema.Ref("ExactDate"),
				jsonschema.NewObject().Set("const", "present").Set("type", "string"),
			},
			Description: `The end date in YYYY-MM-DD, YYYY-MM, or YYYY format. Use "present" for` +
				" ongoing events, or omit it to indicate the event is ongoing.",
			Examples: []any{"2024-05-20", "2024-05", "2024", "present"},
		},
		{
			Name: "location", Type: "string", Optional: true,
			Examples: []any{"Istanbul, Türkiye", "New York, NY", "Remote"},
		},
		{
			Name: "summary", Type: "string", Optional: true,
			Examples: []any{
				"Led a team of 5 engineers to develop innovative solutions.",
				"Completed advanced coursework in machine learning and artificial intelligence.",
			},
		},
		{
			Name: "highlights", Optional: true,
			Arms: []any{jsonschema.NewObject().
				Set("items", jsonschema.NewObject().Set("type", "string")).
				Set("type", "array")},
			Description: "Bullet points for key achievements, responsibilities, or contributions.",
			Examples: []any{[]any{
				"Increased system performance by 40% through optimization.",
				"Mentored 3 junior developers and conducted code reviews.",
				"Implemented CI/CD pipeline reducing deployment time by 60%.",
			}},
		},
	}
}

// withComplexFields appends the inherited `date` and the five complex fields to
// an entry's own, which is the field order of spec 003 §3.2 — own fields first.
func withComplexFields(own ...jsonschema.Property) []jsonschema.Property {
	all := append([]jsonschema.Property(nil), own...)
	all = append(all, dateProperty())
	return append(all, complexProperties()...)
}

// SchemaDefs returns every `$defs` entry the entry models own, keyed by its
// upstream name.
//
// `TextEntry` is absent, and that is upstream's doing rather than an omission:
// it is `str`, not a model, so it has no entry at all (spec 003 §3.1).
func SchemaDefs() map[string]*jsonschema.Object {
	return map[string]*jsonschema.Object{
		"ArbitraryDate": DateSchema(),
		"ListOfEntries": ListOfEntriesSchema(),
		"ExactDate":     ExactDateSchema(),

		// The four whose class names are unique.
		"BulletEntry": jsonschema.EntryModel("BulletEntry", []jsonschema.Property{{
			Name: "bullet", Type: "string",
			Examples: []any{"Python, JavaScript, C++", "Excellent communication skills"},
		}}),
		"NumberedEntry": jsonschema.EntryModel("NumberedEntry", []jsonschema.Property{{
			Name: "number", Type: "string",
			Examples: []any{"First publication about XYZ", "Patent for ABC technology"},
		}}),
		"ReversedNumberedEntry": jsonschema.EntryModel("ReversedNumberedEntry", []jsonschema.Property{{
			Name: "reversed_number", Type: "string",
			Description: "Reverse-numbered list item. Numbering goes in reverse" +
				" (5, 4, 3, 2, 1), making recent items have higher numbers.",
			Examples: []any{"Latest research paper", "Recent patent application"},
		}}),

		// The five that collide and are qualified.
		jsonschema.DefName("OneLineEntry", entriesModule+".one_line"): jsonschema.EntryModel(
			"OneLineEntry", []jsonschema.Property{
				{Name: "label", Type: "string", Examples: []any{"Languages", "Citizenship", "Security Clearance"}},
				{Name: "details", Type: "string", Examples: []any{
					"English (native), Spanish (fluent)", "US Citizen", "Top Secret",
				}},
			}),
		jsonschema.DefName("NormalEntry", entriesModule+".normal"): jsonschema.EntryModel(
			"NormalEntry", withComplexFields(
				jsonschema.Property{Name: "name", Type: "string", Examples: []any{
					"Some Project", "Some Event", "Some Award",
				}},
			)),
		jsonschema.DefName("ExperienceEntry", entriesModule+".experience"): jsonschema.EntryModel(
			"ExperienceEntry", withComplexFields(
				jsonschema.Property{Name: "company", Type: "string", Examples: []any{
					"Microsoft", "Google", "Princeton Plasma Physics Laboratory",
				}},
				jsonschema.Property{Name: "position", Type: "string", Examples: []any{
					"Software Engineer", "Research Assistant", "Project Manager",
				}},
			)),
		jsonschema.DefName("EducationEntry", entriesModule+".education"): jsonschema.EntryModel(
			"EducationEntry", withComplexFields(
				jsonschema.Property{Name: "institution", Type: "string", Examples: []any{
					"Boğaziçi University", "MIT", "Harvard University",
				}},
				jsonschema.Property{
					Name: "area", Type: "string",
					Description: "Field of study or major.",
					Examples: []any{
						"Mechanical Engineering", "Computer Science", "Electrical Engineering",
					},
				},
				jsonschema.Property{
					Name: "degree", Type: "string", Optional: true,
					Examples: []any{"BS", "BA", "PhD", "MS"},
				},
			)),
		jsonschema.DefName("PublicationEntry", entriesModule+".publication"): publicationSchema(),
	}
}

// publicationSchema is its own function because PublicationEntry does not build
// on the complex-field base: it takes `date` and nothing else, and its `url`
// carries pydantic's HttpUrl constraints (spec 003 §3.10).
func publicationSchema() *jsonschema.Object {
	return jsonschema.EntryModel("PublicationEntry", []jsonschema.Property{
		{Name: "title", Type: "string", Examples: []any{
			"Deep Learning for Computer Vision", "Advances in Quantum Computing",
		}},
		{
			Name: "authors",
			Arms: []any{jsonschema.NewObject().
				Set("items", jsonschema.NewObject().Set("type", "string")).
				Set("type", "array")},
			Description: "You can bold your name with **double asterisks**.",
			Examples:    []any{[]any{"John Doe", "**Jane Smith**", "Bob Johnson"}},
		},
		{Name: "summary", Type: "string", Optional: true, Examples: []any{
			"This paper presents a new method for computer vision.",
		}},
		{
			Name: "doi", Optional: true,
			// The pattern is the one spec 004 §3.4 behavior 13 showed is
			// unreachable by dictionary row 4 — single backslashes here, doubled
			// there.
			Arms: []any{jsonschema.NewObject().
				Set("pattern", `\b10\..*`).
				Set("type", "string")},
			Description: "The DOI (Digital Object Identifier). If provided, it will be" +
				" used as the link instead of the URL.",
			Examples: []any{"10.48550/arXiv.2310.03138"},
		},
		{
			Name: "url", Optional: true,
			// pydantic.HttpUrl's constraints. maxLength is the 2083 of
			// spec 004 §3.13 behavior 46.
			Arms: []any{jsonschema.NewObject().
				Set("format", "uri").
				Set("maxLength", 2083).
				Set("minLength", 1).
				Set("type", "string")},
			Description: "A URL link to the publication. Ignored if DOI is provided.",
		},
		{
			Name: "journal", Type: "string", Optional: true,
			Description: "The journal, conference, or venue where it was published.",
			Examples: []any{
				"Nature", "IEEE Conference on Computer Vision", "arXiv preprint",
			},
		},
		dateProperty(),
	})
}

// ListOfEntriesSchema is `ListOfEntries` (section.py's entry union).
//
// The arm order is the union's declaration order, which is also the order
// section discrimination tries the types in (spec 003 §3.56) — so it is the same
// list the registry carries, and a reordering is observable twice.
//
// The first arm is `array of string`, which is `TextEntry`: the one entry type
// with no `$defs` entry of its own, because it is `str` rather than a model.
func ListOfEntriesSchema() *jsonschema.Object {
	arms := []any{arrayOf(jsonschema.NewObject().Set("type", "string"))}
	for _, name := range []string{
		jsonschema.DefName("OneLineEntry", entriesModule+".one_line"),
		jsonschema.DefName("NormalEntry", entriesModule+".normal"),
		jsonschema.DefName("ExperienceEntry", entriesModule+".experience"),
		jsonschema.DefName("EducationEntry", entriesModule+".education"),
		jsonschema.DefName("PublicationEntry", entriesModule+".publication"),
		"BulletEntry",
		"NumberedEntry",
		"ReversedNumberedEntry",
	} {
		arms = append(arms, arrayOf(jsonschema.Ref(name)))
	}
	return jsonschema.NewObject().Set("anyOf", arms)
}

func arrayOf(items *jsonschema.Object) *jsonschema.Object {
	return jsonschema.NewObject().Set("items", items).Set("type", "array")
}
