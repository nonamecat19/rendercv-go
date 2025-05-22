package jsonschema

import "testing"

// Spec 005 §3.2 behavior 9 and the title rule.
//
// Every `want` is a property schema lifted from upstream's `schema.json`, so
// these are measured rather than derived from the helper's own behavior.
func TestPropertySchema(t *testing.T) {
	tests := []struct {
		name     string
		property Property
		want     string
	}{
		{
			name:     "a required scalar",
			property: Property{Name: "bullet", Type: "string", Examples: []any{"Python, JavaScript, C++", "Excellent communication skills"}},
			want: `{
  "examples": [
    "Python, JavaScript, C++",
    "Excellent communication skills"
  ],
  "title": "Bullet",
  "type": "string"
}`,
		},
		{
			// An optional scalar gains the null arm and the null default, and
			// keeps its title.
			name:     "an optional scalar",
			property: Property{Name: "degree", Type: "string", Optional: true, Examples: []any{"BS", "BA", "PhD", "MS"}},
			want: `{
  "anyOf": [
    {
      "type": "string"
    },
    {
      "type": "null"
    }
  ],
  "default": null,
  "examples": [
    "BS",
    "BA",
    "PhD",
    "MS"
  ],
  "title": "Degree"
}`,
		},
		{
			// The title rule: exactly one $ref plus null, so no title. `date`
			// and `start_date` are the two entry fields shaped this way, and
			// neither carries one upstream.
			name:     "an optional reference has no title",
			property: Property{Name: "start_date", Ref: "ExactDate", Optional: true, Description: "The start date in YYYY-MM-DD, YYYY-MM, or YYYY format.", Examples: []any{"2020-09-24", "2020-09", "2020"}},
			want: `{
  "anyOf": [
    {
      "$ref": "#/$defs/ExactDate"
    },
    {
      "type": "null"
    }
  ],
  "default": null,
  "description": "The start date in YYYY-MM-DD, YYYY-MM, or YYYY format.",
  "examples": [
    "2020-09-24",
    "2020-09",
    "2020"
  ]
}`,
		},
		{
			// A third arm, so the title comes back. This is the row that shows
			// the rule is about the schema's shape and not about the field.
			name: "a reference with a third arm keeps its title",
			property: Property{
				Name:     "end_date",
				Arms:     []any{Ref("ExactDate"), NewObject().Set("const", "present").Set("type", "string")},
				Optional: true,
				Examples: []any{"2024-05-20"},
			},
			want: `{
  "anyOf": [
    {
      "$ref": "#/$defs/ExactDate"
    },
    {
      "const": "present",
      "type": "string"
    },
    {
      "type": "null"
    }
  ],
  "default": null,
  "examples": [
    "2024-05-20"
  ],
  "title": "End Date"
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Marshal(test.property.Schema())
			if err != nil {
				t.Fatalf("Marshal = %v", err)
			}
			if got != test.want {
				t.Errorf("=\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}

// Pydantic's default title generator: underscores to spaces, each word
// capitalized.
func TestTitleFor(t *testing.T) {
	for field, want := range map[string]string{
		"bullet":          "Bullet",
		"reversed_number": "Reversed Number",
		"end_date":        "End Date",
		"social_networks": "Social Networks",
		"doi":             "Doi",
	} {
		if got := TitleFor(field); got != want {
			t.Errorf("TitleFor(%q) = %q, want %q", field, got, want)
		}
	}
}

// The model envelope: `description` is a present null rather than an omission,
// and `additionalProperties` is explicit in both directions.
func TestModelEnvelope(t *testing.T) {
	got, err := Marshal(Model("BulletEntry", true, []Property{
		{Name: "bullet", Type: "string", Examples: []any{"Python, JavaScript, C++", "Excellent communication skills"}},
	}))
	if err != nil {
		t.Fatalf("Marshal = %v", err)
	}

	// Upstream's `$defs.BulletEntry`, verbatim.
	const want = `{
  "additionalProperties": true,
  "description": null,
  "properties": {
    "bullet": {
      "examples": [
        "Python, JavaScript, C++",
        "Excellent communication skills"
      ],
      "title": "Bullet",
      "type": "string"
    }
  },
  "required": [
    "bullet"
  ],
  "title": "BulletEntry",
  "type": "object"
}`
	if got != want {
		t.Errorf("=\n%s\nwant\n%s", got, want)
	}
}
