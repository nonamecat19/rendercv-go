package schemaerr_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// The Input Value column carries `str()` of the parsed Python object, and for a
// float that is Python's shortest-round-trip `repr` — which no single
// strconv.FormatFloat call produces. Every row below was measured through the
// vendored CPython: the token on the left, `str(float(token))` on the right.
//
// The rows that matter most are the form switches. `1e15` stays decimal and
// `1e16` does not; `1e-4` stays decimal and `1e-5` does not; an overflow is
// `inf` and not the token; and an integral value keeps the `.0` Go's shortest
// formatting drops.
func TestAFloatRendersAsPythonReprsIt(t *testing.T) {
	tests := []struct{ raw, want string }{
		{raw: "1.5", want: "1.5"},
		{raw: "1.0", want: "1.0"},
		{raw: ".5", want: "0.5"},
		{raw: "0.0001", want: "0.0001"},
		{raw: "1e-5", want: "1e-05"},
		{raw: "1e-4", want: "0.0001"},
		{raw: "1e15", want: "1000000000000000.0"},
		{raw: "1e16", want: "1e+16"},
		{raw: "1e308", want: "1e+308"},
		{raw: "1e400", want: "inf"},
		{raw: "-1e400", want: "-inf"},
		{raw: ".inf", want: "inf"},
		{raw: "-.inf", want: "-inf"},
		{raw: "+.inf", want: "inf"},
		{raw: ".NaN", want: "nan"},
		{raw: "1_000.5", want: "1000.5"},
		{raw: "1.50", want: "1.5"},
		{raw: "-0.0", want: "-0.0"},
		{raw: "3.141592653589793", want: "3.141592653589793"},
		{raw: "2.5e-10", want: "2.5e-10"},
		{raw: "1e100", want: "1e+100"},
		{raw: "0.1", want: "0.1"},
		{raw: "1234567890123456.0", want: "1234567890123456.0"},
		{raw: "12345678901234567.0", want: "1.2345678901234568e+16"},
		{raw: "6.02e23", want: "6.02e+23"},
		{raw: "1E5", want: "100000.0"},
		{raw: "-4.9e-324", want: "-5e-324"},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			node := &yamldoc.Node{Kind: yamldoc.KindFloat, Raw: test.raw}
			if got := schemaerr.RenderInput(node); got != test.want {
				t.Errorf("RenderInput(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

// A token the parser cannot read keeps its text rather than becoming a wrong
// number — the same fallback the integer half has.
func TestAnUnreadableFloatKeepsItsText(t *testing.T) {
	node := &yamldoc.Node{Kind: yamldoc.KindFloat, Raw: "not a float"}
	if got := schemaerr.RenderInput(node); got != "not a float" {
		t.Errorf("RenderInput = %q, want the raw text", got)
	}
}
