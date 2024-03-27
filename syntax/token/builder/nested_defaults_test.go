package builder_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/grafana/alloy/syntax/vm"
	"github.com/stretchr/testify/require"
)

const (
	defaultNumber      = 123
	otherDefaultNumber = 321
)

var testCases = []struct {
	name          string
	input         interface{}
	expectedAlloy string
}{
	{
		name:          "struct propagating default - input matching default",
		input:         StructPropagatingDefault{Inner: AttrWithDefault{Number: defaultNumber}},
		expectedAlloy: "",
	},
	{
		name:  "struct propagating default - input with zero-value struct",
		input: StructPropagatingDefault{},
		expectedAlloy: `
		inner {
			number = 0
		}	
		`,
	},
	{
		name:  "struct propagating default - input with non-default value",
		input: StructPropagatingDefault{Inner: AttrWithDefault{Number: 42}},
		expectedAlloy: `
		inner {
			number = 42
		}	
		`,
	},
	{
		name:          "pointer propagating default - input matching default",
		input:         PtrPropagatingDefault{Inner: &AttrWithDefault{Number: defaultNumber}},
		expectedAlloy: "",
	},
	{
		name:  "pointer propagating default - input with zero value",
		input: PtrPropagatingDefault{Inner: &AttrWithDefault{}},
		expectedAlloy: `
		inner {
			number = 0
		}	
		`,
	},
	{
		name:  "pointer propagating default - input with non-default value",
		input: PtrPropagatingDefault{Inner: &AttrWithDefault{Number: 42}},
		expectedAlloy: `
		inner {
			number = 42
		}	
		`,
	},
	{
		name:          "zero default - input with zero value",
		input:         ZeroDefault{Inner: &AttrWithDefault{}},
		expectedAlloy: "",
	},
	{
		name:  "zero default - input with non-default value",
		input: ZeroDefault{Inner: &AttrWithDefault{Number: 42}},
		expectedAlloy: `
		inner {
			number = 42
		}	
		`,
	},
	{
		name:  "no default - input with zero value",
		input: NoDefaultDefined{Inner: &AttrWithDefault{}},
		expectedAlloy: `
		inner {
			number = 0
		}	
		`,
	},
	{
		name:  "no default - input with non-default value",
		input: NoDefaultDefined{Inner: &AttrWithDefault{Number: 42}},
		expectedAlloy: `
		inner {
			number = 42
		}	
		`,
	},
	{
		name:          "mismatching default - input matching outer default",
		input:         MismatchingDefault{Inner: &AttrWithDefault{Number: otherDefaultNumber}},
		expectedAlloy: "",
	},
	{
		name:          "mismatching default - input matching inner default",
		input:         MismatchingDefault{Inner: &AttrWithDefault{Number: defaultNumber}},
		expectedAlloy: "inner { }",
	},
	{
		name:  "mismatching default - input with non-default value",
		input: MismatchingDefault{Inner: &AttrWithDefault{Number: 42}},
		expectedAlloy: `
		inner {
			number = 42
		}	
		`,
	},
}

func TestNestedDefaults(t *testing.T) {
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%T/%s", tc.input, tc.name), func(t *testing.T) {
			f := builder.NewFile()
			f.Body().AppendFrom(tc.input)
			actualAlloy := string(f.Bytes())
			expected := format(t, tc.expectedAlloy)
			require.Equal(t, expected, actualAlloy, "generated Alloy didn't match expected")

			// Now decode the Alloy config produced above and make sure it's the same
			// as the input.
			eval := vm.New(parseBlock(t, actualAlloy))
			vPtr := reflect.New(reflect.TypeOf(tc.input)).Interface()
			require.NoError(t, eval.Evaluate(nil, vPtr), "alloy evaluation error")

			actualOut := reflect.ValueOf(vPtr).Elem().Interface()
			require.Equal(t, tc.input, actualOut, "Invariant violated: encoded and then decoded block didn't match the original value")
		})
	}
}

func TestPtrPropagatingDefaultWithNil(t *testing.T) {
	// This is a special case - when defaults are correctly defined, the `Inner: nil` should mean to use defaults.
	// Encoding will encode to empty string and decoding will produce the default value - `Inner: {Number: 123}`.
	input := PtrPropagatingDefault{}
	expectedEncodedAlloy := ""
	expectedDecoded := PtrPropagatingDefault{Inner: &AttrWithDefault{Number: 123}}

	f := builder.NewFile()
	f.Body().AppendFrom(input)
	actualAlloy := string(f.Bytes())
	expected := format(t, expectedEncodedAlloy)
	require.Equal(t, expected, actualAlloy, "generated Alloy didn't match expected")

	// Now decode the Alloy produced above and make sure it's the same as the input.
	eval := vm.New(parseBlock(t, actualAlloy))
	vPtr := reflect.New(reflect.TypeOf(input)).Interface()
	require.NoError(t, eval.Evaluate(nil, vPtr), "alloy evaluation error")

	actualOut := reflect.ValueOf(vPtr).Elem().Interface()
	require.Equal(t, expectedDecoded, actualOut)
}

// StructPropagatingDefault has the outer defaults matching the inner block's defaults. The inner block is a struct.
type StructPropagatingDefault struct {
	Inner AttrWithDefault `alloy:"inner,block,optional"`
}

func (o *StructPropagatingDefault) SetToDefault() {
	inner := &AttrWithDefault{}
	inner.SetToDefault()
	*o = StructPropagatingDefault{Inner: *inner}
}

// PtrPropagatingDefault has the outer defaults matching the inner block's defaults. The inner block is a pointer.
type PtrPropagatingDefault struct {
	Inner *AttrWithDefault `alloy:"inner,block,optional"`
}

func (o *PtrPropagatingDefault) SetToDefault() {
	inner := &AttrWithDefault{}
	inner.SetToDefault()
	*o = PtrPropagatingDefault{Inner: inner}
}

// MismatchingDefault has the outer defaults NOT matching the inner block's defaults. The inner block is a pointer.
type MismatchingDefault struct {
	Inner *AttrWithDefault `alloy:"inner,block,optional"`
}

func (o *MismatchingDefault) SetToDefault() {
	*o = MismatchingDefault{Inner: &AttrWithDefault{
		Number: otherDefaultNumber,
	}}
}

// ZeroDefault has the outer defaults setting to zero values. The inner block is a pointer.
type ZeroDefault struct {
	Inner *AttrWithDefault `alloy:"inner,block,optional"`
}

func (o *ZeroDefault) SetToDefault() {
	*o = ZeroDefault{Inner: &AttrWithDefault{}}
}

// NoDefaultDefined has no defaults defined. The inner block is a pointer.
type NoDefaultDefined struct {
	Inner *AttrWithDefault `alloy:"inner,block,optional"`
}

// AttrWithDefault has a default value of a non-zero number.
type AttrWithDefault struct {
	Number int `alloy:"number,attr,optional"`
}

func (i *AttrWithDefault) SetToDefault() {
	*i = AttrWithDefault{Number: defaultNumber}
}

func parseBlock(t *testing.T, input string) *ast.BlockStmt {
	t.Helper()

	input = fmt.Sprintf("test { %s }", input)
	res, err := parser.ParseFile("", []byte(input))
	require.NoError(t, err)
	require.Len(t, res.Body, 1)

	stmt, ok := res.Body[0].(*ast.BlockStmt)
	require.True(t, ok, "Expected stmt to be a ast.BlockStmt, got %T", res.Body[0])
	return stmt
}
