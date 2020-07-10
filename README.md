# quickjs

[![MIT License](https://img.shields.io/apm/l/atomic-design-ui.svg?)](LICENSE)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/lithdew/quickjs)
[![Discord Chat](https://img.shields.io/discord/697002823123992617)](https://discord.gg/HZEbkeQ)

Go bindings to [QuickJS](https://bellard.org/quickjs/): a fast, small, and embeddable [ES2020](https://tc39.github.io/ecma262/) JavaScript interpreter.

These bindings are a WIP and do not match full parity with QuickJS' API, though expose just enough features to be usable. The version of QuickJS that these bindings bind to may be located [here](version.h).

These bindings have been tested to cross-compile and run successfully on Linux, Windows, and Mac using gcc-7 and mingw32 without any addtional compiler or linker flags.

## Usage

```
$ go get github.com/lithdew/quickjs
```

## Example

The full example code below may be found by clicking [here](examples/main.go). Find more API examples [here](quickjs_test.go).

```go
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
	defer result.Free()

	fmt.Println(result.String())
	fmt.Println()

	// Test evaluating numeric expressions.

	result, err = context.Eval(`1 + 2 * 100 - 3 + Math.sin(10)`)
	check(err)
	defer result.Free()

	fmt.Println(result.Int64())
	fmt.Println()

	// Test evaluating big integer expressions.

	result, err = context.Eval(`128n ** 16n`)
	check(err)
	defer result.Free()

	fmt.Println(result.BigInt())
	fmt.Println()

	// Test evaluating big decimal expressions.

	result, err = context.Eval(`128l ** 12l`)
	check(err)
	defer result.Free()

	fmt.Println(result.BigFloat())
	fmt.Println()

	// Test evaluating boolean expressions.

	result, err = context.Eval(`false && true`)
	check(err)
	defer result.Free()

	fmt.Println(result.Bool())
	fmt.Println()

	// Test setting and calling functions.

	A := func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		fmt.Println("A got called!")
		return ctx.Null()
	}

	B := func(ctx *quickjs.Context, this quickjs.Value, args []quickjs.Value) quickjs.Value {
		fmt.Println("B got called!")
		return ctx.Null()
	}

	globals.Set("A", context.Function(A))
	globals.Set("B", context.Function(B))

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
		defer val.Free()

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
	defer result.Free()

	if result.IsObject() {
		names, err := result.PropertyNames()
		check(err)

		fmt.Println("Object:")
		for _, name := range names {
			val := result.GetByAtom(name.Atom)
			defer val.Free()

			fmt.Printf("'%s': %s\n", name, val)
		}
	} else {
		fmt.Println(result.String())
	}
}
```

```
$ go run examples/main.go '(() => ({hello: "world", test: 2 ** 3}))()'
Hello world! 2 ** 8 = 256.

197

5192296858534827628530496329220096

1.9342813113834066795e+25

false

A got called!
B got called!
A got called!
B got called!
A got called!
B got called!
A got called!
B got called!
A got called!
B got called!

Globals:
'Object': function Object() {
    [native code]
}
'Function': function Function() {
    [native code]
}
'Error': function Error() {
    [native code]
}
'EvalError': function EvalError() {
    [native code]
}
'RangeError': function RangeError() {
    [native code]
}
'ReferenceError': function ReferenceError() {
    [native code]
}
'SyntaxError': function SyntaxError() {
    [native code]
}
'TypeError': function TypeError() {
    [native code]
}
'URIError': function URIError() {
    [native code]
}
'InternalError': function InternalError() {
    [native code]
}
'AggregateError': function AggregateError() {
    [native code]
}
'Array': function Array() {
    [native code]
}
'parseInt': function parseInt() {
    [native code]
}
'parseFloat': function parseFloat() {
    [native code]
}
'isNaN': function isNaN() {
    [native code]
}
'isFinite': function isFinite() {
    [native code]
}
'decodeURI': function decodeURI() {
    [native code]
}
'decodeURIComponent': function decodeURIComponent() {
    [native code]
}
'encodeURI': function encodeURI() {
    [native code]
}
'encodeURIComponent': function encodeURIComponent() {
    [native code]
}
'escape': function escape() {
    [native code]
}
'unescape': function unescape() {
    [native code]
}
'Infinity': Infinity
'NaN': NaN
'undefined': undefined
'__date_clock': function __date_clock() {
    [native code]
}
'Number': function Number() {
    [native code]
}
'Boolean': function Boolean() {
    [native code]
}
'String': function String() {
    [native code]
}
'Math': [object Math]
'Reflect': [object Object]
'Symbol': function Symbol() {
    [native code]
}
'eval': function eval() {
    [native code]
}
'globalThis': [object Object]
'Date': function Date() {
    [native code]
}
'RegExp': function RegExp() {
    [native code]
}
'JSON': [object JSON]
'Proxy': function Proxy() {
    [native code]
}
'Map': function Map() {
    [native code]
}
'Set': function Set() {
    [native code]
}
'WeakMap': function WeakMap() {
    [native code]
}
'WeakSet': function WeakSet() {
    [native code]
}
'ArrayBuffer': function ArrayBuffer() {
    [native code]
}
'SharedArrayBuffer': function SharedArrayBuffer() {
    [native code]
}
'Uint8ClampedArray': function Uint8ClampedArray() {
    [native code]
}
'Int8Array': function Int8Array() {
    [native code]
}
'Uint8Array': function Uint8Array() {
    [native code]
}
'Int16Array': function Int16Array() {
    [native code]
}
'Uint16Array': function Uint16Array() {
    [native code]
}
'Int32Array': function Int32Array() {
    [native code]
}
'Uint32Array': function Uint32Array() {
    [native code]
}
'BigInt64Array': function BigInt64Array() {
    [native code]
}
'BigUint64Array': function BigUint64Array() {
    [native code]
}
'Float32Array': function Float32Array() {
    [native code]
}
'Float64Array': function Float64Array() {
    [native code]
}
'DataView': function DataView() {
    [native code]
}
'Atomics': [object Atomics]
'Promise': function Promise() {
    [native code]
}
'BigInt': function BigInt() {
    [native code]
}
'BigFloat': function BigFloat() {
    [native code]
}
'BigFloatEnv': function BigFloatEnv() {
    [native code]
}
'BigDecimal': function BigDecimal() {
    [native code]
}
'Operators': function Operators() {
    [native code]
}
'A': function() { return proxy.call(this, id, ...arguments); }
'B': function() { return proxy.call(this, id, ...arguments); }
'HELLO': world
'TEST': false

Object:
'hello': world
'test': 8
```

## License

QuickJS is released under the MIT license.

QuickJS bindings are copyright Kenta Iwasaki, with code copyright Fabrice Bellard and Charlie Gordon.

