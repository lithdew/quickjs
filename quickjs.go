package quickjs

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"unsafe"
)

/*
#cgo CFLAGS: -D_GNU_SOURCE
#cgo CFLAGS: -DCONFIG_BIGNUM
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

func (r Runtime) Free() { C.JS_FreeRuntime(r.ref) }

func (r Runtime) NewContext() Context {
	ctx := Context{ref: C.JS_NewContext(r.ref)}
	C.JS_AddIntrinsicBigFloat(ctx.ref)
	C.JS_AddIntrinsicBigDecimal(ctx.ref)
	C.JS_AddIntrinsicOperators(ctx.ref)
	C.JS_EnableBignumExt(ctx.ref, C.int(1))
	return ctx
}

func (r Runtime) ExecutePendingJob() (Context, error) {
	var ctx Context
	if C.JS_ExecutePendingJob(r.ref, &ctx.ref) < 0 {
		return ctx, ctx.Exception().Error()
	}
	return ctx, nil
}

type Function func(ctx Context, this Value, args []Value) Value

var funcPtrLock sync.Mutex
var funcPtrStore = make(map[unsafe.Pointer]Function)
var funcPtrClassID C.JSClassID

func init() { C.JS_NewClassID(&funcPtrClassID) }

func storeFuncPtr(v Function) unsafe.Pointer {
	if v == nil {
		return nil
	}
	var ptr unsafe.Pointer = C.malloc(C.size_t(1))
	if ptr == nil {
		panic("failed to malloc pointer for function")
	}
	funcPtrLock.Lock()
	defer funcPtrLock.Unlock()
	funcPtrStore[ptr] = v
	return ptr
}

func restoreFuncPtr(ptr unsafe.Pointer) Function {
	if ptr == nil {
		return nil
	}
	funcPtrLock.Lock()
	defer funcPtrLock.Unlock()
	return funcPtrStore[ptr]
}

func freeFuncPtr(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	defer C.free(ptr)
	funcPtrLock.Lock()
	defer funcPtrLock.Unlock()
	delete(funcPtrStore, ptr)
}

//export proxy
func proxy(ctx *C.JSContext, thisVal C.JSValueConst, argc C.int, argv *C.JSValueConst, _ C.int, funcData *C.JSValue) C.JSValue {
	fn := restoreFuncPtr(C.JS_GetOpaque(*funcData, funcPtrClassID))
	refs := (*[1 << 30]C.JSValueConst)(unsafe.Pointer(argv))[:argc:argc]

	args := make([]Value, len(refs))
	for i := 0; i < len(args); i++ {
		args[i].ctx = ctx
		args[i].ref = refs[i]
	}

	return fn(Context{ref: ctx}, Value{ref: thisVal}, args).ref
}

type Context struct {
	ref *C.JSContext
}

func (c Context) Free() { C.JS_FreeContext(c.ref) }

func (c Context) Function(fp Function) Value {
	val := Value{ctx: c.ref, ref: C.JS_NewObjectClass(c.ref, C.int(funcPtrClassID))}
	if val.IsException() {
		return val
	}
	defer val.Free()

	funcPtr := storeFuncPtr(fp)
	C.JS_SetOpaque(val.ref, funcPtr)

	proxy := (*C.JSCFunctionData)(unsafe.Pointer(C.InvokeProxy))
	length := C.int(1)
	magic := C.int(0)

	return Value{ctx: c.ref, ref: C.JS_NewCFunctionData(c.ref, proxy, length, magic, C.int(1), &val.ref)}
}

func (c Context) Null() Value {
	return Value{ctx: c.ref, ref: C.JS_NewNull()}
}

func (c Context) Undefined() Value {
	return Value{ctx: c.ref, ref: C.JS_NewUndefined()}
}

func (c Context) Uninitialized() Value {
	return Value{ctx: c.ref, ref: C.JS_NewUninitialized()}
}

func (c Context) Error(err error) Value {
	val := Value{ctx: c.ref, ref: C.JS_NewError(c.ref)}
	val.Set("message", c.String(err.Error()))
	return val
}

func (c Context) Bool(b bool) Value {
	bv := 0
	if b {
		bv = 1
	}
	return Value{ctx: c.ref, ref: C.JS_NewBool(c.ref, C.int(bv))}
}

func (c Context) Int32(v int32) Value {
	return Value{ctx: c.ref, ref: C.JS_NewInt32(c.ref, C.int32_t(v))}
}

func (c Context) Int64(v int64) Value {
	return Value{ctx: c.ref, ref: C.JS_NewInt64(c.ref, C.int64_t(v))}
}

func (c Context) Uint32(v uint32) Value {
	return Value{ctx: c.ref, ref: C.JS_NewUint32(c.ref, C.uint32_t(v))}
}

func (c Context) BigUint64(v uint64) Value {
	return Value{ctx: c.ref, ref: C.JS_NewBigUint64(c.ref, C.uint64_t(v))}
}

func (c Context) Float64(v float64) Value {
	return Value{ctx: c.ref, ref: C.JS_NewFloat64(c.ref, C.double(v))}
}

func (c Context) String(v string) Value {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Value{ctx: c.ref, ref: C.JS_NewString(c.ref, ptr)}
}

func (c Context) Atom(v string) Atom {
	ptr := C.CString(v)
	defer C.free(unsafe.Pointer(ptr))
	return Atom{ctx: c.ref, ref: C.JS_NewAtom(c.ref, ptr)}
}

func (c Context) Eval(code string) (Value, error) { return c.EvalFile(code, "code") }

func (c Context) EvalFile(code, filename string) (Value, error) {
	codePtr := C.CString(code)
	defer C.free(unsafe.Pointer(codePtr))

	filenamePtr := C.CString(filename)
	defer C.free(unsafe.Pointer(filenamePtr))

	val := Value{ctx: c.ref, ref: C.JS_Eval(c.ref, codePtr, C.size_t(len(code)), filenamePtr, C.int(0))}
	if val.IsException() {
		return val, c.Exception().Error()
	}
	return val, nil
}

func (c Context) Globals() Value {
	return Value{ctx: c.ref, ref: C.JS_GetGlobalObject(c.ref)}
}

func (c Context) Throw(v Value) Value {
	return Value{ctx: c.ref, ref: C.JS_Throw(c.ref, v.ref)}
}

func (c Context) ThrowError(err error) Value { return c.Throw(c.Error(err)) }

func (c Context) ThrowSyntaxError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: c.ref, ref: C.ThrowSyntaxError(c.ref, causePtr)}
}

func (c Context) ThrowTypeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: c.ref, ref: C.ThrowTypeError(c.ref, causePtr)}
}

func (c Context) ThrowReferenceError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: c.ref, ref: C.ThrowReferenceError(c.ref, causePtr)}
}

func (c Context) ThrowRangeError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: c.ref, ref: C.ThrowRangeError(c.ref, causePtr)}
}

func (c Context) ThrowInternalError(format string, args ...interface{}) Value {
	cause := fmt.Sprintf(format, args...)
	causePtr := C.CString(cause)
	defer C.free(unsafe.Pointer(causePtr))
	return Value{ctx: c.ref, ref: C.ThrowInternalError(c.ref, causePtr)}
}

func (c Context) Exception() Value {
	return Value{ctx: c.ref, ref: C.JS_GetException(c.ref)}
}

func (c Context) Object() Value {
	return Value{ctx: c.ref, ref: C.JS_NewObject(c.ref)}
}

func (c Context) Array() Value {
	return Value{ctx: c.ref, ref: C.JS_NewArray(c.ref)}
}

type Atom struct {
	ctx *C.JSContext
	ref C.JSAtom
}

func (a Atom) String() string {
	return C.GoString(C.JS_AtomToCString(a.ctx, a.ref))
}

func (a Atom) Value() Value {
	return Value{ctx: a.ctx, ref: C.JS_AtomToValue(a.ctx, a.ref)}
}

type Value struct {
	ctx *C.JSContext
	ref C.JSValue
}

func (v Value) Free() { C.JS_FreeValue(v.ctx, v.ref) }

func (v Value) Context() Context { return Context{ref: v.ctx} }

func (v Value) Bool() bool { return C.JS_ToBool(v.ctx, v.ref) == 1 }

func (v Value) String() string {
	return C.GoString(C.JS_ToCString(v.ctx, v.ref))
}

func (v Value) Int64() int64 {
	val := C.int64_t(0)
	C.JS_ToInt64(v.ctx, &val, v.ref)
	return int64(val)
}

func (v Value) Int32() int32 {
	val := C.int32_t(0)
	C.JS_ToInt32(v.ctx, &val, v.ref)
	return int32(val)
}

func (v Value) Uint32() uint32 {
	val := C.uint32_t(0)
	C.JS_ToUint32(v.ctx, &val, v.ref)
	return uint32(val)
}

func (v Value) Float64() float64 {
	val := C.double(0)
	C.JS_ToFloat64(v.ctx, &val, v.ref)
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
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyStr(v.ctx, v.ref, namePtr)}
}

func (v Value) GetByAtom(atom Atom) Value {
	return Value{ctx: v.ctx, ref: C.JS_GetProperty(v.ctx, v.ref, atom.ref)}
}

func (v Value) GetByUint32(idx uint32) Value {
	return Value{ctx: v.ctx, ref: C.JS_GetPropertyUint32(v.ctx, v.ref, C.uint32_t(idx))}
}

func (v Value) SetByAtom(atom Atom, val Value) {
	C.JS_SetProperty(v.ctx, v.ref, atom.ref, val.ref)
}

func (v Value) SetByInt64(idx int64, val Value) {
	C.JS_SetPropertyInt64(v.ctx, v.ref, C.int64_t(idx), val.ref)
}

func (v Value) SetByUint32(idx uint32, val Value) {
	C.JS_SetPropertyUint32(v.ctx, v.ref, C.uint32_t(idx), val.ref)
}

func (v Value) Len() int64 { return v.Get("length").Int64() }

func (v Value) Set(name string, val Value) {
	namePtr := C.CString(name)
	defer C.free(unsafe.Pointer(namePtr))
	C.JS_SetPropertyStr(v.ctx, v.ref, namePtr, val.ref)
}

func (v Value) SetFunction(name string, fn Function) {
	v.Set(name, v.Context().Function(fn))
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
	if stack.IsUndefined() {
		return &Error{Cause: cause}
	}
	return &Error{Cause: cause, Stack: stack.String()}
}

func (v Value) IsNumber() bool        { return C.JS_IsNumber(v.ref) == 1 }
func (v Value) IsBigInt() bool        { return C.JS_IsBigInt(v.ctx, v.ref) == 1 }
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
func (v Value) IsArray() bool         { return C.JS_IsArray(v.ctx, v.ref) == 1 }

func (v Value) IsError() bool       { return C.JS_IsError(v.ctx, v.ref) == 1 }
func (v Value) IsFunction() bool    { return C.JS_IsFunction(v.ctx, v.ref) == 1 }
func (v Value) IsConstructor() bool { return C.JS_IsConstructor(v.ctx, v.ref) == 1 }

func (v Value) PropertyNames() ([]PropertyEnum, error) {
	var (
		ptr  *C.JSPropertyEnum
		size C.uint32_t
	)

	result := int(C.JS_GetOwnPropertyNames(v.ctx, &ptr, &size, v.ref, C.int(1<<0|1<<1|1<<2|1<<4|1<<5)))
	if result < 0 {
		return nil, errors.New("value does not contain properties")
	}

	entries := (*[1 << 30]C.JSPropertyEnum)(unsafe.Pointer(ptr))

	names := make([]PropertyEnum, uint32(size))

	for i := 0; i < len(names); i++ {
		names[i].IsEnumerable = entries[i].is_enumerable == 1
		names[i].Atom = Atom{ctx: v.ctx, ref: entries[i].atom}
	}

	return names, nil
}

type PropertyEnum struct {
	IsEnumerable bool
	Atom         Atom
}

func (p PropertyEnum) String() string { return p.Atom.String() }
