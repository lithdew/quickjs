package quickjs

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"sync/atomic"
	"unsafe"
)

/*
#cgo CFLAGS: -D_GNU_SOURCE
#cgo CFLAGS: -DCONFIG_BIGNUM
#cgo CFLAGS: -fno-asynchronous-unwind-tables
#cgo LDFLAGS: -lm -lpthread -static

#include "bridge.h"
*/
import "C"

type Runtime struct {
	ref *C.JSRuntime
}

func NewRuntime() Runtime {
	rt := Runtime{ref: C.JS_NewRuntime()}
	C.JS_SetCanBlock(rt.ref, C.int(1))
	return rt
}

func (r Runtime) RunGC() { C.JS_RunGC(r.ref) }

func (r Runtime) Free() { C.JS_FreeRuntime(r.ref) }

func (r Runtime) NewContext() *Context {
	ref := C.JS_NewContext(r.ref)

	C.JS_AddIntrinsicBigFloat(ref)
	C.JS_AddIntrinsicBigDecimal(ref)
	C.JS_AddIntrinsicOperators(ref)
	C.JS_EnableBignumExt(ref, C.int(1))

	return &Context{ref: ref}
}

func (r Runtime) ExecutePendingJob() (Context, error) {
	var ctx Context

	err := C.JS_ExecutePendingJob(r.ref, &ctx.ref)
	if err <= 0 {
		if err == 0 {
			return ctx, io.EOF
		}
		return ctx, ctx.Exception()
	}

	return ctx, nil
}

type Function func(ctx *Context, this Value, args []Value) Value

type funcEntry struct {
	ctx *Context
	fn  Function
}

var funcPtrLen int64
var funcPtrLock sync.Mutex
var funcPtrStore = make(map[int64]funcEntry)
var funcPtrClassID C.JSClassID

func init() { C.JS_NewClassID(&funcPtrClassID) }

func storeFuncPtr(v funcEntry) int64 {
	id := atomic.AddInt64(&funcPtrLen, 1) - 1
	funcPtrLock.Lock()
	defer funcPtrLock.Unlock()
	funcPtrStore[id] = v
	return id
}

func restoreFuncPtr(ptr int64) funcEntry {
	funcPtrLock.Lock()
	defer funcPtrLock.Unlock()
	return funcPtrStore[ptr]
}

//func freeFuncPtr(ptr int64) {
//	funcPtrLock.Lock()
//	defer funcPtrLock.Unlock()
//	delete(funcPtrStore, ptr)
//}

//export proxy
func proxy(ctx *C.JSContext, thisVal C.JSValueConst, argc C.int, argv *C.JSValueConst) C.JSValue {
	refs := (*[1 << 30]C.JSValueConst)(unsafe.Pointer(argv))[:argc:argc]

	id := C.int64_t(0)
	C.JS_ToInt64(ctx, &id, refs[0])

	entry := restoreFuncPtr(int64(id))

	args := make([]Value, len(refs)-1)
	for i := 0; i < len(args); i++ {
		args[i].ctx = entry.ctx
		args[i].ref = refs[1+i]
	}

	result := entry.fn(entry.ctx, Value{ctx: entry.ctx, ref: thisVal}, args)

	return result.ref
}

type Context struct {
	ref     *C.JSContext
	globals *Value
	proxy   *Value
}

func (ctx *Context) Free() {
	if ctx.proxy != nil {
		ctx.proxy.Free()
	}
	if ctx.globals != nil {
		ctx.globals.Free()
	}

	C.JS_FreeContext(ctx.ref)
}

func (ctx *Context) Function(fn Function) Value {
	val := ctx.eval(`(proxy, id) => function() { return proxy.call(this, id, ...arguments); }`)
	if val.IsException() {
		return val
	}
	defer val.Free()

	funcPtr := storeFuncPtr(funcEntry{ctx: ctx, fn: fn})
	funcPtrVal := ctx.Int64(funcPtr)

	if ctx.proxy == nil {
		ctx.proxy = &Value{
			ctx: ctx,
			ref: C.JS_NewCFunction(ctx.ref, (*C.JSCFunction)(unsafe.Pointer(C.InvokeProxy)), nil, C.int(0)),
		}
	}

	args := []C.JSValue{ctx.proxy.ref, funcPtrVal.ref}

	return Value{ctx: ctx, ref: C.JS_Call(ctx.ref, val.ref, ctx.Null().ref, C.int(len(args)), &args[0])}
}

func (ctx *Context) Null() Value {
	return Value{ctx: ctx, ref: C.JS_NewNull()}
}

func (ctx *Context) Undefined() Value {
	return Value{ctx: ctx, ref: C.JS_NewUndefined()}
}

func (ctx *Context) Uninitialized() Value {
	return Value{ctx: ctx, ref: C.JS_NewUninitialized()}
}

func (ctx *Context) Error(err error) Value {
	val := Value{ctx: ctx, ref: C.JS_NewError(ctx.ref)}
	val.Set("message", ctx.String(err.Error()))
	return val
}

func (ctx *Context) Bool(b bool) Value {
	bv := 0
	if b {
		bv = 1
	}
	return Value{ctx: ctx, ref: C.JS_NewBool(ctx.ref, C.int(bv))}
}

func (ctx *Context) Int32(v int32) Value {
	return Value{ctx: ctx, ref: C.JS_NewInt32(ctx.ref, C.int32_t(v))}
}

func (ctx *Context) Int64(v int64) Value {
	return Value{ctx: ctx, ref: C.JS_NewInt64(ctx.ref, C.int64_t(v))}
}

func (ctx *Context) Uint32(v uint32) Value {
	return Value{ctx: ctx, ref: C.JS_NewUint32(ctx.ref, C.uint32_t(v))}
}

func (ctx *Context) BigUint64(v uint64) Value {
	return Value{ctx: ctx, ref: C.JS_NewBigUint64(ctx.ref, C.uint64_t(v))}
}

func (ctx *Context) Float64(v float64) Value {
	return Value{ctx: ctx, ref: C.JS_NewFloat64(ctx.ref, C.double(v))}
}

func (ctx *Context) String(v string) Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Value{ctx: ctx, ref: C.JS_NewString(ctx.ref, ptr)}
}

func (ctx *Context) Atom(v string) Atom {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Atom{ctx: ctx, ref: C.JS_NewAtom(ctx.ref, ptr)}
}

func (ctx *Context) eval(code string) Value {
	return ctx.evalFile(code, "code")
}

func (ctx *Context) evalFile(code, filename string) Value {
	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	filenamePtr := C.CString(filename)
	defer C.free(unsafe.Pointer(filenamePtr))

	return Value{ctx: ctx, ref: C.JS_Eval(ctx.ref, codePtr, C.size_t(len(code)), filenamePtr, C.int(0))}
}

func (ctx *Context) Eval(code string) (Value, error) { return ctx.EvalFile(code, "code") }

func (ctx *Context) EvalFile(code, filename string) (Value, error) {
	val := ctx.evalFile(code, filename)
	if val.IsException() {
		return val, ctx.Exception()
	}
	return val, nil
}

func (ctx *Context) Globals() Value {
	if ctx.globals == nil {
		ctx.globals = &Value{
			ctx: ctx,
			ref: C.JS_GetGlobalObject(ctx.ref),
		}
	}
	return *ctx.globals
}

func (ctx *Context) Throw(v Value) Value {
	return Value{ctx: ctx, ref: C.JS_Throw(ctx.ref, v.ref)}
}

func (ctx *Context) ThrowError(err error) Value { return ctx.Throw(ctx.Error(err)) }

func (ctx *Context) ThrowSyntaxError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowSyntaxError(ctx.ref, causePtr)}
}

func (ctx *Context) ThrowTypeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowTypeError(ctx.ref, causePtr)}
}

func (ctx *Context) ThrowReferenceError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowReferenceError(ctx.ref, causePtr)}
}

func (ctx *Context) ThrowRangeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowRangeError(ctx.ref, causePtr)}
}

func (ctx *Context) ThrowInternalError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: ctx, ref: C.ThrowInternalError(ctx.ref, causePtr)}
}

func (ctx *Context) Exception() error {
	val := Value{ctx: ctx, ref: C.JS_GetException(ctx.ref)}
	defer val.Free()
	return val.Error()
}

func (ctx *Context) Object() Value {
	return Value{ctx: ctx, ref: C.JS_NewObject(ctx.ref)}
}

func (ctx *Context) Array() Value {
	return Value{ctx: ctx, ref: C.JS_NewArray(ctx.ref)}
}

type Atom struct {
	ctx *Context
	ref C.JSAtom
}

func (a Atom) Free() { C.JS_FreeAtom(a.ctx.ref, a.ref) }

func (a Atom) String() string {
	ptr := C.JS_AtomToCString(a.ctx.ref, a.ref)
	defer C.JS_FreeCString(a.ctx.ref, ptr)
	return C.GoString(ptr)
}

func (a Atom) Value() Value {
	return Value{ctx: a.ctx, ref: C.JS_AtomToValue(a.ctx.ref, a.ref)}
}

type Value struct {
	ctx *Context
	ref C.JSValue
}

func (v Value) Free() { C.JS_FreeValue(v.ctx.ref, v.ref) }

func (v Value) Context() *Context { return v.ctx }

func (v Value) Bool() bool { return C.JS_ToBool(v.ctx.ref, v.ref) == 1 }

func (v Value) String() string {
	ptr := C.JS_ToCString(v.ctx.ref, v.ref)
	defer C.JS_FreeCString(v.ctx.ref, ptr)
	return C.GoString(ptr)
}

func (v Value) Int64() int64 {
	val := C.int64_t(0)
	C.JS_ToInt64(v.ctx.ref, &val, v.ref)
	return int64(val)
}

func (v Value) Int32() int32 {
	val := C.int32_t(0)
	C.JS_ToInt32(v.ctx.ref, &val, v.ref)
	return int32(val)
}

func (v Value) Uint32() uint32 {
	val := C.uint32_t(0)
	C.JS_ToUint32(v.ctx.ref, &val, v.ref)
	return uint32(val)
}

func (v Value) Float64() float64 {
	val := C.double(0)
	C.JS_ToFloat64(v.ctx.ref, &val, v.ref)
	return float64(val)
}

func (v Value) BigInt() *big.Int {
	if !v.IsBigInt() {
		return nil
	}
	val, ok := new(big.Int).SetString(v.String(), 10)
	if !ok {
		return nil
	}
	return val
}

func (v Value) BigFloat() *big.Float {
	if !v.IsBigDecimal() && !v.IsBigFloat() {
		return nil
	}
	val, ok := new(big.Float).SetString(v.String())
	if !ok {
		return nil
	}
	return val
}

func (v Value) Get(name string) Value {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyStr(v.ctx.ref, v.ref, namePtr)}
}

func (v Value) GetByAtom(atom Atom) Value {
	return Value{ctx: v.ctx, ref: C.JS_GetProperty(v.ctx.ref, v.ref, atom.ref)}
}

func (v Value) GetByUint32(idx uint32) Value {
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx))}
}

func (v Value) SetByAtom(atom Atom, val Value) {
	C.JS_SetProperty(v.ctx.ref, v.ref, atom.ref, val.ref)
}

func (v Value) SetByInt64(idx int64, val Value) {
	C.JS_SetPropertyInt64(v.ctx.ref, v.ref, C.int64_t(idx), val.ref)
}

func (v Value) SetByUint32(idx uint32, val Value) {
	C.JS_SetPropertyUint32(v.ctx.ref, v.ref, C.uint32_t(idx), val.ref)
}

func (v Value) Len() int64 { return v.Get("length").Int64() }

func (v Value) Set(name string, val Value) {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	C.JS_SetPropertyStr(v.ctx.ref, v.ref, namePtr, val.ref)
}

func (v Value) SetFunction(name string, fn Function) {
	v.Set(name, v.ctx.Function(fn))
}

type Error struct {
	Cause string
	Stack string
}

func (err Error) Error() string { return err.Cause }

func (v Value) Error() error {
	if !v.IsError() {
		return nil
	}
	cause := v.String()

	stack := v.Get("stack")
	defer stack.Free()

	if stack.IsUndefined() {
		return &Error{Cause: cause}
	}
	return &Error{Cause: cause, Stack: stack.String()}
}

func (v Value) IsNumber() bool        { return C.JS_IsNumber(v.ref) == 1 }
func (v Value) IsBigInt() bool        { return C.JS_IsBigInt(v.ctx.ref, v.ref) == 1 }
func (v Value) IsBigFloat() bool      { return C.JS_IsBigFloat(v.ref) == 1 }
func (v Value) IsBigDecimal() bool    { return C.JS_IsBigDecimal(v.ref) == 1 }
func (v Value) IsBool() bool          { return C.JS_IsBool(v.ref) == 1 }
func (v Value) IsNull() bool          { return C.JS_IsNull(v.ref) == 1 }
func (v Value) IsUndefined() bool     { return C.JS_IsUndefined(v.ref) == 1 }
func (v Value) IsException() bool     { return C.JS_IsException(v.ref) == 1 }
func (v Value) IsUninitialized() bool { return C.JS_IsUninitialized(v.ref) == 1 }
func (v Value) IsString() bool        { return C.JS_IsString(v.ref) == 1 }
func (v Value) IsSymbol() bool        { return C.JS_IsSymbol(v.ref) == 1 }
func (v Value) IsObject() bool        { return C.JS_IsObject(v.ref) == 1 }
func (v Value) IsArray() bool         { return C.JS_IsArray(v.ctx.ref, v.ref) == 1 }

func (v Value) IsError() bool       { return C.JS_IsError(v.ctx.ref, v.ref) == 1 }
func (v Value) IsFunction() bool    { return C.JS_IsFunction(v.ctx.ref, v.ref) == 1 }
func (v Value) IsConstructor() bool { return C.JS_IsConstructor(v.ctx.ref, v.ref) == 1 }

func (v Value) PropertyNames() ([]PropertyEnum, error) {
	var (
		ptr  *C.JSPropertyEnum
		size C.uint32_t
	)

	result := int(C.JS_GetOwnPropertyNames(v.ctx.ref, &ptr, &size, v.ref, C.int(1<<0|1<<1|1<<2)))
	if result < 0 {
		return nil, errors.New("value does not contain properties")
	}
	defer C.js_free(v.ctx.ref, unsafe.Pointer(ptr))

	entries := (*[1 << 30]C.JSPropertyEnum)(unsafe.Pointer(ptr))

	names := make([]PropertyEnum, uint32(size))

	for i := 0; i < len(names); i++ {
		names[i].IsEnumerable = entries[i].is_enumerable == 1

		names[i].Atom = Atom{ctx: v.ctx, ref: entries[i].atom}
		names[i].Atom.Free()
	}

	return names, nil
}

type PropertyEnum struct {
	IsEnumerable bool
	Atom         Atom
}

func (p PropertyEnum) String() string { return p.Atom.String() }
