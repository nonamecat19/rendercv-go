// Package binder is the pydantic-shaped half of validation: it matches a
// document mapping against a model's field set, applies the extra-key policy of
// spec §3.32, and accumulates errors in order.
//
// It mirrors what pydantic does for `RenderCVBaseModel` and
// `RenderCVBaseModelWithExtraKeys` (src/rendercv/schema/models/base.py:1-10);
// there is no single upstream function to point at, because upstream gets this
// behavior from the framework.
package binder

import (
	"github.com/nonamecat19/rendercv-go/internal/schema/schemaerr"
	"github.com/nonamecat19/rendercv-go/internal/schema/yamldoc"
)

// Policy is one of the two base kinds of spec §3.32.
type Policy uint8

const (
	// ForbidExtra rejects unknown keys (models/base.py:5).
	ForbidExtra Policy = iota
	// AllowExtra keeps unknown keys on the object, readable by name
	// (models/base.py:9).
	AllowExtra
)

// Error codes. The user-visible text for each is pydantic's, not RenderCV's,
// and is therefore deferred with the other borrowed strings.
//
// TODO(iteration-4): spec §7.3 — pin the text of CodeMissing, CodeExtraForbidden
// and CodeModelType against upstream's rendered output, and record a divergence
// if it cannot be reproduced.
const (
	CodeMissing        schemaerr.Code = "missing"
	CodeExtraForbidden schemaerr.Code = "extra_forbidden"
	CodeModelType      schemaerr.Code = "model_type"
)

const (
	messageMissing        = "Field required"
	messageExtraForbidden = "Extra inputs are not permitted"
	messageModelType      = "Input should be a valid dictionary"
)

// Field is one declared model field. Spec §3.34: there are no aliases, so the
// YAML key is the field name exactly.
//
// Required marks a field with no default. A required field whose declared type
// admits null — `custom_connections[].url` (spec §3.81) — is still Required:
// the key must be present, and a null value binds to a nil node.
type Field struct {
	Name     string
	Required bool
}

// Spec is a model's field set plus its extra-key policy.
type Spec struct {
	Fields []Field
	Policy Policy
}

// Result is what a successful (or partially successful) bind produced.
type Result struct {
	// Values holds every declared field that was present, keyed by field name.
	// A present-but-null field maps to a node of kind KindNull, which is how
	// absent and present-and-null stay distinguishable here and nowhere else
	// (plan §4).
	Values map[string]*yamldoc.Node

	// Extras holds unknown keys, in input order, when the policy allows them
	// (spec §3.32, models/base.py:9).
	Extras []yamldoc.Item

	// KeyOrder is the input order of declared keys whose value is not null,
	// mirroring `_key_order` (models/cv/cv.py:166-173, spec §3.50). It is empty
	// for a non-mapping input (spec §5.16).
	KeyOrder []string
}

// Bind matches node against spec. It returns a result together with the errors
// it accumulated; both are meaningful, because upstream reports every field
// problem it can rather than stopping at the first.
//
// location is the schema location of the model being bound; each error's own
// location is that path with the offending key appended.
func Bind(
	node *yamldoc.Node,
	spec Spec,
	location []string,
	source schemaerr.YamlSource,
) (*Result, []schemaerr.ValidationError) {
	result := &Result{Values: map[string]*yamldoc.Node{}}

	// Spec §5.16: a non-mapping input records an empty key order and leaves the
	// type failure to be reported, rather than raising while reading keys.
	if node == nil || node.Kind != yamldoc.KindMapping {
		return result, []schemaerr.ValidationError{{
			Code:           CodeModelType,
			SchemaLocation: append([]string(nil), location...),
			YamlLocation:   spanOf(node),
			YamlSource:     source,
			Message:        messageModelType,
			Input:          inputOf(node),
		}}
	}

	declared := make(map[string]struct{}, len(spec.Fields))
	for _, field := range spec.Fields {
		declared[field.Name] = struct{}{}
	}

	var errs []schemaerr.ValidationError
	for _, item := range node.Items {
		if _, ok := declared[item.Key]; !ok {
			// Spec §5.15: the value is never consulted, so a null-valued
			// unknown key is rejected like any other.
			if spec.Policy == ForbidExtra {
				errs = append(errs, schemaerr.ValidationError{
					Code:           CodeExtraForbidden,
					SchemaLocation: locationWith(location, item.Key),
					YamlLocation:   &yamldoc.Span{Start: item.KeySpan.Start, End: item.KeySpan.End},
					YamlSource:     source,
					Message:        messageExtraForbidden,
					Input:          inputOf(item.Value),
				})
				continue
			}
			result.Extras = append(result.Extras, item)
			continue
		}

		result.Values[item.Key] = item.Value
		if item.Value != nil && item.Value.Kind != yamldoc.KindNull {
			result.KeyOrder = append(result.KeyOrder, item.Key)
		}
	}

	// Missing required fields are reported in declaration order, after the
	// extra-key errors that the input order produced.
	//
	// TODO(iteration-4): spec §6.6 makes error order contractual, but no
	// upstream test pins the relative order of extra-key and missing-field
	// errors. Confirm against upstream output when Axis 4 lands.
	for _, field := range spec.Fields {
		if !field.Required {
			continue
		}
		if _, ok := result.Values[field.Name]; ok {
			continue
		}
		errs = append(errs, schemaerr.ValidationError{
			Code:           CodeMissing,
			SchemaLocation: locationWith(location, field.Name),
			YamlLocation:   spanOf(node),
			YamlSource:     source,
			Message:        messageMissing,
			Input:          inputOf(node),
		})
	}

	return result, errs
}

// Value reports a declared field's node and whether the key was present at all.
// A present-and-null key yields a node of kind KindNull and true.
func (r *Result) Value(name string) (*yamldoc.Node, bool) {
	value, ok := r.Values[name]
	return value, ok
}

// Extra reports an unknown key retained under the AllowExtra policy
// (spec §3.67: unknown keys on an entry base are readable by name).
func (r *Result) Extra(name string) (*yamldoc.Node, bool) {
	for _, item := range r.Extras {
		if item.Key == name {
			return item.Value, true
		}
	}
	return nil, false
}

func locationWith(location []string, key string) []string {
	out := make([]string, 0, len(location)+1)
	out = append(out, location...)
	return append(out, key)
}

func spanOf(node *yamldoc.Node) *yamldoc.Span {
	if node == nil {
		return nil
	}
	span := node.Span
	return &span
}

func inputOf(node *yamldoc.Node) string {
	if node == nil {
		return ""
	}
	return node.Raw
}
