package structraw

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unsafe"
)

func marshalField(field reflect.StructField, value reflect.Value, w io.Writer) (int, error) {
	marshaller, err := newFieldMarshaler(field)
	if err != nil {
		return 0, err
	}
	return marshaller.marshalFunc(value, w)
}

func unmarshalField(field reflect.StructField, value reflect.Value, r io.Reader) (int, error) {
	unmarshaler, err := newFieldUnmarshaler(field)
	if err != nil {
		return 0, err
	}
	return unmarshaler.unmarshalFunc(value, r)
}

const (
	FieldTagName = "structraw"
)

type fieldTag struct {
	endian binary.ByteOrder
}

func newFieldTag(fieldName string, tag string, uo *uintOptions) (*fieldTag, error) {
	// tag is split by ","
	t := &fieldTag{}
	ls := strings.Split(tag, ",")
	for _, l := range ls {
		switch l {
		case "":
			// pass
		case "be", "le":
			if t.endian != nil {
				return nil, fmt.Errorf("%w: Endian duplicate", ErrTagFormat)
			}
			if l == "be" {
				t.endian = binary.BigEndian
			} else {
				t.endian = binary.LittleEndian
			}
		default:
			return nil, fmt.Errorf("%w: field:%s tag: %s not support", ErrTagFormat, fieldName, l)
		}
	}
	if uo != nil && uo.NeedEndian && t.endian == nil {
		return nil, fmt.Errorf("%w: Endian not found", ErrTagFormat)
	}
	return t, nil
}

type marshalFunc func(v reflect.Value, w io.Writer) (int, error)

type fieldMarshaler struct {
	field       reflect.StructField
	tag         *fieldTag
	marshalFunc marshalFunc
}

type unmarshalFunc func(v reflect.Value, r io.Reader) (int, error)

type fieldUnmarshaler struct {
	field         reflect.StructField
	tag           *fieldTag
	unmarshalFunc unmarshalFunc
}

func parseUintOptions(kind reflect.Kind) *uintOptions {
	switch kind {
	case reflect.Uint8:
		return &uintOptions{NeedEndian: false, BitSize: 1}
	case reflect.Uint16:
		return &uintOptions{NeedEndian: true, BitSize: 2}
	case reflect.Uint32:
		return &uintOptions{NeedEndian: true, BitSize: 4}
	case reflect.Uint64:
		return &uintOptions{NeedEndian: true, BitSize: 8}
	default:
		return nil
	}
}

type uintOptions struct {
	NeedEndian bool
	BitSize    int
}

func newMarshalFunc(field reflect.StructField, uo *uintOptions, tag *fieldTag) (marshalFunc, error) {
	switch field.Type.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(v reflect.Value, w io.Writer) (int, error) {
			return putUint(v.Uint(), tag.endian, uo.BitSize, w)
		}, nil
	case reflect.Array, reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("%w: field:%s kind:%s not support", ErrInvalidType, field.Name, field.Type.Kind())
		}
		return func(v reflect.Value, w io.Writer) (int, error) {
			var n int
			var err error
			if field.Type.Kind() == reflect.Array {
				n, err = w.Write(valueByteArrayToByteSlice(v))
			} else {
				n, err = w.Write(v.Bytes())
			}
			if err != nil {
				return 0, err
			}
			if n != v.Len() {
				return 0, fmt.Errorf("%w:write data len[%d] not equal to data len[%d]", ErrWriteDataLen, n, v.Len())
			}
			return n, nil
		}, nil
	default:
		return nil, fmt.Errorf("%w: field:%s kind:%s not support", ErrInvalidType, field.Name, field.Type.Kind())
	}
}

func newFieldMarshaler(field reflect.StructField) (*fieldMarshaler, error) {
	uo := parseUintOptions(field.Type.Kind())
	tag, err := newFieldTag(field.Name, field.Tag.Get(FieldTagName), uo)
	if err != nil {
		return nil, err
	}
	marshalFunc, err := newMarshalFunc(field, uo, tag)
	if err != nil {
		return nil, err
	}
	marshaler := &fieldMarshaler{
		field:       field,
		tag:         tag,
		marshalFunc: marshalFunc,
	}
	return marshaler, nil
}

func newUnmarshalFunc(field reflect.StructField, uo *uintOptions, tag *fieldTag) (unmarshalFunc, error) {
	switch field.Type.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(v reflect.Value, r io.Reader) (int, error) {
			if u, n, err := getUint(tag.endian, uo.BitSize, r); err != nil {
				return 0, err
			} else {
				v.SetUint(u)
				return n, nil
			}
		}, nil
	case reflect.Array, reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return nil, fmt.Errorf("%w: field:%s kind:%s not support", ErrInvalidType, field.Name, field.Type.Kind())
		}
		return func(v reflect.Value, r io.Reader) (int, error) {
			b := make([]byte, v.Len())
			if n, err := r.Read(b); err != nil {
				return 0, err
			} else if n != v.Len() {
				return 0, ErrReadData
			}
			if field.Type.Kind() == reflect.Array {
				copy(valueByteArrayToByteSlice(v), b)
			} else {
				v.SetBytes(b)
			}
			return v.Len(), nil
		}, nil
	default:
		return nil, fmt.Errorf("%w: field:%s kind:%s not support", ErrInvalidType, field.Name, field.Type.Kind())
	}
}

func newFieldUnmarshaler(field reflect.StructField) (*fieldUnmarshaler, error) {
	uo := parseUintOptions(field.Type.Kind())
	tag, err := newFieldTag(field.Name, field.Tag.Get(FieldTagName), uo)
	if err != nil {
		return nil, err
	}
	unmarshalFunc, err := newUnmarshalFunc(field, uo, tag)
	if err != nil {
		return nil, err
	}
	unmarshaler := &fieldUnmarshaler{
		field:         field,
		tag:           tag,
		unmarshalFunc: unmarshalFunc,
	}
	return unmarshaler, nil
}

func putUint(ui uint64, endian binary.ByteOrder, bitSize int, w io.Writer) (int, error) {
	if bitSize > 1 && endian == nil {
		panic("endianness bitSize is too large")
	}
	b := [8]byte{}
	switch bitSize {
	case 1:
		b[0] = byte(ui)
	case 2:
		endian.PutUint16(b[:2], uint16(ui))
	case 4:
		endian.PutUint32(b[:4], uint32(ui))
	case 8:
		endian.PutUint64(b[:8], ui)
	default:
		panic("invalid bitSize")
	}
	n, err := w.Write(b[:bitSize])
	if err != nil {
		return 0, fmt.Errorf("write data error:%w", err)
	}
	if n != bitSize {
		return 0, fmt.Errorf("%w:write data len[%d] not equal to bit size[%d]", ErrWriteDataLen, n, bitSize)
	}
	return n, nil
}

func getUint(endian binary.ByteOrder, bitSize int, r io.Reader) (uint64, int, error) {
	if bitSize > 1 && endian == nil {
		panic("endianness bitSize is too large")
	}
	b := [8]byte{}
	n, err := r.Read(b[:bitSize])
	if err != nil {
		return 0, 0, fmt.Errorf("read data error:%w", err)
	}
	if n != bitSize {
		return 0, 0, fmt.Errorf("%w:read bit size[%d] not equal to bit size[%d]", ErrReadData, n, bitSize)
	}
	switch bitSize {
	case 1:
		return uint64(b[0]), bitSize, nil
	case 2:
		return uint64(endian.Uint16(b[:2])), bitSize, nil
	case 4:
		return uint64(endian.Uint32(b[:4])), bitSize, nil
	case 8:
		return endian.Uint64(b[:8]), bitSize, nil
	default:
		panic("invalid bitSize")
	}
}

type slice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func valueByteArrayToByteSlice(value reflect.Value) []byte {
	return *(*[]byte)(unsafe.Pointer(&slice{
		Data: unsafe.Pointer(value.Index(0).Addr().Pointer()),
		Len:  value.Len(),
		Cap:  value.Len(),
	}))
}
