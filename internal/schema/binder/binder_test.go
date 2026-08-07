package binder_test

import (
	"testing"

	"github.com/nonamecat19/rendercv-go/internal/schema/binder"
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamlreader"
)

func parse(t *testing.T, src string) *yamldoc.Node {
	t.Helper()
	node, err := yamlreader.ReadString(src)
	if err != nil {
		t.Fatalf("ReadString(%q): %v", src, err)
	}
	return node
}

func cvSpec(policy binder.Policy) binder.Spec {
	return binder.Spec{
		Fields: []binder.Field{
			{Name: "name"},
			{Name: "email"},
			{Name: "location"},
			{Name: "url", Required: true},
		},
		Policy: policy,
	}
}

// Spec §3.32 — the two base kinds: unknown keys rejected, or kept and readable.
func TestExtraKeyPolicies(t *testing.T) {
	src := "name: John\nurl: null\nunknown: 1\n"

	t.Run("forbid", func(t *testing.T) {
		result, errs := bindAll(parse(t, src), cvSpec(binder.ForbidExtra), nil, schemaerr.SourceMain)
		if len(errs) != 1 {
			t.Fatalf("errs = %+v, want exactly one", errs)
		}
		if errs[0].Code != binder.CodeExtraForbidden {
			t.Errorf("code = %q, want %q", errs[0].Code, binder.CodeExtraForbidden)
		}
		if len(result.Extras) != 0 {
			t.Errorf("extras = %+v, want none retained", result.Extras)
		}
	})

	t.Run("allow", func(t *testing.T) {
		result, errs := bindAll(parse(t, src), cvSpec(binder.AllowExtra), nil, schemaerr.SourceMain)
		if len(errs) != 0 {
			t.Fatalf("errs = %+v, want none", errs)
		}
		// Spec §3.67 — the unknown key is retained and readable by name.
		if _, ok := result.Extra("unknown"); !ok {
			t.Errorf("extras = %+v, want `unknown` retained", result.Extras)
		}
	})
}

// Spec §5.15 — a null-valued unknown key is still rejected; the value is never consulted.
func TestNullValuedUnknownKeyIsRejected(t *testing.T) {
	_, errs := bindAll(
		parse(t, "name: John\nurl: null\nunknown: null\n"),
		cvSpec(binder.ForbidExtra), nil, schemaerr.SourceMain,
	)
	if len(errs) != 1 || errs[0].Code != binder.CodeExtraForbidden {
		t.Fatalf("errs = %+v, want one extra_forbidden", errs)
	}
}

// Plan §4 — absent and present-and-null are distinguishable here and nowhere else.
func TestAbsentVersusPresentAndNull(t *testing.T) {
	result, _ := binder.Bind(
		parse(t, "name: John\nurl: null\n"),
		cvSpec(binder.ForbidExtra), nil, schemaerr.SourceMain,
	)

	value, present := result.Value("url")
	if !present {
		t.Fatal("url reported absent, want present")
	}
	if value.Kind != yamldoc.KindNull {
		t.Errorf("url kind = %v, want KindNull", value.Kind)
	}
	if _, present := result.Value("email"); present {
		t.Error("email reported present, want absent")
	}
}

// Spec §3.81 — a required-but-nullable field is satisfied by an explicit null and
// reported missing only when the key is absent.
func TestRequiredButNullable(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantCodes []schemaerr.Code
	}{
		{name: "explicit null satisfies", src: "name: John\nurl: null\n"},
		{name: "value satisfies", src: "name: John\nurl: https://example.com\n"},
		{name: "absent is missing", src: "name: John\n", wantCodes: []schemaerr.Code{binder.CodeMissing}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := bindAll(parse(t, tc.src), cvSpec(binder.ForbidExtra), nil, schemaerr.SourceMain)
			if len(errs) != len(tc.wantCodes) {
				t.Fatalf("errs = %+v, want %d", errs, len(tc.wantCodes))
			}
			for i, code := range tc.wantCodes {
				if errs[i].Code != code {
					t.Errorf("errs[%d].Code = %q, want %q", i, errs[i].Code, code)
				}
			}
		})
	}
}

// Spec §3.50, §5.15 — key order records present, non-null keys in input order.
func TestKeyOrderDropsNulls(t *testing.T) {
	result, _ := binder.Bind(
		parse(t, "name: John\nemail: null\nlocation: Istanbul\nurl: null\n"),
		cvSpec(binder.ForbidExtra), nil, schemaerr.SourceMain,
	)

	want := []string{"name", "location"}
	if len(result.KeyOrder) != len(want) {
		t.Fatalf("key order = %v, want %v", result.KeyOrder, want)
	}
	for i, key := range want {
		if result.KeyOrder[i] != key {
			t.Fatalf("key order = %v, want %v", result.KeyOrder, want)
		}
	}
}

// Spec §5.16 — a non-mapping input records an empty key order.
func TestNonMappingInput(t *testing.T) {
	node := &yamldoc.Node{Kind: yamldoc.KindSequence}
	result, errs := bindAll(node, cvSpec(binder.ForbidExtra), []string{"cv"}, schemaerr.SourceMain)

	if len(result.KeyOrder) != 0 {
		t.Errorf("key order = %v, want empty", result.KeyOrder)
	}
	if len(errs) != 1 || errs[0].Code != binder.CodeModelType {
		t.Fatalf("errs = %+v, want one model_type", errs)
	}
	if len(errs[0].SchemaLocation) != 1 || errs[0].SchemaLocation[0] != "cv" {
		t.Errorf("schema location = %v, want [cv]", errs[0].SchemaLocation)
	}
}

// Spec §6.6 — errors accumulate in the order the validator produced them:
// **declared-field failures first, in declaration order, then unknown keys in
// input order.**
//
// Iteration 2 had it the other way round. Nothing pinned it then; it is measured
// now — `SocialNetwork(z=1, y=2)` reports `network` missing, `username` missing,
// then `z` and `y` (spec 004 §3.9 behavior 32 step 3).
func TestErrorAccumulationOrder(t *testing.T) {
	spec := binder.Spec{
		Fields: []binder.Field{
			{Name: "a", Required: true},
			{Name: "b", Required: true},
		},
		Policy: binder.ForbidExtra,
	}
	_, errs := bindAll(parse(t, "z: 1\ny: 2\n"), spec, []string{"cv"}, schemaerr.SourceMain)

	want := []struct {
		code schemaerr.Code
		key  string
	}{
		{code: binder.CodeMissing, key: "a"},
		{code: binder.CodeMissing, key: "b"},
		{code: binder.CodeExtraForbidden, key: "z"},
		{code: binder.CodeExtraForbidden, key: "y"},
	}
	if len(errs) != len(want) {
		t.Fatalf("errs = %+v, want %d", errs, len(want))
	}
	for i, tc := range want {
		if errs[i].Code != tc.code {
			t.Errorf("errs[%d].Code = %q, want %q", i, errs[i].Code, tc.code)
		}
		got := errs[i].SchemaLocation
		if len(got) != 2 || got[0] != "cv" || got[1] != tc.key {
			t.Errorf("errs[%d].SchemaLocation = %v, want [cv %s]", i, got, tc.key)
		}
	}
}

// Every error carries the source it was produced against and a YAML location.
func TestErrorsCarrySourceAndLocation(t *testing.T) {
	_, errs := bindAll(
		parse(t, "name: John\nurl: null\nunknown: 1\n"),
		cvSpec(binder.ForbidExtra), nil, schemaerr.SourceDesign,
	)
	if len(errs) != 1 {
		t.Fatalf("errs = %+v, want one", errs)
	}
	if errs[0].YamlSource != schemaerr.SourceDesign {
		t.Errorf("source = %q, want %q", errs[0].YamlSource, schemaerr.SourceDesign)
	}
	if errs[0].YamlLocation == nil || errs[0].YamlLocation.Start.Line != 3 {
		t.Errorf("yaml location = %+v, want line 3", errs[0].YamlLocation)
	}
}

// bindAll is binder.Bind with the unknown-key failures appended, which is what
// every production caller does at the end of its own validation. Bind returns
// them separately so a caller's own field failures can come first
// (binder.Result.ExtraErrors).
func bindAll(
	node *yamldoc.Node,
	spec binder.Spec,
	location []string,
	source schemaerr.YamlSource,
) (*binder.Result, []schemaerr.ValidationError) {
	result, errs := binder.Bind(node, spec, location, source)
	return result, append(errs, result.ExtraErrors...)
}
