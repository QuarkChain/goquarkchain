package serialize

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

func Serialize(w *[]byte, val interface{}) error {
	rval := reflect.ValueOf(val)
	ti, err := cachedTypeInfo(rval.Type(), tags{})
	if err != nil {
		return err
	}

	return ti.serializer(rval, w)
}

// SerializeToBytes returns the serialize result of val.
func SerializeToBytes(val interface{}) ([]byte, error) {
	w := make([]byte, 0, 1)
	if err := Serialize(&w, val); err != nil {
		return nil, err
	}

	return w, nil
}

func makeSerializer(typ reflect.Type, ts tags) (serializer, error) {
	kind := typ.Kind()
	switch {
	//check Ptr first and add optional byte output if ts is nilok,
	//then get serializer for typ.Elem() which is not a ptr
	case kind == reflect.Ptr:
		return makePtrSerialize(typ, ts)
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(serializableInterface):
		return serializeSerializableInterface, nil
	case typ.AssignableTo(bigInt):
		return serializeBigIntNoPtr, nil
	case isUint(kind):
		return serializeUint, nil
	case kind == reflect.Bool:
		return serializeBool, nil
	case kind == reflect.String:
		return serializeString, nil
	case kind == reflect.Slice && isByte(typ.Elem()):
		return serializeByteSlice, nil
	case kind == reflect.Array && isByte(typ.Elem()):
		return serializeByteArray, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return serializeList, nil
	case kind == reflect.Struct:
		return serializeStruct, nil
	default:
		return nil, fmt.Errorf("type %v is not serializable", typ)
	}
}

func serializeSerializableInterface(val reflect.Value, w *[]byte) error {
	if !val.CanAddr() {
		return fmt.Errorf("ser: unaddressable value of type %v, Serialize is pointer method", val.Type())
	}

	return val.Addr().Interface().(Serializable).Serialize(w)
}

func prefillByteArray(size int, barray []byte) ([]byte, error) {
	len := len(barray)
	if len > size {
		return nil, errors.New("barray len is larger then expected size")
	}
	if len == size {
		return barray, nil
	}

	bytes := make([]byte, size, size)
	var startIndex = size - len
	copy(bytes[startIndex:], barray)
	return bytes, nil
}

func uint2ByteArray(ui uint64) []byte {
	var bi big.Int
	return bi.SetUint64(ui).Bytes()
}

func serializeUint(val reflect.Value, w *[]byte) error {
	kind := val.Type().Kind()
	var bytes []byte
	var err error
	switch {
	case kind > reflect.Uint && kind <= reflect.Uintptr:
		bytes, err = prefillByteArray(val.Type().Bits()/8, uint2ByteArray(val.Uint()))
		break
	case kind == reflect.Uint:
		//As Uint would be 32/64 bit, so
		var bi big.Int
		serializeBigInt(bi.SetUint64(val.Uint()), w)
		break
	default:
		err = fmt.Errorf("ser: invalid Uint type: %s", val.Type().Name())
		break
	}
	if err == nil {
		*w = append(*w, bytes...)
	}

	return err
}

func serializeFixSizeBigUint(val *big.Int, size int, w *[]byte) error {
	if val == nil {
		bytes := make([]byte, size, size)
		*w = append(*w, bytes...)
		return nil
	}
	bytes, err := prefillByteArray(size, val.Bytes())
	if err == nil {
		*w = append(*w, bytes...)
	}

	return err
}

func serializeBigIntNoPtr(val reflect.Value, w *[]byte) error {
	i := val.Interface().(big.Int)
	return serializeBigInt(&i, w)
}

func serializeBigInt(i *big.Int, w *[]byte) error {
	var bytes []byte
	if cmp := i.Cmp(big.NewInt(0)); cmp == -1 {
		return fmt.Errorf("ser: cannot serialize negative *big.Int")
	} else if cmp == 0 {
		bytes = append(bytes, 0)
	} else {
		bytes = i.Bytes()
	}

	*w = append(*w, uint8(len(bytes)))
	*w = append(*w, bytes...)
	return nil
}

func serializeBool(val reflect.Value, w *[]byte) error {
	if val.Bool() {
		*w = append(*w, 0x01)
	} else {
		*w = append(*w, 0x00)
	}

	return nil
}

func serializeByteArray(val reflect.Value, w *[]byte) error {
	if val.Kind() != reflect.Array {
		return fmt.Errorf("ser: invalid byte array type: %s", val.Kind())
	}
	if val.Type().Elem().Kind() != reflect.Uint8 {
		return fmt.Errorf("ser: invalid byte array type: [%d]%s", val.Len(), val.Kind())
	}

	if !val.CanAddr() {
		// Slice requires the value to be addressable.
		// Make it addressable by copying.
		copy := reflect.New(val.Type()).Elem()
		copy.Set(val)
		val = copy
	}

	size := val.Len()
	slice := val.Slice(0, size).Bytes()

	*w = append(*w, slice...)
	return nil
}

func writeListLen(val reflect.Value, w *[]byte) error {
	var byteSize int = 1
	var err error = nil
	if reflect.PtrTo(val.Type()).Implements(serializableListInterface) {
		// as the type SerializableList is fix number for a struct,
		// we new a value to resolve nil CanAddr is false instead of use
		//
		v := reflect.New(val.Type())
		byteSize = v.Interface().(SerializableList).GetLenByteSize()
	}

	sizeBytes := uint2ByteArray(uint64(val.Len()))
	sizeBytes, err = prefillByteArray(byteSize, sizeBytes)
	if err != nil {
		return nil
	}

	*w = append(*w, sizeBytes...)
	return nil
}

//serializePrependedSizeBytes
func serializeByteSlice(val reflect.Value, w *[]byte) error {
	err := writeListLen(val, w)
	if err != nil {
		return nil
	}

	bytes := val.Bytes()
	*w = append(*w, bytes...)
	return nil
}

//PrependedSizeListSerializer
func serializeList(val reflect.Value, w *[]byte) error {
	typeinfo, err := cachedTypeInfo1(val.Type().Elem(), tags{})
	if err != nil {
		return err
	}

	if val.Kind() == reflect.Slice {
		err = writeListLen(val, w)
		if err != nil {
			return err
		}
	}

	vlen := val.Len()
	for i := 0; i < vlen; i++ {
		if err := typeinfo.serializer(val.Index(i), w); err != nil {
			return err
		}
	}

	return nil
}

func serializeStruct(val reflect.Value, w *[]byte) error {
	fields, err := structFields(val.Type())
	if err != nil {
		return err
	}

	for _, f := range fields {
		if err := f.info.serializer(val.Field(f.index), w); err != nil {
			return err
		}
	}

	return nil
}

func serializeString(val reflect.Value, w *[]byte) error {
	s := val.String()

	sizeBytes := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(val.Len()))

	*w = append(*w, sizeBytes...)
	*w = append(*w, s...)
	return nil
}

//process pointer optional
func makePtrSerialize(typ reflect.Type, ts tags) (serializer, error) {
	typeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}

	ser := func(val reflect.Value, w *[]byte) error {
		switch {
		case val.IsNil() && ts.nilOK:
			*w = append(*w, 0)
			return nil
		case val.IsNil() && typ.Implements(serializableInterface):
			zero := reflect.New(typ.Elem())
			return typeinfo.serializer(zero.Elem(), w)
		case val.IsNil():
			zero := reflect.Zero(typ.Elem())
			return typeinfo.serializer(zero, w)
		default:
			if ts.nilOK {
				*w = append(*w, 1)
			}

			return typeinfo.serializer(val.Elem(), w)
		}
	}

	return ser, err
}
