// this code is from https://github.com/brunocalza/go-bustub
// there is license and copyright notice in licenses/go-bustub dir

//package table
package tuple

import (
	"github.com/ryogrid/SamehadaDB/storage/page"
	"github.com/ryogrid/SamehadaDB/storage/table/schema"
	"github.com/ryogrid/SamehadaDB/types"
)

type Tuple struct {
	rid  *page.RID
	size uint32
	data []byte
}

func NewTuple(rid *page.RID, size uint32, data []byte) *Tuple {
	return &Tuple{rid, size, data}
}

// NewTupleFromSchema creates a new tuple based on input value
func NewTupleFromSchema(values []types.Value, schema *schema.Schema) *Tuple {
	// calculate tuple size considering varchar columns
	tupleSize := schema.Length()
	for _, colIndex := range schema.GetUnlinedColumns() {
		tupleSize += values[colIndex].Size()
	}
	tuple := &Tuple{}
	tuple.size = tupleSize

	// allocate memory
	tuple.data = make([]byte, tupleSize)

	// serialize each attribute base on the input value
	tupleEndOffset := schema.Length()
	for i := uint32(0); i < schema.GetColumnCount(); i++ {
		if schema.GetColumn(i).IsInlined() {
			tuple.Copy((*(schema.GetColumn(i))).GetOffset(), values[i].Serialize())
		} else {
			tuple.Copy((*(schema.GetColumn(i))).GetOffset(), types.UInt32(tupleEndOffset).Serialize())
			tuple.Copy(tupleEndOffset, values[i].Serialize())
			tupleEndOffset += values[i].Size()
		}
	}
	return tuple
}

func (t *Tuple) GetValue(schema *schema.Schema, colIndex uint32) types.Value {
	column := *(schema.GetColumn(colIndex))
	//column := (*((*interfaces.ISchema)(unsafe.Pointer(&(schema.(interfaces.ISchema)))))).GetColumn(colIndex)
	//column := (schema.(interfaces.ISchema)).GetColumn(colIndex)
	offset := column.GetOffset()
	//castedColumn := (*Column)(unsafe.Pointer(&column))
	if !column.IsInlined() {
		offset = uint32(types.NewUInt32FromBytes(t.data[offset : offset+column.FixedLength()]))
	}

	value := types.NewValueFromBytes(t.data[offset:], column.GetType())
	if value == nil {
		panic(value)
	}
	return *value
}

func (t *Tuple) Size() uint32 {
	return t.size
}

func (t *Tuple) Data() []byte {
	return t.data
}

func (t *Tuple) GetRID() *page.RID {
	return t.rid
}

func (t *Tuple) Copy(offset uint32, data []byte) {
	copy(t.data[offset:], data)
}