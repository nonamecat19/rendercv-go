package yamlreader_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func TestResolveScalar(t *testing.T) {
	tests := []struct {
		raw   string
		style yamldoc.ScalarStyle
		want  yamldoc.Kind
	}{
		{"", yamldoc.StylePlain, yamldoc.KindNull},
		{"~", yamldoc.StylePlain, yamldoc.KindNull},
		{"null", yamldoc.StylePlain, yamldoc.KindNull},
		{"Null", yamldoc.StylePlain, yamldoc.KindNull},
		{"NULL", yamldoc.StylePlain, yamldoc.KindNull},
		{"true", yamldoc.StylePlain, yamldoc.KindBool},
		{"True", yamldoc.StylePlain, yamldoc.KindBool},
		{"TRUE", yamldoc.StylePlain, yamldoc.KindBool},
		{"false", yamldoc.StylePlain, yamldoc.KindBool},
		{"False", yamldoc.StylePlain, yamldoc.KindBool},
		{"FALSE", yamldoc.StylePlain, yamldoc.KindBool},
		{".inf", yamldoc.StylePlain, yamldoc.KindFloat},
		{".Inf", yamldoc.StylePlain, yamldoc.KindFloat},
		{".INF", yamldoc.StylePlain, yamldoc.KindFloat},
		{"+.inf", yamldoc.StylePlain, yamldoc.KindFloat},
		{"-.inf", yamldoc.StylePlain, yamldoc.KindFloat},
		{".nan", yamldoc.StylePlain, yamldoc.KindFloat},
		{".NaN", yamldoc.StylePlain, yamldoc.KindFloat},
		{".NAN", yamldoc.StylePlain, yamldoc.KindFloat},
		{"0", yamldoc.StylePlain, yamldoc.KindInt},
		{"42", yamldoc.StylePlain, yamldoc.KindInt},
		{"-7", yamldoc.StylePlain, yamldoc.KindInt},
		{"+7", yamldoc.StylePlain, yamldoc.KindInt},
		{"0x1F", yamldoc.StylePlain, yamldoc.KindInt},
		{"0o17", yamldoc.StylePlain, yamldoc.KindInt},
		{"1_000", yamldoc.StylePlain, yamldoc.KindInt},
		{"00123", yamldoc.StylePlain, yamldoc.KindInt},
		{"3.14", yamldoc.StylePlain, yamldoc.KindFloat},
		{"1e10", yamldoc.StylePlain, yamldoc.KindFloat},
		{"2020", yamldoc.StylePlain, yamldoc.KindInt},
		{"2020-09", yamldoc.StylePlain, yamldoc.KindString},
		{"2020-09-24", yamldoc.StylePlain, yamldoc.KindString},
		{"2020-09-24T10:00:00Z", yamldoc.StylePlain, yamldoc.KindString},
		{"hello", yamldoc.StylePlain, yamldoc.KindString},
		{"yes", yamldoc.StylePlain, yamldoc.KindString},
		{"no", yamldoc.StylePlain, yamldoc.KindString},
		{"on", yamldoc.StylePlain, yamldoc.KindString},
		{"off", yamldoc.StylePlain, yamldoc.KindString},
		{"null", yamldoc.StyleDoubleQuoted, yamldoc.KindString},
		{"true", yamldoc.StyleSingleQuoted, yamldoc.KindString},
		{"2020", yamldoc.StyleDoubleQuoted, yamldoc.KindString},
	}

	for _, tt := range tests {
		t.Run(tt.raw+"_"+styleName(tt.style), func(t *testing.T) {
			got := yamlreader.ResolveScalar(tt.raw, tt.style)
			if got != tt.want {
				t.Errorf("ResolveScalar(%q, %v) = %v, want %v", tt.raw, tt.style, got, tt.want)
			}
		})
	}
}

func styleName(s yamldoc.ScalarStyle) string {
	switch s {
	case yamldoc.StylePlain:
		return "plain"
	case yamldoc.StyleSingleQuoted:
		return "single"
	case yamldoc.StyleDoubleQuoted:
		return "double"
	default:
		return "other"
	}
}
