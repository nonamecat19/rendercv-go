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
	"strconv"

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
	CodeStringType     schemaerr.Code = "string_type"
	CodeListType       schemaerr.Code = "list_type"

	// The two codes only a date field's union arms produce (ValueType.branches).
	CodeIntType      schemaerr.Code = "int_type"
	CodeLiteralError schemaerr.Code = "literal_error"

	// CodeDateValue is the arbitrary `date` field's code; CodeDateOther is
	// `start_date`/`end_date`'s. See ValueType.dateCode.
	CodeDateValue schemaerr.Code = "value_error"
	CodeDateOther schemaerr.Code = "rendercv_other_error"
)

const (
	messageMissing        = "Field required"
	messageExtraForbidden = "Extra inputs are not permitted"
	messageModelType      = "Input should be a valid dictionary"

	// TODO(iteration-4): spec 003 §4.4 and §4.5 — these two are pydantic's own
	// text as well, and belong with the borrowed strings of spec 002 §7.3.
	messageStringType = "Input should be a valid string"
	messageListType   = "Input should be a valid list"

	// The union-arm texts of ValueType.branches, pydantic's own as well.
	// `literalPresent` is the location element, not a message: pydantic spells the
	// arm with Python's repr of the literal, single quotes included.
	messageIntType        = "Input should be a valid integer"
	messageLiteralPresent = "Input should be 'present'"
	literalPresent        = "literal['present']"
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

	// Value is the field's declared shape. The zero value, ValueAny, means the
	// value is bound as a raw node and never checked, which is what every field
	// declared before spec 003 §3.13 did.
	Value ValueType

	// Scalar is an additional per-field check on a scalar value, run at the
	// field's declared position and only after its shape check passed. It is a
	// hook rather than a call into the validating package because the date parsers
	// and the DOI matcher live in models/cv/entries/**, which imports this package
	// — the dependency only runs one way.
	//
	// raw is the scalar as written; isInteger reports that the YAML reader
	// resolved it to an integer, which upstream distinguishes because
	// `get_date_object` treats `int` and `str` differently
	// (entry_with_complex_fields.py:67-68).
	//
	// Set for the three date shapes and for `PublicationEntry.doi`'s pattern. Its
	// failures carry ScalarCode when that is set, and the value type's own date
	// code otherwise.
	Scalar func(raw string, isInteger bool) error

	// ScalarCode overrides the code a Scalar failure carries. Needed because
	// `doi`'s pattern failure is `string_pattern_mismatch` while a date's is one
	// of the two date codes.
	ScalarCode schemaerr.Code
}

// ValueType is the declared shape of a field's value. It is deliberately not a
// general type system: upstream's entry models declare exactly three shapes
// (spec 003 §3.13, plan §4).
type ValueType uint8

const (
	// ValueAny performs no check — the field is bound as a raw node.
	ValueAny ValueType = iota
	// ValueString is Python's `str`.
	ValueString
	// ValueStringList is Python's `list[str]`.
	ValueStringList

	// The three date shapes. They are value types rather than post-hoc checks so
	// that Bind's single declaration-order pass emits them in place — upstream
	// reports a bad `date` between the own fields and `location`, which an
	// appended check cannot do (spec 004 §3.9a behavior 33a, plan §2.5).
	//
	// Each names its own declared arm order, which is what a non-scalar value
	// needs: `date` is `int | str` and reports `int` first, `start_date` is
	// `str | int` and reports `str` first (spec 004 §3.9b behavior 33d).

	// ValueArbitraryDate is `ArbitraryDate = int | str` with a lenient validator
	// (entry_with_date.py:34-36). Its failures are `value_error`, because upstream
	// lets a bare ValueError escape.
	ValueArbitraryDate
	// ValueExactDate is `ExactDate = str | int` with a strict validator
	// (entry_with_complex_fields.py:40). Its failures are `rendercv_other_error`,
	// because upstream raises PydanticCustomError.
	ValueExactDate
	// ValueExactDateOrPresent is `ExactDate | Literal["present"]`
	// (entry_with_complex_fields.py:98). Same code as ValueExactDate; the extra
	// literal arm only shows up in the non-scalar branch list.
	ValueExactDateOrPresent
)

// isDate reports whether a value type is one of the three date shapes.
func (v ValueType) isDate() bool {
	return v == ValueArbitraryDate || v == ValueExactDate || v == ValueExactDateOrPresent
}

// dateCode is the error code a date shape's failures carry. The split is
// upstream's: `entry_with_date.py:26-29` lets a bare ValueError through, while
// `entry_with_complex_fields.py:31-36` raises PydanticCustomError(other). Measured
// on both paths (spec 004 §3.9c behavior 33h).
func (v ValueType) dateCode() schemaerr.Code {
	if v == ValueArbitraryDate {
		return CodeDateValue
	}
	return CodeDateOther
}

// dateBranch is one arm of a date field's declared union, as pydantic reports
// it for a value that fits no arm at all.
//
// suffix is appended to the field's location. These are the **only** synthetic
// location elements the port constructs, and they are not decoration: spec 004
// §3.9b behavior 33f is the argument for them. `date` is declared `int | str`
// and `start_date` is declared `str | int`, so the two fields survive spec §3.3's
// filter and §3.8's dedup with *different* messages — "Input should be a valid
// integer." for `date`, "Input should be a valid string." for `start_date`.
// Emitting one hand-picked record per field gets one of the two wrong; emitting
// the arms in declared order and letting the filter and the dedup run gets both
// right. Do not collapse these to a single record.
type dateBranch struct {
	suffix  []string
	code    schemaerr.Code
	message string
}

// exactDateWrapper is the name pydantic gives the `function-after` step wrapping
// `end_date`'s union, and it appears verbatim in that field's locations
// (spec 004 §3.9b behavior 33d, measured).
const exactDateWrapper = "function-after[validate_exact_date(), union[str,int]]"

// branches lists a date shape's union arms in declared order. Measured against
// the vendored Python for a mapping and for a sequence value; both give the same
// list.
func (v ValueType) branches() []dateBranch {
	switch v {
	case ValueArbitraryDate:
		// `ArbitraryDate = int | str` (entry_with_date.py:34-36) — `int` first.
		return []dateBranch{
			{suffix: []string{"int"}, code: CodeIntType, message: messageIntType},
			{suffix: []string{"str"}, code: CodeStringType, message: messageStringType},
		}
	case ValueExactDate:
		// `ExactDate = str | int` (entry_with_complex_fields.py:40) — `str` first.
		return []dateBranch{
			{suffix: []string{"str"}, code: CodeStringType, message: messageStringType},
			{suffix: []string{"int"}, code: CodeIntType, message: messageIntType},
		}
	case ValueExactDateOrPresent:
		// `ExactDate | Literal["present"]` (entry_with_complex_fields.py:98). The
		// two `ExactDate` arms sit under the after-validator's wrapper name; the
		// literal arm does not.
		return []dateBranch{
			{suffix: []string{exactDateWrapper, "str"}, code: CodeStringType, message: messageStringType},
			{suffix: []string{exactDateWrapper, "int"}, code: CodeIntType, message: messageIntType},
			{suffix: []string{literalPresent}, code: CodeLiteralError, message: messageLiteralPresent},
		}
	case ValueAny, ValueString, ValueStringList:
	}
	return nil
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

	// One pass in declaration order, emitting per field either its absence or
	// its shape failure. Pydantic interleaves the two rather than reporting all
	// absences first — verified against the vendored Python, where
	// `PublicationEntry(summary=5, authors="x")` reports `title` missing, then
	// `authors` list_type, then `summary` string_type, which is exactly the
	// declared order of those three fields (spec 003 §5.10).
	//
	// A present field is never also reported as missing, so the two branches are
	// mutually exclusive: a required field written as null reports its type
	// failure, not its absence (spec 003 §5.7).
	//
	// TODO(iteration-4): spec §6.6 makes error order contractual, but no
	// upstream test pins the relative order of extra-key and field errors.
	// Confirm against upstream output when Axis 4 lands.
	for _, field := range spec.Fields {
		value, present := result.Values[field.Name]
		if !present {
			if field.Required {
				errs = append(errs, schemaerr.ValidationError{
					Code:           CodeMissing,
					SchemaLocation: locationWith(location, field.Name),
					YamlLocation:   spanOf(node),
					YamlSource:     source,
					Message:        messageMissing,
					Input:          inputOf(node),
				})
			}
			continue
		}
		errs = append(errs, checkValue(field, value, location, source)...)
	}

	return result, errs
}

// checkValue applies a field's declared shape to a present value, per the table
// of plan §4.
//
// Null is the asymmetric case: pydantic declares these fields `str | None` and
// `list[str] | None`, so a null in an optional field is the default and passes.
// A null in a *required* field is a type failure rather than an absence, because
// the key is there (spec 003 §5.7).
func checkValue(
	field Field,
	value *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if field.Value == ValueAny {
		return nil
	}

	isNull := value == nil || value.Kind == yamldoc.KindNull
	if isNull && !field.Required {
		return nil
	}

	if field.Value.isDate() {
		if isNonScalar(value) {
			return dateBranchErrors(field, value, location, source)
		}
		return checkScalar(field, value, location, source)
	}

	switch field.Value {
	case ValueString:
		if isNull || !isTextKind(value.Kind) {
			return []schemaerr.ValidationError{
				valueError(CodeStringType, messageStringType, locationWith(location, field.Name), value, source),
			}
		}
		// The shape held, so an additional per-field constraint runs here, at the
		// field's declared position (spec 004 §3.9a behavior 33a).
		return checkScalar(field, value, location, source)
	case ValueStringList:
		if isNull || value.Kind != yamldoc.KindSequence {
			return []schemaerr.ValidationError{
				valueError(CodeListType, messageListType, locationWith(location, field.Name), value, source),
			}
		}
		// Element errors are located at the element's own index, rendered as a
		// decimal string — the shape upstream's own fixture uses for an entry
		// index (expected_errors.yaml:56-57).
		var errs []schemaerr.ValidationError
		for i, element := range value.Elems {
			if element != nil && isTextKind(element.Kind) {
				continue
			}
			elementLocation := locationWith(locationWith(location, field.Name), strconv.Itoa(i))
			errs = append(errs, valueError(CodeStringType, messageStringType, elementLocation, element, source))
		}
		return errs
	case ValueAny, ValueArbitraryDate, ValueExactDate, ValueExactDateOrPresent:
		// Handled above; listed so the switch stays exhaustive.
	}

	return nil
}

// isNonScalar reports whether a value is a mapping or a sequence — the two
// kinds that fit no arm of a date field's union.
func isNonScalar(value *yamldoc.Node) bool {
	return value != nil &&
		(value.Kind == yamldoc.KindMapping || value.Kind == yamldoc.KindSequence)
}

// dateBranchErrors reports one failure per declared union arm. See dateBranch
// for why the port emits the whole list rather than the one record that
// survives.
func dateBranchErrors(
	field Field,
	value *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	fieldLocation := locationWith(location, field.Name)

	branches := field.Value.branches()
	errs := make([]schemaerr.ValidationError, 0, len(branches))
	for _, branch := range branches {
		branchLocation := fieldLocation
		for _, element := range branch.suffix {
			branchLocation = locationWith(branchLocation, element)
		}
		errs = append(errs, valueError(branch.code, branch.message, branchLocation, value, source))
	}
	return errs
}

// checkScalar runs a field's additional scalar constraint at its declared
// position. Non-scalar values never reach it: dateBranchErrors handles those.
func checkScalar(
	field Field,
	value *yamldoc.Node,
	location []string,
	source schemaerr.YamlSource,
) []schemaerr.ValidationError {
	if field.Scalar == nil || value == nil {
		return nil
	}
	if isNonScalar(value) || value.Kind == yamldoc.KindNull {
		return nil
	}

	err := field.Scalar(value.Raw, value.Kind == yamldoc.KindInt)
	if err == nil {
		return nil
	}

	code := field.ScalarCode
	if code == "" {
		code = field.Value.dateCode()
	}
	return []schemaerr.ValidationError{{
		Code:           code,
		SchemaLocation: locationWith(location, field.Name),
		YamlLocation:   spanOf(value),
		YamlSource:     source,
		Message:        err.Error(),
		Input:          value.Raw,
	}}
}

// isTextKind reports whether a node's kind binds to a Python `str`.
//
// Only KindString does. Pydantic rejects `int`, `float` and `bool` for a `str`
// field even with strict mode off — verified against the vendored Python for
// `summary: 5`, which reports `string_type`. This matters because the YAML
// reader classifies `2020` as KindInt (spec 002 plan §3), so `location: 2020`
// must fail rather than coerce (plan §4).
func isTextKind(kind yamldoc.Kind) bool {
	return kind == yamldoc.KindString
}

func valueError(
	code schemaerr.Code,
	message string,
	location []string,
	value *yamldoc.Node,
	source schemaerr.YamlSource,
) schemaerr.ValidationError {
	return schemaerr.ValidationError{
		Code:           code,
		SchemaLocation: location,
		YamlLocation:   spanOf(value),
		YamlSource:     source,
		Message:        message,
		Input:          inputOf(value),
	}
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
