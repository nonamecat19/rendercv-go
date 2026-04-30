package modelbuilder

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// goccy reports one message — `unexpected directive value. document not
// started`, always at the first directive line, column 1 — for four different
// documents, three of which ruamel rejects differently and the fourth of which
// it renders.
//
// Every row was measured through the vendored `read_yaml`, reading `problem`
// and `problem_mark` off the raised exception.
func TestDirectiveScanReconstructsRuamelsVerdict(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		want     string
		at       yamldoc.Position
		declines bool // the scan must decline
	}{
		{
			name: "no document marker",
			src:  "%TAG !e! tag:x,1:\nk: v\n",
			want: "mapping values are not allowed here",
			at:   yamldoc.Position{Line: 2, Column: 2},
		},
		{
			name: "no document marker after an unrecognised directive",
			src:  "%FOO bar\nk: v\n",
			want: "mapping values are not allowed here",
			at:   yamldoc.Position{Line: 2, Column: 2},
		},
		{
			name: "duplicate YAML directive",
			src:  "%YAML 1.2\n%YAML 1.2\n---\nk: v\n",
			want: "found duplicate YAML directive",
			at:   yamldoc.Position{Line: 2, Column: 1},
		},
		{
			// The versions need not agree for it to be a duplicate.
			name: "duplicate YAML directive, different versions",
			src:  "%YAML 1.2\n%YAML 1.1\n---\nk: v\n",
			want: "found duplicate YAML directive",
			at:   yamldoc.Position{Line: 2, Column: 1},
		},
		{
			name: "duplicate tag handle",
			src:  "%TAG !e! a:\n%TAG !e! b:\n---\nk: v\n",
			want: "duplicate tag handle '!e!'",
			at:   yamldoc.Position{Line: 2, Column: 1},
		},
		{
			name: "duplicate primary tag handle",
			src:  "%TAG !! a:\n%TAG !! b:\n---\nk: v\n",
			want: "duplicate tag handle '!!'",
			at:   yamldoc.Position{Line: 2, Column: 1},
		},
		{
			// A `%YAML` before them does not move the mark off the second
			// `%TAG`: ruamel reports where it stopped, not where the block began.
			name: "duplicate tag handle after a version directive",
			src:  "%YAML 1.2\n%TAG !e! a:\n%TAG !e! b:\n---\nk: v\n",
			want: "duplicate tag handle '!e!'",
			at:   yamldoc.Position{Line: 3, Column: 1},
		},
		{
			// **The document upstream renders.** goccy cannot read more than one
			// directive, ruamel reads three; there is no ruamel failure to
			// reconstruct, so the scan declines and goccy's own line reaches the
			// user rather than a fabricated upstream phrasing.
			name:     "several directives and a marker",
			src:      "%YAML 1.2\n%TAG !e! a:\n%TAG !f! b:\n---\nk: v\n",
			declines: true,
		},
		{
			// No key indicator on the content line: ruamel says `expected
			// '<document start>', but found ('<scalar>',)`, whose found-token
			// spelling is not reconstructible here.
			name:     "no marker and no key indicator",
			src:      "%FOO bar\nfoo\n",
			declines: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failure, ok := directiveScan(test.src)
			if ok == test.declines {
				t.Fatalf("directiveScan ok = %v, want %v", ok, !test.declines)
			}
			if test.declines {
				return
			}
			if failure.message != test.want {
				t.Errorf("message = %q, want %q", failure.message, test.want)
			}
			if failure.at != test.at {
				t.Errorf("at = %+v, want %+v", failure.at, test.at)
			}
		})
	}
}
