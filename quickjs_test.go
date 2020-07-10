package quickjs

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	stdruntime "runtime"
	"sync"
	"testing"
)

func TestObject(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	test := context.Object()
	test.Set("A", context.String("String A"))
	test.Set("B", context.String("String B"))
	test.Set("C", context.String("String C"))
	context.Globals().Set("test", test)

	result, err := context.Eval(`Object.keys(test).map(key => test[key]).join(" ")`)
	require.NoError(t, err)
	defer result.Free()

	require.EqualValues(t, "String A String B String C", result.String())
}

func TestArray(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	test := context.Array()
	for i := int64(0); i < 3; i++ {
		test.SetByInt64(i, context.String(fmt.Sprintf("test %d", i)))
	}
	for i := int64(0); i < test.Len(); i++ {
		require.EqualValues(t, fmt.Sprintf("test %d", i), test.GetByUint32(uint32(i)).String())
	}

	context.Globals().Set("test", test)

	result, err := context.Eval(`test.map(v => v.toUpperCase())`)
	require.NoError(t, err)
	defer result.Free()

	require.EqualValues(t, `TEST 0,TEST 1,TEST 2`, result.String())
}

func TestBadSyntax(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	_, err := context.Eval(`"bad syntax'`)
	require.Error(t, err)
}

func TestFunctionThrowError(t *testing.T) {
	expected := errors.New("expected error")

	runtime := NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	context.Globals().SetFunction("A", func(ctx *Context, this Value, args []Value) Value {
		return ctx.ThrowError(expected)
	})

	_, actual := context.Eval("A()")
	require.Error(t, actual)
	require.EqualValues(t, "Error: "+expected.Error(), actual.Error())
}

func TestFunction(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	A := make(chan struct{})
	B := make(chan struct{})

	context.Globals().SetFunction("A", func(ctx *Context, this Value, args []Value) Value {
		require.Len(t, args, 4)
		require.True(t, args[0].IsString() && args[0].String() == "hello world!")
		require.True(t, args[1].IsNumber() && args[1].Int32() == 1)
		require.True(t, args[2].IsNumber() && args[2].Int64() == 8)
		require.True(t, args[3].IsNull())

		close(A)

		return ctx.String("A says hello")
	})

	context.Globals().SetFunction("B", func(ctx *Context, this Value, args []Value) Value {
		require.Len(t, args, 0)

		close(B)

		return ctx.Float64(256)
	})

	result, err := context.Eval(`A("hello world!", 1, 2 ** 3, null)`)
	require.NoError(t, err)
	defer result.Free()

	require.True(t, result.IsString() && result.String() == "A says hello")
	<-A

	result, err = context.Eval(`B()`)
	require.NoError(t, err)
	defer result.Free()

	require.True(t, result.IsNumber() && result.Uint32() == 256)
	<-B
}

func TestConcurrency(t *testing.T) {
	n := 32
	m := 10000

	var wg sync.WaitGroup
	wg.Add(n)

	req := make(chan struct{}, n)
	res := make(chan int64, m)

	for i := 0; i < n; i++ {
		go func() {
			stdruntime.LockOSThread()

			defer wg.Done()

			runtime := NewRuntime()
			defer runtime.Free()

			context := runtime.NewContext()
			defer context.Free()

			for range req {
				result, err := context.Eval(`new Date().getTime()`)
				require.NoError(t, err)

				res <- result.Int64()

				result.Free()
			}
		}()
	}

	for i := 0; i < m; i++ {
		req <- struct{}{}
	}
	close(req)

	wg.Wait()

	for i := 0; i < m; i++ {
		<-res
	}
}
