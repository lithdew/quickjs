#include "_cgo_export.h"

JSValue InvokeProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
	 return proxy(ctx, this_val, argc, argv);
}