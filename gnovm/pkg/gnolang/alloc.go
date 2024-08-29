package gnolang

import (
	"fmt"
	"reflect"
)

// Keeps track of in-memory allocations.
// In the future, allocations within realm boundaries will be
// (optionally?) condensed (objects to be GC'd will be discarded),
// but for now, allocations strictly increment across the whole tx.
type Allocator struct {
	maxBytes int64
	bytes    int64
	heap     *Heap
}

// for gonative, which doesn't consider the allocator.
var nilAllocator = (*Allocator)(nil)

const (
	// go elemental
	_allocBase    = 24 // defensive... XXX
	_allocPointer = 8
	// gno types
	_allocSlice            = 24
	_allocPointerValue     = 40
	_allocStructValue      = 152
	_allocArrayValue       = 176
	_allocSliceValue       = 40
	_allocFuncValue        = 136
	_allocMapValue         = 144
	_allocBoundMethodValue = 176
	_allocBlock            = 464
	_allocNativeValue      = 48
	_allocTypeValue        = 16
	_allocTypedValue       = 40
	_allocBigint           = 200 // XXX
	_allocBigdec           = 200 // XXX
	_allocType             = 200 // XXX
	_allocAny              = 200 // XXX
)

const (
	allocString      = _allocBase
	allocStringByte  = 1
	allocBigint      = _allocBase + _allocPointer + _allocBigint
	allocBigintByte  = 1
	allocBigdec      = _allocBase + _allocPointer + _allocBigdec
	allocBigdecByte  = 1
	allocPointer     = _allocBase
	allocArray       = _allocBase + _allocPointer + _allocArrayValue
	allocArrayItem   = _allocTypedValue
	allocSlice       = _allocBase + _allocPointer + _allocSliceValue
	allocStruct      = _allocBase + _allocPointer + _allocStructValue
	allocStructField = _allocTypedValue
	allocFunc        = _allocBase + _allocPointer + _allocFuncValue
	allocMap         = _allocBase + _allocPointer + _allocMapValue
	allocMapItem     = _allocTypedValue * 3 // XXX
	allocBoundMethod = _allocBase + _allocPointer + _allocBoundMethodValue
	allocBlock       = _allocBase + _allocPointer + _allocBlock
	allocBlockItem   = _allocTypedValue
	allocNative      = _allocBase + _allocPointer + _allocNativeValue
	allocType        = _allocBase + _allocPointer + _allocType
	// allocDataByte    = 1
	// allocPackge = 1
	allocAmino     = _allocBase + _allocPointer + _allocAny
	allocAminoByte = 10 // XXX
	allocHeapItem  = _allocBase + _allocPointer + _allocTypedValue
)

func NewAllocator(maxBytes int64, heap *Heap) *Allocator {
	if maxBytes == 0 {
		return nil
	}
	return &Allocator{
		maxBytes: maxBytes,
		heap:     heap,
	}
}

func (alloc *Allocator) Status() (maxBytes int64, bytes int64) {
	return alloc.maxBytes, alloc.bytes
}

func (alloc *Allocator) Reset() *Allocator {
	if alloc == nil {
		return nil
	}
	alloc.bytes = 0
	return alloc
}

func (alloc *Allocator) Fork() *Allocator {
	if alloc == nil {
		return nil
	}
	return &Allocator{
		maxBytes: alloc.maxBytes,
		bytes:    alloc.bytes,
	}
}

func (alloc *Allocator) Allocate(size int64) {
	if alloc == nil {
		// this can happen for map items just prior to assignment.
		return
	}

	fmt.Printf("BEFORE ALLOCATE: alloc.bytes: %+v\n", alloc.bytes)
	alloc.bytes += size
	fmt.Printf("AFTER ALLOCATE: alloc.bytes: %+v\n", alloc.bytes)
	if alloc.bytes > alloc.maxBytes {
		if alloc.heap != nil {
			deleted := alloc.heap.MarkAndSweep()
			alloc.DeallocDeleted(deleted)
			if alloc.bytes > alloc.maxBytes {
				panic("allocation limit exceeded")
			}
		}
	}
}

func (alloc *Allocator) DeallocDeleted(objs []*GcObj) {
	for _, obj := range objs {
		alloc.DeallocObj(obj.tv)
	}
}

func (alloc *Allocator) DeallocObj(tv TypedValue) {
	switch v := tv.V.(type) {
	case PointerValue:
		alloc.DeallocObj(*v.TV)
	case *StructValue:
		alloc.DeallocateStruct()
		alloc.DeallocateStructFields(int64(len(v.Fields)))

		for _, field := range v.Fields {
			alloc.DeallocObj(field)
		}
	case *SliceValue:
		alloc.DeallocateSlice()
	case *ArrayValue:
		alloc.DeallocateDataArray(int64(len(v.Data)))
	default:
		fmt.Printf("DeallocDeleted: unimplemented %T\n", v)
	}
}

func (alloc *Allocator) Deallocate(size int64) {
	if alloc == nil {
		// this can happen for map items just prior to assignment.
		return
	}

	alloc.bytes -= size
	fmt.Printf("DEALLOCATE: alloc.bytes: %+v\n", alloc.bytes)
	if alloc.bytes < 0 {
		panic("Deallocate called with negative size")
	}
}

func (alloc *Allocator) DeallocateString(size int64) {
	alloc.Deallocate(allocString + allocStringByte*size)
}

func (alloc *Allocator) AllocateString(size int64) {
	alloc.Allocate(allocString + allocStringByte*size)
}

func (alloc *Allocator) AllocatePointer() {
	println("AllocatePointer")
	alloc.Allocate(allocPointer)
}

func (alloc *Allocator) AllocateDataArray(size int64) {
	println("AllocateDataArray")
	alloc.Allocate(allocArray + size)
}

func (alloc *Allocator) DeallocateDataArray(size int64) {
	println("DeallocateDataArray")
	alloc.Deallocate(allocArray + size)
}

func (alloc *Allocator) AllocateListArray(items int64) {
	println("AllocateListArray")
	alloc.Allocate(allocArray + allocArrayItem*items)
}

func (alloc *Allocator) AllocateSlice() {
	println("AllocateSlice")
	alloc.Allocate(allocSlice)
}

func (alloc *Allocator) DeallocateSlice() {
	println("DeallocateSlice")
	alloc.Deallocate(allocSlice)
}

// NOTE: fields must be allocated separately.
func (alloc *Allocator) AllocateStruct() {
	println("AllocateStruct")
	alloc.Allocate(allocStruct)
}

func (alloc *Allocator) DeallocateStruct() {
	println("DeallocateStruct")
	alloc.Deallocate(allocStruct)
}

func (alloc *Allocator) AllocateStructFields(fields int64) {
	alloc.Allocate(allocStructField * fields)
}

func (alloc *Allocator) DeallocateStructFields(fields int64) {
	alloc.Deallocate(allocStructField * fields)
}

func (alloc *Allocator) AllocateFunc() {
	println("AllocateFunc")
	alloc.Allocate(allocFunc)
}

func (alloc *Allocator) AllocateMap(items int64) {
	println("AllocateMap")
	alloc.Allocate(allocMap + allocMapItem*items)
}

func (alloc *Allocator) AllocateMapItem() {
	alloc.Allocate(allocMapItem)
}

func (alloc *Allocator) AllocateBoundMethod() {
	alloc.Allocate(allocBoundMethod)
}

func (alloc *Allocator) AllocateBlock(items int64) {
	println("AllocateBlock")
	alloc.Allocate(allocBlock + allocBlockItem*items)
}

func (alloc *Allocator) DeallocateBlock(items int64) {
	println("DeallocateBlock")
	alloc.Deallocate(allocBlock + allocBlockItem*items)
}

func (alloc *Allocator) AllocateBlockItems(items int64) {
	alloc.Allocate(allocBlockItem * items)
}

// NOTE: does not allocate for the underlying value.
func (alloc *Allocator) AllocateNative() {
	println("AllocateNative")
	alloc.Allocate(allocNative)
}

/* NOTE: Not used, account for with AllocatePointer.
func (alloc *Allocator) AllocateDataByte() {
	alloc.Allocate(allocDataByte)
}
*/

func (alloc *Allocator) AllocateType() {
	alloc.Allocate(allocType)
}

// NOTE: a reasonable max-bounds calculation for simplicity.
func (alloc *Allocator) AllocateAmino(l int64) {
	alloc.Allocate(allocAmino + allocAminoByte*l)
}

func (alloc *Allocator) AllocateHeapItem() {
	println("AllocateHeapItem")
	alloc.Allocate(allocHeapItem)
}

//----------------------------------------
// constructor utilities.

func (alloc *Allocator) NewString(s string) StringValue {
	println("NewString")
	alloc.AllocateString(int64(len(s)))
	return StringValue(s)
}

func (alloc *Allocator) NewListArray(n int) *ArrayValue {
	println("NewListArray")
	alloc.AllocateListArray(int64(n))
	return &ArrayValue{
		List: make([]TypedValue, n),
	}
}

func (alloc *Allocator) NewDataArray(n int) *ArrayValue {
	println("NewDataArray")
	alloc.AllocateDataArray(int64(n))
	return &ArrayValue{
		Data: make([]byte, n),
	}
}

func (alloc *Allocator) NewArrayFromData(data []byte) *ArrayValue {
	println("NewArrayFromData")
	av := alloc.NewDataArray(len(data))
	copy(av.Data, data)
	return av
}

func (alloc *Allocator) NewSlice(base Value, offset, length, maxcap int) *SliceValue {
	println("NewSlice")
	alloc.AllocateSlice()
	return &SliceValue{
		Base:   base,
		Offset: offset,
		Length: length,
		Maxcap: maxcap,
	}
}

// NOTE: also allocates the underlying array from list.
func (alloc *Allocator) NewSliceFromList(list []TypedValue) *SliceValue {
	println("NewSliceFromList")
	alloc.AllocateSlice()
	alloc.AllocateListArray(int64(cap(list)))
	fullList := list[:cap(list)]
	return &SliceValue{
		Base: &ArrayValue{
			List: fullList,
		},
		Offset: 0,
		Length: len(list),
		Maxcap: cap(list),
	}
}

// NOTE: also allocates the underlying array from data.
func (alloc *Allocator) NewSliceFromData(data []byte) *SliceValue {
	println("NewSliceFromData")
	alloc.AllocateSlice()
	alloc.AllocateDataArray(int64(cap(data)))
	fullData := data[:cap(data)]
	return &SliceValue{
		Base: &ArrayValue{
			Data: fullData,
		},
		Offset: 0,
		Length: len(data),
		Maxcap: cap(data),
	}
}

// NOTE: fields must be allocated (e.g. from NewStructFields)
func (alloc *Allocator) NewStruct(fields []TypedValue) *StructValue {
	println("NewStruct")
	alloc.AllocateStruct()
	return &StructValue{
		Fields: fields,
	}
}

func (alloc *Allocator) NewStructFields(fields int) []TypedValue {
	alloc.AllocateStructFields(int64(fields))
	return make([]TypedValue, fields)
}

// NOTE: fields will be allocated.
func (alloc *Allocator) NewStructWithFields(fields ...TypedValue) *StructValue {
	tvs := alloc.NewStructFields(len(fields))
	copy(tvs, fields)
	return alloc.NewStruct(tvs)
}

func (alloc *Allocator) NewMap(size int) *MapValue {
	alloc.AllocateMap(int64(size))
	mv := &MapValue{}
	mv.MakeMap(size)
	return mv
}

func (alloc *Allocator) NewBlock(source BlockNode, parent *Block) *Block {
	println("NewBlock")
	alloc.AllocateBlock(int64(source.GetNumNames()))
	return NewBlock(source, parent)
}

func (alloc *Allocator) NewNative(rv reflect.Value) *NativeValue {
	println("NewNative")
	alloc.AllocateNative()
	return &NativeValue{
		Value: rv,
	}
}

func (alloc *Allocator) NewType(t Type) Type {
	println("NewType")
	alloc.AllocateType()
	return t
}

func (alloc *Allocator) NewHeapItem(tv TypedValue) *HeapItemValue {
	println("NewHeapItem")
	alloc.AllocateHeapItem()

	gcObj := NewObject(tv)
	alloc.heap.AddObject(gcObj)

	return &HeapItemValue{Value: tv}
}
