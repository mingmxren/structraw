package structraw

import (
	"bytes"
	"io"
	"reflect"
	"sync"
)

// Marshal with struct_raw tag
func Marshal(s any) ([]byte, error) {
	bb := &bytes.Buffer{}
	if _, err := MarshalToWriter(s, bb); err != nil {
		return nil, err
	}
	return bb.Bytes(), nil
}

func MarshalToWriter(s any, w io.Writer) (int, error) {
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

var marshalCache sync.Map

func getMarshaler(typ reflect.Type) (*marshaler, error) {
	c, ok := marshalCache.Load(typ)
	if !ok {
		m, err := newMarshaler(typ)
		if err != nil {
			return nil, err
		}
		c, _ = marshalCache.LoadOrStore(typ, m)
	}
	return c.(*marshaler), nil
}

func marshal(value reflect.Value, w io.Writer) (int, error) {
	m, err := getMarshaler(value.Type())
	if err != nil {
		return 0, err
	}
	return m.marshal(value, w)
}

func Unmarshal(data []byte, s any) error {
	bb := bytes.NewBuffer(data)
	_, err := UnmarshalFromReader(bb, s)
	return err
}

func UnmarshalFromReader(r io.Reader, s any) (int, error) {
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

var unmarshalCache sync.Map

func getUnmarshaler(typ reflect.Type) (*unmarshaler, error) {
	c, ok := unmarshalCache.Load(typ)
	if !ok {
		u, err := newUnmarshaler(typ)
		if err != nil {
			return nil, err
		}
		c, _ = unmarshalCache.LoadOrStore(typ, u)
	}
	return c.(*unmarshaler), nil
}

func unmarshal(r io.Reader, value reflect.Value) (int, error) {
	u, err := getUnmarshaler(value.Type())
	if err != nil {
		return 0, err
	}
	return u.unmarshal(value, r)
}
