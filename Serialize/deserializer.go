package serialize

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

var (
	errNoPointer     = errors.New("deser: interface given to Deserialize must be a pointer")
	errDeserializeIntoNil = errors.New("deser: pointer given to Deserialize must not be nil")
)

func Deserialize(bb *ByteBuffer, val interface{}) error {
	if val == nil {
		return errDeserializeIntoNil
	}

	rval := reflect.ValueOf(val)
	rtyp := rval.Type()
	if rtyp.Kind() != reflect.Ptr {
		return errNoPointer
	}
	if rval.IsNil() {
		return errDeserializeIntoNil
	}

	info, err := cachedTypeInfo(rtyp.Elem(), tags{})
	if err != nil {
		return err
	}

	err = info.deserializer(bb, rval.Elem())
	return err
}

func DeserializeFromBytes(b []byte, val interface{}) error {
	bb := ByteBuffer{b, 0}
	return Deserialize(&bb, val)
}

func makeDeserializer(typ reflect.Type, ts tags) (deserializer, error) {
	kind := typ.Kind()
	switch {
	//check Ptr first and add optional byte output if ts is nilok,
	//then get serializer for typ.Elem() which is not a ptr
	case kind == reflect.Ptr:
		return makePtrDeserialize(typ, ts)
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(serializableInterface):
		return deserializeSerializableInterface, nil
	case typ.AssignableTo(bigInt):
		return deserializeBigIntNoPtr, nil
	case isUint(kind):
		return deserializeUint, nil
	case kind == reflect.Bool:
		return deserializeBool, nil
	case kind == reflect.String:
		return deserializeString, nil
	case kind == reflect.Slice && isByte(typ.Elem()):
		return deserializeByteSlice, nil
	case kind == reflect.Array && isByte(typ.Elem()):
		return deserializeByteArray, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return deserializeList, nil
	case kind == reflect.Struct:
		return deserializeStruct, nil
	default:
		return nil, fmt.Errorf("type %v is not serializable", typ)
	}
}

func deserializeSerializableInterface(bb *ByteBuffer, val reflect.Value) error {
	return val.Addr().Interface().(Serializable).Deserialize(bb)
}

func deserializeUint(bb *ByteBuffer, val reflect.Value) error {
	kind := val.Type().Kind()
	var bytes []byte
	var err error
	switch {
	case kind > reflect.Uint && kind <= reflect.Uintptr:
		bytes, err = bb.getBytes(val.Type().Bits() / 8)
		break
	case kind == reflect.Uint:
		bytes, err = bb.getVarBytes(1)
		break
	default:
		err = fmt.Errorf("deser: invalid Uint type: %s", val.Type().Name())
		break
	}

	if err == nil {
		var ui uint64 = 0
		for i := 0; i < len(bytes); i++ {
			 ui = ui << 8 | uint64(bytes[i])
		}
		val.SetUint(ui)
	}

	return err
}

func deserializeFixSizeBigUint(bb *ByteBuffer, val *big.Int, size int) error {
	bytes, err := bb.getBytes(size)
	if err == nil {
		val.SetBytes(bytes)
	}

	return err
}

func deserializeBigIntNoPtr(bb *ByteBuffer, val reflect.Value) error {
	return deserializeBigInt(bb, val.Addr())
}

func deserializeBigInt(bb *ByteBuffer, val reflect.Value) error {
	bytes, err := bb.getVarBytes(1)
	if err != nil {
		return err
	}

	i := val.Interface().(*big.Int)
	if i == nil {
		i = new(big.Int)
		val.Set(reflect.ValueOf(i))
	}

	i.SetBytes(bytes)
	return nil
}

func deserializeBool(bb *ByteBuffer, val reflect.Value) error {
	b, err := bb.getBytes(1)
	if err == nil {
		switch b[0] {
		case 0x00:
			val.SetBool(false)
		case 0x01:
			val.SetBool(true)
		default:
			err = fmt.Errorf("deser: invalid boolean value: %d", b[0])
		}
	}

	return err
}

// FixedSizeBytes
func deserializeByteArray(bb *ByteBuffer, val reflect.Value) error {
	if val.Kind() != reflect.Array {
		return fmt.Errorf("deser: invalid byte array type: %s", val.Kind())
	}
	if val.Type().Elem().Kind() != reflect.Uint8 {
		return fmt.Errorf("deser: invalid byte array type: [%d]%s", val.Len(), val.Kind())
	}

	bytes, err := bb.getBytes(val.Len())
	if err == nil{
		reflect.Copy(val, reflect.ValueOf(bytes))
	}

	return err
}

func getByteSize(val reflect.Value) (int, error) {
	var byteSize int = 1
	if reflect.PtrTo(val.Type()).Implements(reflect.TypeOf(new(SerializableList)).Elem()) {
		if !val.CanAddr() {
			return 0, fmt.Errorf("ser: unaddressable value of type %v, Serialize is pointer method", val.Type())
		}

		byteSize = val.Addr().Interface().(SerializableList).GetLenByteSize()
	}

	return byteSize, nil
}
//deserializePrependedSizeBytes
func deserializeByteSlice(bb *ByteBuffer, val reflect.Value) error {
	byteSize, e := getByteSize(val)
	if e != nil {
		return e
	}

	bytes, err := bb.getVarBytes(byteSize)
	if err == nil{
		val.SetBytes(bytes)
	}

	return err
}

func deserializeList(bb *ByteBuffer, val reflect.Value) error {
	typeinfo, err := cachedTypeInfo1(val.Type().Elem(), tags{})
	if err != nil {
		return err
	}

	var vlen int = 0
	if val.Kind() == reflect.Slice {
		byteSize, err := getByteSize(val)
		if err != nil {
			return err
		}

		vlen, err = bb.getLen(byteSize)
		if err != nil {
			return err
		}

		newv := reflect.MakeSlice(val.Type(), vlen, vlen)
		reflect.Copy(newv, val)
		val.Set(newv)
	} else if val.Kind() == reflect.Array{
		vlen = val.Len()
	}

	for i := 0; i < vlen; i++ {
		if err := typeinfo.deserializer(bb, val.Index(i)); err != nil {
			return err
		}
	}

	return nil
}

func deserializeStruct(bb *ByteBuffer, val reflect.Value) error {
	fields, err := structFields(val.Type())
	if err != nil {
		return err
	}

	for _, f := range fields {
		err := f.info.deserializer(bb, val.Field(f.index))
		if err != nil {
			return fmt.Errorf("deser: %s for %v%s", err.Error() , val.Type(), "."+val.Type().Field(f.index).Name)
		}
	}

	return nil
}

func deserializeString(bb *ByteBuffer, val reflect.Value) error {
	b, err := bb.getVarBytes(4)
	if err != nil {
		return err
	}

	val.SetString(string(b))
	return nil
}

func makePtrDeserialize(typ reflect.Type, ts tags) (deserializer, error) {
	t := typ.Elem()
	typeinfo, err := cachedTypeInfo1(t, tags{})
	if err != nil {
		return nil, err
	}

	deser := func(bb *ByteBuffer, val reflect.Value) error {
		if ts.nilOK {
			b, err := bb.getUInt8()
			if err != nil {
				return err
			}

			if b == 0 {
				// set the pointer to nil.
				val.Set(reflect.Zero(typ))
				return nil
			}
		}

		newval := val
		if val.IsNil() {
			newval = reflect.New(t)
		}

		err = typeinfo.deserializer(bb, newval.Elem())
		if err == nil {
			val.Set(newval)
		}

		return err
	}

	return deser, err
}