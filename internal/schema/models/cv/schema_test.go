package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/jsonschema"
	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
)

// The leaf `$defs`, against upstream's bytes. The per-def differential covers
// these too; these assertions exist so a failure names the model rather than
// arriving as one row of an eighteen-row diff.
func TestLeafSchemas(t *testing.T) {
	tests := []struct {
		name   string
		schema *jsonschema.Object
		want   string
	}{
		{
			name:   "TypstDimension",
			schema: cv.TypstDimensionSchema(),
			want:   "{\n  \"type\": \"string\"\n}",
		},
		{
			// `format: path` is pydantic's annotation for a pathlib.Path.
			name:   "ExistingPathRelativeToInput",
			schema: cv.ExistingPathRelativeToInputSchema(),
			want:   "{\n  \"format\": \"path\",\n  \"type\": \"string\"\n}",
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

// The enum members come from the validator's own list, so the schema and the
// error message of spec 004 §4.23 cannot disagree about the order.
func TestSocialNetworkNameSchemaFollowsTheValidator(t *testing.T) {
	value, ok := cv.SocialNetworkNameSchema().Get("enum")
	if !ok {
		t.Fatal("no enum key")
	}
	members, ok := value.([]any)
	if !ok {
		t.Fatalf("enum = %T, want a list", value)
	}
	if len(members) != len(cv.SocialNetworkNames) {
		t.Fatalf("enum has %d members, the validator has %d",
			len(members), len(cv.SocialNetworkNames))
	}
	for i, name := range cv.SocialNetworkNames {
		if members[i] != string(name) {
			t.Errorf("member %d = %v, want %q", i, members[i], name)
		}
	}
	// The first and last are upstream's, so a reversal fails here too.
	if members[0] != "LinkedIn" || members[len(members)-1] != "Reddit" {
		t.Errorf("enum runs %v … %v, want LinkedIn … Reddit", members[0], members[len(members)-1])
	}
}
