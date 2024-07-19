package structraw

import (
	"bytes"
	"io"
	"reflect"
	"sync"
)

// Marshal with struct_raw tag
func Marshal(s interface{}) ([]byte, error) {
	bb := &bytes.Buffer{}
	if _, err := MarshalToWriter(s, bb); err != nil {
		return nil, err
	}
	return bb.Bytes(), nil
}

func MarshalToWriter(s interface{}, w io.Writer) (int, error) {
	value := reflect.ValueOf(s)
	if reflect.ValueOf(s).Kind() == reflect.Ptr {
		value = reflect.Indirect(reflect.ValueOf(s))
	}
	if value.Kind() != reflect.Struct {
		return 0, ErrInvalidType
	}
	return marshal(value, w)
}

type marshaler struct {
	fieldIndex     [][]int
	fieldMarshaler []*fieldMarshaler
}

func (mc *marshaler) marshal(value reflect.Value, w io.Writer) (int, error) {
	n := 0
	for i, index := range mc.fieldIndex {
		if num, err := mc.fieldMarshaler[i].marshalFunc(value.FieldByIndex(index), w); err != nil {
			return 0, err
		} else {
			n += num
		}
	}
	return n, nil
}

func newMarshaler(typ reflect.Type) (*marshaler, error) {
	mc := &marshaler{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		fm, err := newFieldMarshaler(field)
		if err != nil {
			return nil, err
		}
		mc.fieldIndex = append(mc.fieldIndex, field.Index)
		mc.fieldMarshaler = append(mc.fieldMarshaler, fm)
	}
	return mc, nil
}

var marshalCache = NewSafeMap[reflect.Type, *marshaler]()

func getMarshaler(typ reflect.Type) (*marshaler, error) {
	c, ok := marshalCache.Load(typ)
	if !ok {
		m, err := newMarshaler(typ)
		if err != nil {
			return nil, err
		}
		c, _ = marshalCache.LoadOrStore(typ, m)
	}
	return c, nil
}

func marshal(value reflect.Value, w io.Writer) (int, error) {
	m, err := getMarshaler(value.Type())
	if err != nil {
		return 0, err
	}
	return m.marshal(value, w)
}

func Unmarshal(data []byte, s interface{}) error {
	bb := bytes.NewBuffer(data)
	_, err := UnmarshalFromReader(bb, s)
	return err
}

func UnmarshalFromReader(r io.Reader, s interface{}) (int, error) {
	if reflect.ValueOf(s).Kind() != reflect.Ptr {
		return 0, ErrInvalidType
	}
	value := reflect.Indirect(reflect.ValueOf(s))
	if value.Kind() != reflect.Struct {
		return 0, ErrInvalidType
	}
	return unmarshal(r, value)
}

func newUnmarshaler(typ reflect.Type) (*unmarshaler, error) {
	uc := &unmarshaler{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		fu, err := newFieldUnmarshaler(field)
		if err != nil {
			return nil, err
		}
		uc.fieldIndex = append(uc.fieldIndex, field.Index)
		uc.fieldUnmarshaler = append(uc.fieldUnmarshaler, fu)
	}
	return uc, nil
}

type unmarshaler struct {
	fieldIndex       [][]int
	fieldUnmarshaler []*fieldUnmarshaler
}

func (uc *unmarshaler) unmarshal(value reflect.Value, r io.Reader) (int, error) {
	n := 0
	for i, index := range uc.fieldIndex {
		if num, err := uc.fieldUnmarshaler[i].unmarshalFunc(value.FieldByIndex(index), r); err != nil {
			return 0, err
		} else {
			n += num
		}
	}
	return n, nil
}

var unmarshalCache = NewSafeMap[reflect.Type, *unmarshaler]()

func getUnmarshaler(typ reflect.Type) (*unmarshaler, error) {
	c, ok := unmarshalCache.Load(typ)
	if !ok {
		u, err := newUnmarshaler(typ)
		if err != nil {
			return nil, err
		}
		c, _ = unmarshalCache.LoadOrStore(typ, u)
	}
	return c, nil
}

func unmarshal(r io.Reader, value reflect.Value) (int, error) {
	u, err := getUnmarshaler(value.Type())
	if err != nil {
		return 0, err
	}
	return u.unmarshal(value, r)
}

type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

func (sm *SafeMap[K, V]) Load(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	return val, ok
}

func (sm *SafeMap[K, V]) Store(key K, val V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = val
}

func (sm *SafeMap[K, V]) LoadOrStore(key K, val V) (actual V, loaded bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if existing, ok := sm.m[key]; ok {
		return existing, true
	}
	sm.m[key] = val
	return val, false
}
