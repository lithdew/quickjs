package quickjs

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestObject(t *testing.T) {
	ctx := NewRuntime().NewContext()

	test := ctx.Object()
	test.Set("A", ctx.String("String A"))
	test.Set("B", ctx.String("String B"))
	test.Set("C", ctx.String("String C"))
	ctx.Globals().Set("test", test)

	result, err := ctx.Eval(`Object.keys(test).map(key => test[key]).join(" ")`)
	require.NoError(t, err)

	require.EqualValues(t, "String A String B String C", result.String())
}

func TestArray(t *testing.T) {
	ctx := NewRuntime().NewContext()

	test := ctx.Array()
	for i := int64(0); i < 3; i++ {
		test.SetByInt64(i, ctx.String(fmt.Sprintf("test %d", i)))
	}
	for i := int64(0); i < test.Len(); i++ {
		require.EqualValues(t, fmt.Sprintf("test %d", i), test.GetByUint32(uint32(i)).String())
	}

	ctx.Globals().Set("test", test)

	result, err := ctx.Eval(`test.map(v => v.toUpperCase())`)
	require.NoError(t, err)

	require.EqualValues(t, `TEST 0,TEST 1,TEST 2`, result.String())
}

func TestFunctionThrowError(t *testing.T) {
	expected := errors.New("expected error")

	ctx := NewRuntime().NewContext()
	ctx.Globals().SetFunction("A", func(ctx Context, this Value, args []Value) Value {
		return ctx.ThrowError(expected)
	})

	_, actual := ctx.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "Error: "+expected.Error(), actual.Error())
}

func TestFunction(t *testing.T) {
	rt := NewRuntime()
	ctx := rt.NewContext()

	A := make(chan struct{})
	B := make(chan struct{})

	ctx.Globals().Set("A", ctx.Function(func(ctx Context, this Value, args []Value) Value {
		require.Len(t, args, 4)
		require.True(t, args[0].IsString() && args[0].String() == "hello world!")
		require.True(t, args[1].IsNumber() && args[1].Int32() == 1)
		require.True(t, args[2].IsNumber() && args[2].Int64() == 8)
		require.True(t, args[3].IsNull())

		close(A)

		return ctx.String("A says hello")
	}))

	ctx.Globals().Set("B", ctx.Function(func(ctx Context, this Value, args []Value) Value {
		require.Len(t, args, 0)

		close(B)

		return ctx.Float64(256)
	}))

	result, err := ctx.Eval(`A("hello world!", 1, 2 ** 3, null)`)
	require.NoError(t, err)
	require.True(t, result.IsString() && result.String() == "A says hello")
	<-A

	result, err = ctx.Eval(`B()`)
	require.NoError(t, err)
	require.True(t, result.IsNumber() && result.Uint32() == 256)
	<-B
}
