package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/lithdew/quickjs"
	"strings"
)

func check(err error) {
	if err != nil {
		var evalErr *quickjs.Error
		if errors.As(err, &evalErr) {
			fmt.Println(evalErr.Cause)
			fmt.Println(evalErr.Stack)
		}
		panic(err)
	}
}

func main() {
	runtime := quickjs.NewRuntime()
	defer runtime.Free()

	context := runtime.NewContext()
	defer context.Free()

	globals := context.Globals()

	// Test evaluating template strings.

	result, err := context.Eval("`Hello world! 2 ** 8 = ${2 ** 8}.`")
	check(err)

	fmt.Println(result.String())
	fmt.Println()

	// Test evaluating numeric expressions.

	result, err = context.Eval(`1 + 2 * 100 - 3 + Math.sin(10)`)
	check(err)

	fmt.Println(result.Int64())
	fmt.Println()

	// Test evaluating big integer expressions.

	result, err = context.Eval(`128n ** 16n`)
	check(err)

	fmt.Println(result.BigInt())
	fmt.Println()

	// Test evaluating big decimal expressions.

	result, err = context.Eval(`128l ** 12l`)
	check(err)

	fmt.Println(result.BigFloat())
	fmt.Println()

	// Test evaluating boolean expressions.

	result, err = context.Eval(`false && true`)
	check(err)

	fmt.Println(result.Bool())
	fmt.Println()

	// Test setting and calling functions.

	globals.Set("A", context.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		fmt.Println("A got called!")
		return ctx.Null()
	}))

	globals.Set("B", context.Function(func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		fmt.Println("B got called!")
		return ctx.Null()
	}))

	_, err = context.Eval(`for (let i = 0; i < 10; i++) { if (i % 2 === 0) A(); else B(); }`)
	check(err)

	fmt.Println()

	// Test setting global variables.

	_, err = context.Eval(`HELLO = "world"; TEST = false;`)
	check(err)

	names, err := globals.PropertyNames()
	check(err)

	fmt.Println("Globals:")
	for _, name := range names {
		val := globals.GetByAtom(name.Atom)
		fmt.Printf("'%s': %s\n", name, val)
	}
	fmt.Println()

	// Test evaluating arbitrary expressions from flag arguments.

	flag.Parse()
	if flag.NArg() == 0 {
		return
	}

	result, err = context.Eval(strings.Join(flag.Args(), " "))
	check(err)

	if result.IsObject() {
		names, err := result.PropertyNames()
		check(err)

		fmt.Println("Object:")
		for _, name := range names {
			val := result.GetByAtom(name.Atom)
			fmt.Printf("'%s': %s\n", name, val)
		}
	} else {
		fmt.Println(result.String())
	}
}
