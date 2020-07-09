#include "_cgo_export.h"

JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv, int magic, JSValue *func_data) {
	 return proxy(ctx, this_val, argc, argv, magic, func_data);
}