package wasm

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

var undefined = &struct{}{}
var bridges = map[string]*Bridge{}
var mu sync.RWMutex

type context struct{ n string }

func setBridge(b *Bridge) unsafe.Pointer {
	mu.Lock()
	defer mu.Unlock()
	bridges[b.name] = b
	return unsafe.Pointer(&context{n: b.name})
}

func getBridge(ctx unsafe.Pointer) *Bridge {
	ictx := wasmer.IntoInstanceContext(ctx)
	c := (*(*context)(unsafe.Pointer(ictx.Data().(unsafe.Pointer))))
	mu.RLock()
	defer mu.RUnlock()
	return bridges[c.n]
}

type Bridge struct {
	name     string
	instance wasmer.Instance
	vmExit   bool
	exitCode os.Signal
	values   []interface{}
	refs     map[interface{}]int
}

func BridgeFromBytes(name string, bytes []byte, imports *wasmer.Imports) (*Bridge, error) {
	b := new(Bridge)
	if imports == nil {
		imports = wasmer.NewImports()
	}
	b.name = name
	err := b.addImports(imports)
	if err != nil {
		return nil, err
	}
	inst, err := wasmer.NewInstanceWithImports(bytes, imports)
	if err != nil {
		return nil, err
	}
	b.instance = inst
	inst.SetContextData(setBridge(b))
	b.addValues()
	b.refs = make(map[interface{}]int)
	return b, nil
}

func BridgeFromFile(name, file string, imports *wasmer.Imports) (*Bridge, error) {
	bytes, err := wasmer.ReadBytes(file)
	if err != nil {
		return nil, err
	}
	return BridgeFromBytes(name, bytes, imports)
}

func (b *Bridge) loadString(a, c int32) string {
	return string(b.mem()[a : a+c])
}

func (b *Bridge) addValues() {

	b.values = []interface{}{
		math.NaN(),
		float64(0),
		nil,
		true,
		false,
		&object{
			props: map[string]interface{}{
				"Object":     &object{name: "Object", props: map[string]interface{}{}},
				"Array":      &object{name: "Array", props: map[string]interface{}{}},
				"Uint8Array": typedArray("Uint8Array"),
				"process":    undefined,
				"document": &object{name: "document", props: map[string]interface{}{
					"cookie": "test=1",
				}},
				"window": &object{name: "window", props: map[string]interface{}{
					"crypto": &object{name: "crypto", props: map[string]interface{}{
						"getRandomValues": func(arr *stypedArray) *stypedArray {
							rand.Read(arr.Buffer.data)
							return arr
						},
					}},
				}},
				"navigator": &object{name: "navigator", props: map[string]interface{}{
					"userAgent": "Blabla",
				}},

				"fs": &object{name: "fs", props: map[string]interface{}{
					"constants": &object{name: "constants", props: map[string]interface{}{
						"O_WRONLY": syscall.O_WRONLY,
						"O_RDWR":   syscall.O_RDWR,
						"O_CREAT":  syscall.O_CREAT,
						"O_TRUNC":  syscall.O_TRUNC,
						"O_APPEND": syscall.O_APPEND,
						"O_EXCL":   syscall.O_EXCL,
					}},
				}},
			},
		},
	}
}
func (b *Bridge) Schedule() error {
	schedule := b.instance.Exports["go_scheduler"]
	_, err := schedule()
	if err != nil {
		log.Println(wasmer.GetLastError())
		return err
	}
	return nil
}

// Run start the wasm instance.
func (b *Bridge) Run() error {
	defer b.instance.Close()
	start := b.instance.Exports["_start"]
	_, err := start()
	if err != nil {
		log.Println(wasmer.GetLastError())
		return err
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigChan
		b.exitCode = s
	}()
	fmt.Printf("WASM exited with code: %v\n", b.exitCode)
	return nil
}

func (b *Bridge) mem() []byte {
	return b.instance.Memory.Data()
}

func (b *Bridge) storeString(addr int32, str string) {
	e := []byte(str)
	b.storeValue(addr, &e)
	b.setUint32(addr+8, uint32(len(e)))
	b.setUint32(addr+12, 0)
}

func (b *Bridge) storeBytes(addr, c int32, bytes []byte) {
	mem := b.mem()
	for i, b := range bytes {
		mem[int(addr)+i] = b
		if i == int(c) {
			break
		}
	}
}

func (b *Bridge) setInt64(offset int32, v int64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], uint64(v))
}

func (b *Bridge) setUint8(offset int32, v uint8) {
	mem := b.mem()
	mem[offset] = v
}

func (b *Bridge) getInt64(offset int32) int64 {
	mem := b.mem()
	return int64(binary.LittleEndian.Uint64(mem[offset:]))
}

func (b *Bridge) setUint32(offset int32, v uint32) {
	mem := b.mem()
	binary.LittleEndian.PutUint32(mem[offset:], v)
}

func (b *Bridge) setUint64(offset int32, v uint64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], v)
}

func (b *Bridge) getUnit64(offset int32) uint64 {
	mem := b.mem()
	return binary.LittleEndian.Uint64(mem[offset+0:])
}

func (b *Bridge) setFloat64(offset int32, v float64) {
	uf := math.Float64bits(v)
	b.setUint64(offset, uf)
}

func (b *Bridge) getFloat64(offset int32) float64 {
	uf := b.getUnit64(offset)
	return math.Float64frombits(uf)
}

func (b *Bridge) getInt32(offset int32) int32 {
	mem := b.mem()
	return int32(binary.LittleEndian.Uint32(mem[offset:]))
}

func (b *Bridge) getUint32(offset int32) uint32 {
	return binary.LittleEndian.Uint32(b.mem()[offset+0:])
}

func (b *Bridge) loadSlice(addr, c int32) []interface{} {
	z := make([]interface{}, c, c)
	for d := 0; d < int(c); d++ {
		_, bb := b.loadValue(addr + int32(d*8))
		z[d] = bb
	}
	return z
}

func (b *Bridge) loadValue(addr int32) (uint32, interface{}) {
	f := b.getFloat64(addr)
	if f == 0 {
		return 0, undefined
	}
	if !math.IsNaN(f) {
		return 0, f
	}
	id := b.getUint32(addr)
	return id, b.values[id]
}

func (b *Bridge) storeValue(addr int32, v interface{}) {
	const nanHead = 0x7FF80000

	if i, ok := v.(int); ok {
		v = float64(i)
	}

	if i, ok := v.(uint); ok {
		v = float64(i)
	}

	if v, ok := v.(float64); ok {
		if math.IsNaN(v) {
			b.setUint32(addr+4, nanHead)
			b.setUint32(addr, 0)
			return
		}

		if v == 0 {
			b.setUint32(addr+4, nanHead)
			b.setUint32(addr, 1)
			return
		}

		b.setFloat64(addr, v)
		return
	}

	switch v {
	case undefined:
		b.setFloat64(addr, 0)
		return
	case nil:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 2)
		return
	case true:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 3)
		return
	case false:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 4)
		return
	}
	ref, ok := b.refs[v]
	if !ok {
		ref = len(b.values)
		b.values = append(b.values, v)
		b.refs[v] = ref
	}

	typeFlag := 0
	t := reflect.Indirect(reflect.ValueOf(v))
	switch t.Kind() {
	case reflect.String:
		typeFlag = 1
	case reflect.Func:
		typeFlag = 3
	}
	b.setUint32(addr+4, uint32(nanHead|typeFlag))
	b.setUint32(addr, uint32(ref))
}

type object struct {
	name  string
	props map[string]interface{}
	new   func(args []interface{}) interface{}
}

func typedArray(name string) *object {
	return &object{
		name:  name,
		props: map[string]interface{}{},
		new: func(args []interface{}) interface{} {
			return &stypedArray{
				Buffer: &arrayBuffer{data: make([]byte, int(args[0].(float64)))},
			}
		},
	}
}

type arrayBuffer struct {
	data []byte
}

type stypedArray struct {
	Buffer *arrayBuffer
	Offset int
	Length int
}

func (a *stypedArray) toString() string {
	strs := make([]string, 0)
	for _, b := range a.Buffer.data {
		strs = append(strs, strconv.Itoa(int(b)))
	}
	return strings.Join(strs, ",")
}
