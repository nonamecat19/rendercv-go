package cv_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/models/cv"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
)

// Spec 004 §3.13 behavior 41's third site. `url` is required-but-nullable, so
// the three states are distinct: absent is a missing field, an explicit null is
// the declared default, and a value is parsed.
func TestCustomConnectionURL(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "a valid URL is accepted",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: https://example.com\n",
			want: "",
		},
		{
			name: "an explicit null is the default, not a failure",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: null\n",
			want: "",
		},
		{
			name: "a value that does not parse",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: not a url\n",
			want: "Input should be a valid URL",
		},
		{
			name: "the wrong scheme",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: ftp://example.com\n",
			want: "URL scheme should be 'http' or 'https'",
		},
		// HttpUrl names both shapes it takes in its *type* error, and a
		// non-string never reaches the parser: the port ran every one of these
		// through it and reported a parse failure instead — and accepted the
		// tagged one outright, since its text parses as a URL.
		{
			name: "an integer is a type failure, not a parse failure",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: 5\n",
			want: "URL input should be a string or URL",
		},
		{
			name: "a bool is a type failure",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: true\n",
			want: "URL input should be a string or URL",
		},
		{
			name: "a sequence is a type failure",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: []\n",
			want: "URL input should be a string or URL",
		},
		{
			name: "a tagged scalar is a type failure however its text parses",
			src:  "fontawesome_icon: fa\nplaceholder: p\nurl: !!str https://example.com\n",
			want: "URL input should be a string or URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, errs := cv.ValidateCustomConnection(
				parse(t, test.src), []string{"cv", "custom_connections", "0"},
				schemaerr.SourceMain,
			)
			if test.want == "" {
				if len(errs) != 0 {
					t.Fatalf("errs = %+v, want none", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("errs = %+v, want exactly one", errs)
			}
			if errs[0].Message != test.want {
				t.Errorf("message = %q, want %q", errs[0].Message, test.want)
			}
			if last := errs[0].SchemaLocation[len(errs[0].SchemaLocation)-1]; last != "url" {
				t.Errorf("location ends %q, want url", last)
			}
		})
	}
}
