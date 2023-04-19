package js

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/dop251/goja"
	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/zkevm/log"
	"github.com/ledgerwatch/erigon/zkevm/state/runtime/fakevm"
	jsassets "github.com/ledgerwatch/erigon/zkevm/state/runtime/instrumentation/js/internal/tracers"
	"github.com/ledgerwatch/erigon/zkevm/state/runtime/instrumentation/tracers"
)

var assetTracers = make(map[string]string)

// init retrieves the JavaScript transaction tracers included in go-ethereum.
func init() {
	var err error
	assetTracers, err = jsassets.Load()
	if err != nil {
		panic(err)
	}
	tracers.RegisterLookup(true, NewJsTracer)
}

// bigIntProgram is compiled once and the exported function mostly invoked to convert
// hex strings into big ints.
var bigIntProgram = goja.MustCompile("bigInt", bigIntegerJS, false)

type toBigFn = func(vm *goja.Runtime, val string) (goja.Value, error)
type toBufFn = func(vm *goja.Runtime, val []byte) (goja.Value, error)
type fromBufFn = func(vm *goja.Runtime, buf goja.Value, allowString bool) ([]byte, error)

func toBuf(vm *goja.Runtime, bufType goja.Value, val []byte) (goja.Value, error) {
	// bufType is usually Uint8Array. This is equivalent to `new Uint8Array(val)` in JS.
	res, err := vm.New(bufType, vm.ToValue(val))
	if err != nil {
		return nil, err
	}
	return vm.ToValue(res), nil
}

func fromBuf(vm *goja.Runtime, bufType goja.Value, buf goja.Value, allowString bool) ([]byte, error) {
	obj := buf.ToObject(vm)
	switch obj.ClassName() {
	case "String":
		if !allowString {
			break
		}
		return common.FromHex(obj.String()), nil
	case "Array":
		var b []byte
		if err := vm.ExportTo(buf, &b); err != nil {
			return nil, err
		}
		return b, nil

	case "Object":
		if !obj.Get("constructor").SameAs(bufType) {
			break
		}
		var b []byte
		if err := vm.ExportTo(buf, &b); err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, fmt.Errorf("invalid buffer type")
}

type jsTracer struct {
	vm                *goja.Runtime
	env               *fakevm.FakeEVM
	toBig             toBigFn               // Converts a hex string into a JS bigint
	toBuf             toBufFn               // Converts a []byte into a JS buffer
	fromBuf           fromBufFn             // Converts an array, hex string or Uint8Array to a []byte
	ctx               map[string]goja.Value // KV-bag passed to JS in `result`
	activePrecompiles []common.Address      // List of active precompiles at current block
	traceStep         bool                  // True if tracer object exposes a `step()` method
	traceFrame        bool                  // True if tracer object exposes the `enter()` and `exit()` methods
	gasLimit          uint64                // Amount of gas bought for the whole tx
	err               error                 // Any error that should stop tracing
	obj               *goja.Object          // Trace object

	// Methods exposed by tracer
	result goja.Callable
	fault  goja.Callable
	step   goja.Callable
	enter  goja.Callable
	exit   goja.Callable

	// Underlying structs being passed into JS
	log         *steplog
	frame       *callframe
	frameResult *callframeResult

	// Goja-wrapping of types prepared for JS consumption
	logValue         goja.Value
	dbValue          goja.Value
	frameValue       goja.Value
	frameResultValue goja.Value
}

// NewJsTracer is the JS tracer constructor.
func NewJsTracer(code string, ctx *tracers.Context) (tracers.Tracer, error) {
	if c, ok := assetTracers[code]; ok {
		code = c
	}
	vm := goja.New()
	// By default field names are exported to JS as is, i.e. capitalized.
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	t := &jsTracer{
		vm:  vm,
		ctx: make(map[string]goja.Value),
	}
	if ctx == nil {
		ctx = new(tracers.Context)
	}
	if ctx.BlockHash != (common.Hash{}) {
		t.ctx["blockHash"] = vm.ToValue(ctx.BlockHash.Bytes())
		if ctx.TxHash != (common.Hash{}) {
			t.ctx["txIndex"] = vm.ToValue(ctx.TxIndex)
			t.ctx["txHash"] = vm.ToValue(ctx.TxHash.Bytes())
		}
	}

	err := t.setTypeConverters()
	if err != nil {
		return nil, err
	}
	t.setBuiltinFunctions()
	ret, err := vm.RunString("(" + code + ")")
	if err != nil {
		return nil, err
	}
	// Check tracer's interface for required and optional methods.
	obj := ret.ToObject(vm)
	result, ok := goja.AssertFunction(obj.Get("result"))
	if !ok {
		return nil, errors.New("trace object must expose a function result()")
	}
	fault, ok := goja.AssertFunction(obj.Get("fault"))
	if !ok {
		return nil, errors.New("trace object must expose a function fault()")
	}
	step, ok := goja.AssertFunction(obj.Get("step"))
	t.traceStep = ok
	enter, hasEnter := goja.AssertFunction(obj.Get("enter"))
	exit, hasExit := goja.AssertFunction(obj.Get("exit"))
	if hasEnter != hasExit {
		return nil, errors.New("trace object must expose either both or none of enter() and exit()")
	}
	t.traceFrame = hasEnter
	t.obj = obj
	t.step = step
	t.enter = enter
	t.exit = exit
	t.result = result
	t.fault = fault
	// Setup objects carrying data to JS. These are created once and re-used.
	t.log = &steplog{
		vm:       vm,
		op:       &opObj{vm: vm},
		memory:   &memoryObj{vm: vm, toBig: t.toBig, toBuf: t.toBuf},
		stack:    &stackObj{vm: vm, toBig: t.toBig},
		contract: &contractObj{vm: vm, toBig: t.toBig, toBuf: t.toBuf},
	}
	t.frame = &callframe{vm: vm, toBig: t.toBig, toBuf: t.toBuf}
	t.frameResult = &callframeResult{vm: vm, toBuf: t.toBuf}
	t.frameValue = t.frame.setupObject()
	t.frameResultValue = t.frameResult.setupObject()
	t.logValue = t.log.setupObject()
	return t, nil
}

// CaptureTxStart implements the Tracer interface and is invoked at the beginning of
// transaction processing.
func (t *jsTracer) CaptureTxStart(gasLimit uint64) {
	t.gasLimit = gasLimit
}

// CaptureTxEnd implements the Tracer interface and is invoked at the end of
// transaction processing.
func (t *jsTracer) CaptureTxEnd(restGas uint64) {}

// CaptureStart implements the Tracer interface to initialize the tracing operation.
func (t *jsTracer) CaptureStart(env *fakevm.FakeEVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	t.env = env
	db := &dbObj{db: env.StateDB, vm: t.vm, toBig: t.toBig, toBuf: t.toBuf, fromBuf: t.fromBuf}
	t.dbValue = db.setupObject()
	if create {
		t.ctx["type"] = t.vm.ToValue("CREATE")
	} else {
		t.ctx["type"] = t.vm.ToValue("CALL")
	}
	t.ctx["from"] = t.vm.ToValue(from.Bytes())
	t.ctx["to"] = t.vm.ToValue(to.Bytes())
	t.ctx["input"] = t.vm.ToValue(input)
	t.ctx["gas"] = t.vm.ToValue(gas)
	t.ctx["gasPrice"] = t.vm.ToValue(env.TxContext.GasPrice)
	valueBig, err := t.toBig(t.vm, value.String())
	if err != nil {
		t.err = err
		return
	}
	t.ctx["value"] = valueBig
	t.ctx["block"] = t.vm.ToValue(env.Context.BlockNumber.Uint64())
	// Update list of precompiles based on current block
	rules := env.ChainConfig().Rules(env.Context.BlockNumber, env.Context.Random != nil, env.Context.Time)
	t.activePrecompiles = vm.ActivePrecompiles(rules)
	t.ctx["intrinsicGas"] = t.vm.ToValue(t.gasLimit - gas)
}

// CaptureState implements the Tracer interface to trace a single step of VM execution.
func (t *jsTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *fakevm.ScopeContext, rData []byte, depth int, err error) {
	if !t.traceStep {
		return
	}
	if t.err != nil {
		return
	}

	log := t.log
	log.op.op = op
	log.memory.memory = scope.Memory
	log.stack.stack = scope.Stack
	log.contract.contract = scope.Contract
	log.pc = uint(pc)
	log.gas = uint(gas)
	log.cost = uint(cost)
	log.depth = uint(depth)
	log.err = err
	if _, err := t.step(t.obj, t.logValue, t.dbValue); err != nil {
		t.onError("step", err)
	}
}

// CaptureFault implements the Tracer interface to trace an execution fault
func (t *jsTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *fakevm.ScopeContext, depth int, err error) {
	if t.err != nil {
		return
	}
	// Other log fields have been already set as part of the last CaptureState.
	t.log.err = err
	if _, err := t.fault(t.obj, t.logValue, t.dbValue); err != nil {
		t.onError("fault", err)
	}
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *jsTracer) CaptureEnd(output []byte, gasUsed uint64, duration time.Duration, err error) {
	t.ctx["output"] = t.vm.ToValue(output)
	t.ctx["time"] = t.vm.ToValue(duration.String())
	t.ctx["gasUsed"] = t.vm.ToValue(gasUsed)
	if err != nil {
		t.ctx["error"] = t.vm.ToValue(err.Error())
	}
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *jsTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	if !t.traceFrame {
		return
	}
	if t.err != nil {
		return
	}

	t.frame.typ = typ.String()
	t.frame.from = from
	t.frame.to = to
	t.frame.input = common.CopyBytes(input)
	t.frame.gas = uint(gas)
	t.frame.value = nil
	if value != nil {
		t.frame.value = new(big.Int).SetBytes(value.Bytes())
	}

	if _, err := t.enter(t.obj, t.frameValue); err != nil {
		t.onError("enter", err)
	}
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (t *jsTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
	if !t.traceFrame {
		return
	}

	t.frameResult.gasUsed = uint(gasUsed)
	t.frameResult.output = common.CopyBytes(output)
	t.frameResult.err = err

	if _, err := t.exit(t.obj, t.frameResultValue); err != nil {
		t.onError("exit", err)
	}
}

// GetResult calls the Javascript 'result' function and returns its value, or any accumulated error
func (t *jsTracer) GetResult() (json.RawMessage, error) {
	ctx := t.vm.ToValue(t.ctx)
	res, err := t.result(t.obj, ctx, t.dbValue)
	if err != nil {
		return nil, wrapError("result", err)
	}
	encoded, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), t.err
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *jsTracer) Stop(err error) {
	t.vm.Interrupt(err)
}

// onError is called anytime the running JS code is interrupted
// and returns an error. It in turn pings the EVM to cancel its
// execution.
func (t *jsTracer) onError(context string, err error) {
	t.err = wrapError(context, err)
	// `env` is set on CaptureStart which comes before any JS execution.
	// So it should be non-nil.
	t.env.Cancel()
}

func wrapError(context string, err error) error {
	return fmt.Errorf("%v    in server-side tracer function '%v'", err, context)
}

// setBuiltinFunctions injects Go functions which are available to tracers into the environment.
// It depends on type converters having been set up.
func (t *jsTracer) setBuiltinFunctions() {
	vm := t.vm
	// TODO: load console from goja-nodejs
	err := vm.Set("toHex", func(v goja.Value) string {
		b, err := t.fromBuf(vm, v, false)
		if err != nil {
			vm.Interrupt(err)
			return ""
		}
		return hexutil.Encode(b)
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("toWord", func(v goja.Value) goja.Value {
		// TODO: add test with []byte len < 32 or > 32
		b, err := t.fromBuf(vm, v, true)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		b = common.BytesToHash(b).Bytes()
		res, err := t.toBuf(vm, b)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		return res
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("toAddress", func(v goja.Value) goja.Value {
		a, err := t.fromBuf(vm, v, true)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		a = common.BytesToAddress(a).Bytes()
		res, err := t.toBuf(vm, a)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		return res
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("toContract", func(from goja.Value, nonce uint) goja.Value {
		a, err := t.fromBuf(vm, from, true)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		addr := common.BytesToAddress(a)
		b := crypto.CreateAddress(addr, uint64(nonce)).Bytes()
		res, err := t.toBuf(vm, b)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		return res
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("toContract2", func(from goja.Value, salt string, initcode goja.Value) goja.Value {
		a, err := t.fromBuf(vm, from, true)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		addr := common.BytesToAddress(a)
		code, err := t.fromBuf(vm, initcode, true)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		code = common.CopyBytes(code)
		codeHash := crypto.Keccak256(code)
		b := crypto.CreateAddress2(addr, common.HexToHash(salt), codeHash).Bytes()
		res, err := t.toBuf(vm, b)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		return res
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("isPrecompiled", func(v goja.Value) bool {
		a, err := t.fromBuf(vm, v, true)
		if err != nil {
			vm.Interrupt(err)
			return false
		}
		addr := common.BytesToAddress(a)
		for _, p := range t.activePrecompiles {
			if p == addr {
				return true
			}
		}
		return false
	})
	if err != nil {
		log.Error(err)
	}
	err = vm.Set("slice", func(slice goja.Value, start, end int) goja.Value {
		b, err := t.fromBuf(vm, slice, false)
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		if start < 0 || start > end || end > len(b) {
			vm.Interrupt(fmt.Sprintf("Tracer accessed out of bound memory: available %d, offset %d, size %d", len(b), start, end-start))
			return nil
		}
		res, err := t.toBuf(vm, b[start:end])
		if err != nil {
			vm.Interrupt(err)
			return nil
		}
		return res
	})
	if err != nil {
		log.Error(err)
	}
}

// setTypeConverters sets up utilities for converting Go types into those
// suitable for JS consumption.
func (t *jsTracer) setTypeConverters() error {
	// Inject bigint logic.
	// TODO: To be replaced after goja adds support for native JS bigint.
	toBigCode, err := t.vm.RunProgram(bigIntProgram)
	if err != nil {
		return err
	}
	// Used to create JS bigint objects from go.
	toBigFn, ok := goja.AssertFunction(toBigCode)
	if !ok {
		return errors.New("failed to bind bigInt func")
	}
	toBigWrapper := func(vm *goja.Runtime, val string) (goja.Value, error) {
		return toBigFn(goja.Undefined(), vm.ToValue(val))
	}
	t.toBig = toBigWrapper
	// NOTE: We need this workaround to create JS buffers because
	// goja doesn't at the moment expose constructors for typed arrays.
	//
	// Cache uint8ArrayType once to be used every time for less overhead.
	uint8ArrayType := t.vm.Get("Uint8Array")
	toBufWrapper := func(vm *goja.Runtime, val []byte) (goja.Value, error) {
		return toBuf(vm, uint8ArrayType, val)
	}
	t.toBuf = toBufWrapper
	fromBufWrapper := func(vm *goja.Runtime, buf goja.Value, allowString bool) ([]byte, error) {
		return fromBuf(vm, uint8ArrayType, buf, allowString)
	}
	t.fromBuf = fromBufWrapper
	return nil
}

type opObj struct {
	vm *goja.Runtime
	op vm.OpCode
}

func (o *opObj) ToNumber() int {
	return int(o.op)
}

func (o *opObj) ToString() string {
	return o.op.String()
}

func (o *opObj) IsPush() bool {
	return o.op.IsPush()
}

func (o *opObj) setupObject() *goja.Object {
	obj := o.vm.NewObject()
	err := obj.Set("toNumber", o.vm.ToValue(o.ToNumber))
	if err != nil {
		log.Error(err)
	}
	err = obj.Set("toString", o.vm.ToValue(o.ToString))
	if err != nil {
		log.Error(err)
	}
	err = obj.Set("isPush", o.vm.ToValue(o.IsPush))
	if err != nil {
		log.Error(err)
	}
	return obj
}

type memoryObj struct {
	memory *fakevm.Memory
	vm     *goja.Runtime
	toBig  toBigFn
	toBuf  toBufFn
}

func (mo *memoryObj) Slice(begin, end int64) goja.Value {
	b, err := mo.slice(begin, end)
	if err != nil {
		mo.vm.Interrupt(err)
		return nil
	}
	res, err := mo.toBuf(mo.vm, b)
	if err != nil {
		mo.vm.Interrupt(err)
		return nil
	}
	return res
}

// slice returns the requested range of memory as a byte slice.
func (mo *memoryObj) slice(begin, end int64) ([]byte, error) {
	if end == begin {
		return []byte{}, nil
	}
	if end < begin || begin < 0 {
		return nil, fmt.Errorf("Tracer accessed out of bound memory: offset %d, end %d", begin, end)
	}
	if mo.memory.Len() < int(end) {
		return nil, fmt.Errorf("Tracer accessed out of bound memory: available %d, offset %d, size %d", mo.memory.Len(), begin, end-begin)
	}
	return mo.memory.GetCopy(begin, end-begin), nil
}

func (mo *memoryObj) GetUint(addr int64) goja.Value {
	value, err := mo.getUint(addr)
	if err != nil {
		mo.vm.Interrupt(err)
		return nil
	}
	res, err := mo.toBig(mo.vm, value.String())
	if err != nil {
		mo.vm.Interrupt(err)
		return nil
	}
	return res
}

// getUint returns the 32 bytes at the specified address interpreted as a uint.
func (mo *memoryObj) getUint(addr int64) (*big.Int, error) {
	if mo.memory.Len() < int(addr)+32 || addr < 0 {
		return nil, fmt.Errorf("Tracer accessed out of bound memory: available %d, offset %d, size %d", mo.memory.Len(), addr, fakevm.MemoryItemSize)
	}
	return new(big.Int).SetBytes(mo.memory.GetPtr(addr, int64(fakevm.MemoryItemSize))), nil
}

func (mo *memoryObj) Length() int {
	return mo.memory.Len()
}

func (mo *memoryObj) setupObject() *goja.Object {
	o := mo.vm.NewObject()
	err := o.Set("slice", mo.vm.ToValue(mo.Slice))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getUint", mo.vm.ToValue(mo.GetUint))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("length", mo.vm.ToValue(mo.Length))
	if err != nil {
		log.Error(err)
	}
	return o
}

type stackObj struct {
	stack *fakevm.Stack
	vm    *goja.Runtime
	toBig toBigFn
}

func (s *stackObj) Peek(idx int) goja.Value {
	value, err := s.peek(idx)
	if err != nil {
		s.vm.Interrupt(err)
		return nil
	}
	res, err := s.toBig(s.vm, value.String())
	if err != nil {
		s.vm.Interrupt(err)
		return nil
	}
	return res
}

// peek returns the nth-from-the-top element of the stack.
func (s *stackObj) peek(idx int) (*big.Int, error) {
	if len(s.stack.Data()) <= idx || idx < 0 {
		return nil, fmt.Errorf("Tracer accessed out of bound stack: size %d, index %d", len(s.stack.Data()), idx)
	}
	return s.stack.Back(idx).ToBig(), nil
}

func (s *stackObj) Length() int {
	return len(s.stack.Data())
}

func (s *stackObj) setupObject() *goja.Object {
	o := s.vm.NewObject()
	err := o.Set("peek", s.vm.ToValue(s.Peek))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("length", s.vm.ToValue(s.Length))
	if err != nil {
		log.Error(err)
	}
	return o
}

type dbObj struct {
	db      fakevm.FakeDB
	vm      *goja.Runtime
	toBig   toBigFn
	toBuf   toBufFn
	fromBuf fromBufFn
}

func (do *dbObj) GetBalance(addrSlice goja.Value) goja.Value {
	a, err := do.fromBuf(do.vm, addrSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	addr := common.BytesToAddress(a)
	value := do.db.GetBalance(addr)
	res, err := do.toBig(do.vm, value.String())
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	return res
}

func (do *dbObj) GetNonce(addrSlice goja.Value) uint64 {
	a, err := do.fromBuf(do.vm, addrSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return 0
	}
	addr := common.BytesToAddress(a)
	return do.db.GetNonce(addr)
}

func (do *dbObj) GetCode(addrSlice goja.Value) goja.Value {
	a, err := do.fromBuf(do.vm, addrSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	addr := common.BytesToAddress(a)
	code := do.db.GetCode(addr)
	res, err := do.toBuf(do.vm, code)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	return res
}

func (do *dbObj) GetState(addrSlice goja.Value, hashSlice goja.Value) goja.Value {
	a, err := do.fromBuf(do.vm, addrSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	addr := common.BytesToAddress(a)
	h, err := do.fromBuf(do.vm, hashSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	hash := common.BytesToHash(h)
	state := do.db.GetState(addr, hash).Bytes()
	res, err := do.toBuf(do.vm, state)
	if err != nil {
		do.vm.Interrupt(err)
		return nil
	}
	return res
}

func (do *dbObj) Exists(addrSlice goja.Value) bool {
	a, err := do.fromBuf(do.vm, addrSlice, false)
	if err != nil {
		do.vm.Interrupt(err)
		return false
	}
	addr := common.BytesToAddress(a)
	return do.db.Exist(addr)
}

func (do *dbObj) setupObject() *goja.Object {
	o := do.vm.NewObject()
	err := o.Set("getBalance", do.vm.ToValue(do.GetBalance))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getNonce", do.vm.ToValue(do.GetNonce))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getCode", do.vm.ToValue(do.GetCode))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getState", do.vm.ToValue(do.GetState))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("exists", do.vm.ToValue(do.Exists))
	if err != nil {
		log.Error(err)
	}
	return o
}

type contractObj struct {
	contract *vm.Contract
	vm       *goja.Runtime
	toBig    toBigFn
	toBuf    toBufFn
}

func (co *contractObj) GetCaller() goja.Value {
	caller := co.contract.Caller().Bytes()
	res, err := co.toBuf(co.vm, caller)
	if err != nil {
		co.vm.Interrupt(err)
		return nil
	}
	return res
}

func (co *contractObj) GetAddress() goja.Value {
	addr := co.contract.Address().Bytes()
	res, err := co.toBuf(co.vm, addr)
	if err != nil {
		co.vm.Interrupt(err)
		return nil
	}
	return res
}

func (co *contractObj) GetValue() goja.Value {
	value := co.contract.Value()
	res, err := co.toBig(co.vm, value.String())
	if err != nil {
		co.vm.Interrupt(err)
		return nil
	}
	return res
}

func (co *contractObj) GetInput() goja.Value {
	input := co.contract.Input
	res, err := co.toBuf(co.vm, input)
	if err != nil {
		co.vm.Interrupt(err)
		return nil
	}
	return res
}

func (co *contractObj) setupObject() *goja.Object {
	o := co.vm.NewObject()
	err := o.Set("getCaller", co.vm.ToValue(co.GetCaller))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getAddress", co.vm.ToValue(co.GetAddress))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getValue", co.vm.ToValue(co.GetValue))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getInput", co.vm.ToValue(co.GetInput))
	if err != nil {
		log.Error(err)
	}
	return o
}

type callframe struct {
	vm    *goja.Runtime
	toBig toBigFn
	toBuf toBufFn

	typ   string
	from  common.Address
	to    common.Address
	input []byte
	gas   uint
	value *big.Int
}

func (f *callframe) GetType() string {
	return f.typ
}

func (f *callframe) GetFrom() goja.Value {
	from := f.from.Bytes()
	res, err := f.toBuf(f.vm, from)
	if err != nil {
		f.vm.Interrupt(err)
		return nil
	}
	return res
}

func (f *callframe) GetTo() goja.Value {
	to := f.to.Bytes()
	res, err := f.toBuf(f.vm, to)
	if err != nil {
		f.vm.Interrupt(err)
		return nil
	}
	return res
}

func (f *callframe) GetInput() goja.Value {
	input := f.input
	res, err := f.toBuf(f.vm, input)
	if err != nil {
		f.vm.Interrupt(err)
		return nil
	}
	return res
}

func (f *callframe) GetGas() uint {
	return f.gas
}

func (f *callframe) GetValue() goja.Value {
	if f.value == nil {
		return goja.Undefined()
	}
	res, err := f.toBig(f.vm, f.value.String())
	if err != nil {
		f.vm.Interrupt(err)
		return nil
	}
	return res
}

func (f *callframe) setupObject() *goja.Object {
	o := f.vm.NewObject()
	err := o.Set("getType", f.vm.ToValue(f.GetType))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getFrom", f.vm.ToValue(f.GetFrom))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getTo", f.vm.ToValue(f.GetTo))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getInput", f.vm.ToValue(f.GetInput))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getGas", f.vm.ToValue(f.GetGas))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getValue", f.vm.ToValue(f.GetValue))
	if err != nil {
		log.Error(err)
	}
	return o
}

type callframeResult struct {
	vm    *goja.Runtime
	toBuf toBufFn

	gasUsed uint
	output  []byte
	err     error
}

func (r *callframeResult) GetGasUsed() uint {
	return r.gasUsed
}

func (r *callframeResult) GetOutput() goja.Value {
	res, err := r.toBuf(r.vm, r.output)
	if err != nil {
		r.vm.Interrupt(err)
		return nil
	}
	return res
}

func (r *callframeResult) GetError() goja.Value {
	if r.err != nil {
		return r.vm.ToValue(r.err.Error())
	}
	return goja.Undefined()
}

func (r *callframeResult) setupObject() *goja.Object {
	o := r.vm.NewObject()
	err := o.Set("getGasUsed", r.vm.ToValue(r.GetGasUsed))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getOutput", r.vm.ToValue(r.GetOutput))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getError", r.vm.ToValue(r.GetError))
	if err != nil {
		log.Error(err)
	}
	return o
}

type steplog struct {
	vm *goja.Runtime

	op       *opObj
	memory   *memoryObj
	stack    *stackObj
	contract *contractObj

	pc     uint
	gas    uint
	cost   uint
	depth  uint
	refund uint
	err    error
}

func (l *steplog) GetPC() uint {
	return l.pc
}

func (l *steplog) GetGas() uint {
	return l.gas
}

func (l *steplog) GetCost() uint {
	return l.cost
}

func (l *steplog) GetDepth() uint {
	return l.depth
}

func (l *steplog) GetRefund() uint {
	return l.refund
}

func (l *steplog) GetError() goja.Value {
	if l.err != nil {
		return l.vm.ToValue(l.err.Error())
	}
	return goja.Undefined()
}

func (l *steplog) setupObject() *goja.Object {
	o := l.vm.NewObject()
	// Setup basic fields.
	err := o.Set("getPC", l.vm.ToValue(l.GetPC))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getGas", l.vm.ToValue(l.GetGas))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getCost", l.vm.ToValue(l.GetCost))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getDepth", l.vm.ToValue(l.GetDepth))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getRefund", l.vm.ToValue(l.GetRefund))
	if err != nil {
		log.Error(err)
	}
	err = o.Set("getError", l.vm.ToValue(l.GetError))
	if err != nil {
		log.Error(err)
	}
	// Setup nested objects.
	err = o.Set("op", l.op.setupObject())
	if err != nil {
		log.Error(err)
	}
	err = o.Set("stack", l.stack.setupObject())
	if err != nil {
		log.Error(err)
	}
	err = o.Set("memory", l.memory.setupObject())
	if err != nil {
		log.Error(err)
	}
	err = o.Set("contract", l.contract.setupObject())
	if err != nil {
		log.Error(err)
	}
	return o
}
