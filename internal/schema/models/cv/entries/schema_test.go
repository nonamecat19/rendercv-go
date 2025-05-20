package entries_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv/entries"
)

// Spec 005 §8. The two date aliases differ only in arm order, and that order is
// upstream's: `ArbitraryDate` is `int | str` and `ExactDate` is `str | int`.
//
// It is the same asymmetry spec 004 §3.9b made observable in error messages —
// `date` surviving dedup with the integer message and `start_date` with the
// string one. Two contracts now depend on it, which is the argument against
// ever "tidying" either.
func TestDateSchemas(t *testing.T) {
	tests := []struct {
		name   string
		schema *jsonschema.Object
		want   string
	}{
		{
			name:   "ArbitraryDate is integer then string",
			schema: entries.DateSchema(),
			want: "{\n  \"anyOf\": [\n    {\n      \"type\": \"integer\"\n    }," +
				"\n    {\n      \"type\": \"string\"\n    }\n  ]\n}",
		},
		{
			name:   "ExactDate is string then integer",
			schema: entries.ExactDateSchema(),
			want: "{\n  \"anyOf\": [\n    {\n      \"type\": \"string\"\n    }," +
				"\n    {\n      \"type\": \"integer\"\n    }\n  ]\n}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := jsonschema.Marshal(test.schema)
			if err != nil {
				t.Fatalf("Marshal = %v", err)
			}
			if got != test.want {
				t.Errorf("=\n%s\nwant\n%s", got, test.want)
			}
		})
	}
}
