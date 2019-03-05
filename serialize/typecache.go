// Modified from go-ethereum under GNU Lesser General Public License

package serialize

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

var (
	typeCacheMutex sync.RWMutex
	typeCache      = make(map[typekey]*typeinfo)
)

type typeinfo struct {
	serializer
	deserializer
}

// represents struct Tags
type Tags struct {
	// ser:"nil" controls whether empty input results in a nil pointer.
	NilOK bool
	// ser:"-" ignores fields.
	Ignored bool
	// bytesize: number
	ByteSize int
}

type typekey struct {
	reflect.Type
	// the key must include the struct Tags because they
	// might generate a different decoder.
	Tags
}

type deserializer func(*ByteBuffer, reflect.Value, Tags) error

type serializer func(reflect.Value, *[]byte, Tags) error

func cachedTypeInfo(typ reflect.Type, tags Tags) (*typeinfo, error) {
	typeCacheMutex.RLock()
	info := typeCache[typekey{typ, tags}]
	typeCacheMutex.RUnlock()
	if info != nil {
		return info, nil
	}
	// not in the cache, need to generate info for this type.
	typeCacheMutex.Lock()
	defer typeCacheMutex.Unlock()
	return cachedTypeInfo1(typ, tags)
}

func cachedTypeInfo1(typ reflect.Type, tags Tags) (*typeinfo, error) {
	key := typekey{typ, tags}
	info := typeCache[key]
	if info != nil {
		// another goroutine got the write lock first
		return info, nil
	}
	// put a dummy value into the cache before generating.
	// if the generator tries to lookup itself, it will get
	// the dummy value and won't call itself recursively.
	typeCache[key] = new(typeinfo)
	info, err := genTypeInfo(typ, tags)
	if err != nil {
		// remove the dummy value if the generator fails
		delete(typeCache, key)
		return nil, err
	}

	*typeCache[key] = *info
	return typeCache[key], err
}

type field struct {
	index int
	info  *typeinfo
	tags  Tags
}

func structFields(typ reflect.Type) (fields []field, err error) {
	for i := 0; i < typ.NumField(); i++ {
		if f := typ.Field(i); f.PkgPath == "" { // exported
			tags, err := parseStructTag(typ, i)
			if err != nil {
				return nil, err
			}
			if tags.Ignored {
				continue
			}
			info, err := cachedTypeInfo1(f.Type, tags)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field{i, info, tags})
		}
	}

	return fields, nil
}

func parseStructTag(typ reflect.Type, fi int) (Tags, error) {
	f := typ.Field(fi)
	var ts Tags
	ts.ByteSize = 1
	for _, t := range strings.Split(f.Tag.Get("ser"), ",") {
		switch t = strings.TrimSpace(t); t {
		case "":
		case "-":
			ts.Ignored = true
		case "nil": // nil equal to optional in PyQuackChain
			ts.NilOK = true
		default:
			return ts, fmt.Errorf("ser: unknown struct tag %q on %v.%s", t, typ, f.Name)
		}
	}
	// bytesize use to specify the bytesize of a slice,
	// only slice is useful
	if f.Type.Kind() == reflect.Slice {
		for _, t := range strings.Split(f.Tag.Get("bytesize"), ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				num, err := strconv.Atoi(t)
				if err != nil {
					return ts, err
				}

				ts.ByteSize = num
			}
		}
	}

	return ts, nil
}

func genTypeInfo(typ reflect.Type, tags Tags) (info *typeinfo, err error) {
	info = new(typeinfo)
	if info.serializer, err = makeSerializer(typ); err != nil {
		return nil, err
	}
	if info.deserializer, err = makeDeserializer(typ); err != nil {
		return nil, err
	}
	return info, nil
}
