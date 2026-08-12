package kindguard_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/kindguard"
)

// testKinds is a deliberately small stand-in kind set, so the table below can
// spell an exhaustive switch in three arms instead of eight. The real set is
// read from the source by Kinds and checked in TestKinds.
var testKinds = []string{"KindMapping", "KindNull", "KindString"}

func TestCheckSource(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []kindguard.Rule
	}{{
		name: "exhaustive switch with no default is the sanctioned shape",
		body: `switch n.Kind {
		case yamldoc.KindMapping, yamldoc.KindNull:
			return true
		case yamldoc.KindString:
			return false
		}
		return false`,
		want: nil,
	}, {
		name: "one comparison is total and stays legal",
		body: `return n.Kind != yamldoc.KindMapping`,
		want: nil,
	}, {
		name: "one comparison anded with an unrelated test stays legal",
		body: `return n != nil && n.Kind == yamldoc.KindNull`,
		want: nil,
	}, {
		name: "a switch missing a kind is caught",
		body: `switch n.Kind {
		case yamldoc.KindMapping:
			return true
		case yamldoc.KindNull:
			return false
		}
		return false`,
		want: []kindguard.Rule{kindguard.RuleSwitchNotExhaustive},
	}, {
		name: "a default clause absorbs a new kind and is caught",
		body: `switch n.Kind {
		case yamldoc.KindMapping, yamldoc.KindNull:
			return true
		case yamldoc.KindString:
			return false
		default:
			return false
		}`,
		want: []kindguard.Rule{kindguard.RuleSwitchHasDefault},
	}, {
		name: "a default clause on a partial switch is caught twice",
		body: `switch n.Kind {
		case yamldoc.KindMapping:
			return true
		default:
			return false
		}`,
		want: []kindguard.Rule{kindguard.RuleSwitchHasDefault, kindguard.RuleSwitchNotExhaustive},
	}, {
		// This is blocker B1's shape verbatim: the predicate that let a
		// tagged scalar render at exit 0.
		name: "an or-chain over two kinds is caught",
		body: `return n.Kind == yamldoc.KindMapping || n.Kind == yamldoc.KindNull`,
		want: []kindguard.Rule{kindguard.RuleMultiConstantPredicate},
	}, {
		name: "an and-chain over two kinds is caught",
		body: `return n.Kind != yamldoc.KindMapping && n.Kind != yamldoc.KindNull`,
		want: []kindguard.Rule{kindguard.RuleMultiConstantPredicate},
	}, {
		name: "a nested or-chain is reported once, at the outermost group",
		body: `return n.Kind == yamldoc.KindMapping ||
			n.Kind == yamldoc.KindNull ||
			n.Kind == yamldoc.KindString`,
		want: []kindguard.Rule{kindguard.RuleMultiConstantPredicate},
	}, {
		name: "a tagless switch enumerating kinds is caught",
		body: `switch {
		case n.Kind == yamldoc.KindMapping:
			return true
		case n.Kind == yamldoc.KindNull:
			return false
		}
		return false`,
		want: []kindguard.Rule{kindguard.RuleMultiConstantPredicate},
	}, {
		name: "a constant from another package's Kind enum is not ours",
		body: `return n.Kind == design.KindMapping || n.Kind == design.KindNull`,
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "package sample\n\nfunc sample(n *yamldoc.Node) bool {\n" + test.body + "\n}\n"
			violations, err := kindguard.CheckSource("sample.go", []byte(source), testKinds)
			if err != nil {
				t.Fatalf("CheckSource: %v", err)
			}
			var got []kindguard.Rule
			for _, violation := range violations {
				got = append(got, violation.Rule)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("rules = %v, want %v\nviolations: %v", got, test.want, violations)
			}
		})
	}
}

// TestKinds pins that the guard reads the real constant set, not a copy that
// can drift from it.
func TestKinds(t *testing.T) {
	kinds, err := kindguard.Kinds(moduleRoot(t))
	if err != nil {
		t.Fatalf("Kinds: %v", err)
	}
	want := []string{
		"KindBool", "KindFloat", "KindInt", "KindMapping",
		"KindNull", "KindSequence", "KindString", "KindTagged",
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("Kinds() = %v, want %v", kinds, want)
	}
}

// TestTreeHasNoKindPredicateOutsideAnExhaustiveSwitch is the constraint itself.
// Everything above proves it can fail.
func TestTreeHasNoKindPredicateOutsideAnExhaustiveSwitch(t *testing.T) {
	violations, err := kindguard.CheckTree(moduleRoot(t))
	if err != nil {
		t.Fatalf("CheckTree: %v", err)
	}
	if len(violations) == 0 {
		return
	}
	lines := make([]string, 0, len(violations))
	for _, violation := range violations {
		lines = append(lines, violation.String())
	}
	t.Fatalf("%d branch(es) over yamldoc.Kind outside the exhaustive-switch shape:\n%s",
		len(violations), strings.Join(lines, "\n"))
}

// moduleRoot walks up from the test's directory to the directory holding
// go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}
