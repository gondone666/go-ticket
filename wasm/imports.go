package wasm

/* #include <stdlib.h>
extern void stringVal(void *context, int32_t a1, int32_t a2, int32_t a3, int32_t a4, int32_t a5);
extern void valueGet(void *context, int32_t a1, int32_t a2, int32_t a4, int32_t a5, int32_t a6, int32_t a7);
extern void valueSet(void *context, int32_t a1, int32_t a2, int32_t a4, int32_t a5, int32_t a6, int32_t a7);
extern void valueCall(void *context, int32_t a1, int32_t a2, int32_t a4, int32_t a5, int32_t a6, int32_t a7, int32_t a8, int32_t a9, int32_t a10);
extern void valueNew(void *context, int32_t a1, int32_t a2, int32_t a4, int32_t a5, int32_t a6, int32_t a7, int32_t a8);
extern int32_t valueLength(void *context, int32_t a1, int32_t a2, int32_t a3);
extern void valuePrepareString(void *context, int32_t a1, int32_t a2, int32_t a3, int32_t a4);
extern void valueLoadString(void *context, int32_t a1, int32_t a2, int32_t a3, int32_t a4, int32_t a5, int32_t a6);
extern double runtimeTicks(void *context);
extern void runtimeSleepTicks(void *context, double a);
*/
import "C"

import (
	"fmt"
	"log"
	"reflect"
	"time"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

func fdWrite(ctx unsafe.Pointer, fd int32, iovsPtr int32, iovsLen int32, nwrittenPtr int32) (err int32) {
	log.Fatal("fdWrite")
	return 0
}

//export stringVal
func stringVal(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5 int32) {
	fmt.Println("stringVal:", sp1, sp2, sp3, sp4, sp5)
	b := getBridge(ctx)
	str := b.loadString(sp2, sp3)
	b.storeValue(sp1, str)
}

//export valueGet
func valueGet(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5, sp6 int32) {
	fmt.Println("valueget", ctx, sp1, sp2, sp3, sp4, sp5, sp6)
	b := getBridge(ctx)
	str := b.loadString(sp3, sp4)
	log.Println("str:", str)
	id, val := b.loadValue(sp2)
	log.Println("id:", id, val)
	obj, ok := val.(*object)
	if !ok {
		log.Fatalln("Object conversion error", str, id, val)
	}
	res, ok := obj.props[str]
	if !ok {
		log.Fatalln("Missing object property", val, str)
	}
	log.Println("res:", res)
	b.storeValue(sp1, res)
}

//export valueSet
func valueSet(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5, sp6 int32) {
	fmt.Println("valueset", sp1, sp2, sp3, sp4, sp5, sp6)
	b := getBridge(ctx)
	_, obj := b.loadValue(sp1)
	log.Println("obj:", obj)
	str1 := b.loadString(sp2, sp3)
	log.Println("prop:", str1)
	_, str2 := b.loadValue(sp4)
	log.Println("value:", str2)
	obj.(*object).props[str1] = str2
}

func valueIndex(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5 int32) {
	log.Fatal("valueindex", sp1, sp2, sp3, sp4, sp5)
}

func valueSetIndex(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5 int32) {
	log.Fatal("valueSetIndex", sp1, sp2, sp3, sp4, sp5)
}

//export runtimeTicks
func runtimeTicks(ctx unsafe.Pointer) float64 {
	t := float64(time.Now().UnixNano() / int64(time.Millisecond/time.Nanosecond))
	log.Println("runtimeTicks:t", t)
	return t
}

//export runtimeSleepTicks
func runtimeSleepTicks(ctx unsafe.Pointer, a float64) {
	log.Println("runtimeSleepTicks", a)
	b := getBridge(ctx)
	time.Sleep(time.Duration(a) * time.Millisecond)
	b.Schedule()
}

//export valueCall
func valueCall(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5, sp6, sp7, sp8, sp9 int32) {
	fmt.Println("valuecall", sp1, sp2, sp3, sp4, sp5, sp6, sp7, sp8, sp9)
	b := getBridge(ctx)
	_, val := b.loadValue(sp2)
	log.Println("val:", val)
	str := b.loadString(sp3, sp4)
	log.Println("method:", str)
	args := b.loadSlice(sp5, sp6)
	if reflect.TypeOf(val) == reflect.TypeOf(&object{}) {
		log.Println("args:", args[0].(*stypedArray))
		obj := val.(*object)
		log.Println("obj:", obj)
		var pfunc func(*stypedArray) *stypedArray
		pfunc = obj.props[str].(func(*stypedArray) *stypedArray)
		res := pfunc(args[0].(*stypedArray))
		b.storeValue(sp1, res)
		b.setUint8(sp1+8, 1)
	} else if reflect.TypeOf(val) == reflect.TypeOf(&stypedArray{}) {
		obj := val.(*stypedArray)
		res := obj.toString()
		log.Println("res:", res)
		b.storeValue(sp1, res)
		b.setUint8(sp1+8, 1)
	}
}

//export valueNew
func valueNew(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5, sp6, sp7 int32) {
	fmt.Println("valuenew", sp1, sp2, sp3, sp4, sp5, sp6, sp7)
	b := getBridge(ctx)
	id, val := b.loadValue(sp2)
	log.Println("id,val:", id, val)
	args := b.loadSlice(sp3, sp4)
	log.Println("args:", args)
	obj, ok := val.(*object)
	if !ok {
		log.Fatal("val is not an object", val)
	}
	log.Println("obj:", obj)
	ret := obj.new(args)
	log.Println("ret:", ret)
	b.storeValue(sp1, ret)
	b.setUint8(sp1+8, 1)
}

func valueLength(ctx unsafe.Pointer, sp1, sp2, sp3 int32) int32 {
	log.Fatal("valuelength", sp1, sp2, sp3)
	return 0
}

//export valuePrepareString
func valuePrepareString(ctx unsafe.Pointer, sp1, sp2, sp3, sp4 int32) {
	fmt.Println("valuePrepareString", sp1, sp2, sp3, sp4)
	b := getBridge(ctx)
	_, v := b.loadValue(sp2)
	log.Println(v)
	b.storeString(sp1, v.(string))
}

//export valueLoadString
func valueLoadString(ctx unsafe.Pointer, sp1, sp2, sp3, sp4, sp5, sp6 int32) {
	fmt.Println("valueLoadString", sp1, sp2, sp3, sp4, sp5, sp6)
	b := getBridge(ctx)
	_, str := b.loadValue(sp1)
	log.Println("str:", str)
	b.storeBytes(sp3, sp4, *str.(*[]byte))
}

// addImports adds go Bridge imports in "go" namespace.
func (b *Bridge) addImports(imps *wasmer.Imports) error {

	var is = []struct {
		space string
		name  string
		imp   interface{}
		cgo   unsafe.Pointer
	}{
		{"wasi_unstable", "fd_write", fdWrite, nil},
		{"env", "runtime.ticks", runtimeTicks, C.runtimeTicks},
		{"env", "runtime.sleepTicks", runtimeSleepTicks, C.runtimeSleepTicks},
		{"env", "syscall/js.stringVal", stringVal, C.stringVal},
		{"env", "syscall/js.valueGet", valueGet, C.valueGet},
		{"env", "syscall/js.valueSet", valueSet, C.valueSet},
		{"env", "syscall/js.valueIndex", valueIndex, nil},
		{"env", "syscall/js.valueSetIndex", valueSetIndex, nil},
		{"env", "syscall/js.valueCall", valueCall, C.valueCall},
		{"env", "syscall/js.valueNew", valueNew, C.valueNew},
		{"env", "syscall/js.valueLength", valueLength, nil},
		{"env", "syscall/js.valuePrepareString", valuePrepareString, C.valuePrepareString},
		{"env", "syscall/js.valueLoadString", valueLoadString, C.valueLoadString},
	}
	for _, imp := range is {
		imps = imps.Namespace(imp.space)
		imps, _ = imps.Append(imp.name, imp.imp, imp.cgo)
	}
	return nil
}
