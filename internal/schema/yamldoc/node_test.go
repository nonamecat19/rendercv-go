package yamldoc_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

func TestKindValues(t *testing.T) {
	if yamldoc.KindNull != 0 {
		t.Error("KindNull should be 0")
	}
	if yamldoc.KindBool != 1 {
		t.Error("KindBool should be 1")
	}
	if yamldoc.KindInt != 2 {
		t.Error("KindInt should be 2")
	}
	if yamldoc.KindFloat != 3 {
		t.Error("KindFloat should be 3")
	}
	if yamldoc.KindString != 4 {
		t.Error("KindString should be 4")
	}
	if yamldoc.KindMapping != 5 {
		t.Error("KindMapping should be 5")
	}
	if yamldoc.KindSequence != 6 {
		t.Error("KindSequence should be 6")
	}
}

func TestPosition(t *testing.T) {
	p := yamldoc.Position{Line: 1, Column: 5}
	if p.Line != 1 || p.Column != 5 {
		t.Errorf("Position = %+v", p)
	}
}

func TestSpan(t *testing.T) {
	s := yamldoc.Span{
		Start: yamldoc.Position{Line: 1, Column: 1},
		End:   yamldoc.Position{Line: 2, Column: 10},
	}
	if s.Start.Line != 1 || s.End.Line != 2 {
		t.Errorf("Span = %+v", s)
	}
}

func TestNodeConstructors(t *testing.T) {
	n := &yamldoc.Node{
		Kind: yamldoc.KindString,
		Span: yamldoc.Span{
			Start: yamldoc.Position{Line: 1, Column: 1},
			End:   yamldoc.Position{Line: 1, Column: 5},
		},
		Raw:   "hello",
		Style: yamldoc.StylePlain,
	}
	if n.Kind != yamldoc.KindString {
		t.Error("Kind mismatch")
	}
	if n.Raw != "hello" {
		t.Error("Raw mismatch")
	}
}

func TestMappingNode(t *testing.T) {
	n := &yamldoc.Node{
		Kind: yamldoc.KindMapping,
		Items: []yamldoc.Item{
			{
				Key: "name",
				KeySpan: yamldoc.Span{
					Start: yamldoc.Position{Line: 1, Column: 1},
					End:   yamldoc.Position{Line: 1, Column: 10},
				},
				Value: &yamldoc.Node{
					Kind: yamldoc.KindString,
					Raw:  "John",
				},
			},
		},
	}
	if len(n.Items) != 1 {
		t.Error("expected 1 item")
	}
	if n.Items[0].Key != "name" {
		t.Error("key mismatch")
	}
	if n.Items[0].Value.Raw != "John" {
		t.Error("value mismatch")
	}
}

func TestSequenceNode(t *testing.T) {
	n := &yamldoc.Node{
		Kind: yamldoc.KindSequence,
		Elems: []*yamldoc.Node{
			{Kind: yamldoc.KindString, Raw: "a"},
			{Kind: yamldoc.KindString, Raw: "b"},
		},
	}
	if len(n.Elems) != 2 {
		t.Error("expected 2 elements")
	}
}
