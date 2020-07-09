package quickjs

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

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
