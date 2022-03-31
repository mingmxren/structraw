package structraw

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"strings"
	"unsafe"
)

var (
	TagFormatErr    = errors.New("TagFormatErr")
	ReadDataErr     = errors.New("ReadDataErr")
	InvalidTypeErr  = errors.New("InvalidTypeErr")
	WriteDataLenErr = errors.New("WriteDataLenErr")
)

// Marshal with struct_raw tag
func Marshal(s interface{}) ([]byte, error) {
	bb := &bytes.Buffer{}
	if err := MarshalToWriter(s, bb); err != nil {
		return nil, err
	}
	return bb.Bytes(), nil
}

func MarshalToWriter(s interface{}, w io.Writer) error {
	value := reflect.ValueOf(s)
	if reflect.ValueOf(s).Kind() == reflect.Ptr {
		value = reflect.Indirect(reflect.ValueOf(s))
	}
	if value.Kind() != reflect.Struct {
		return InvalidTypeErr
	}
	return marshal(value, w)
}

func marshal(value reflect.Value, w io.Writer) error {
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}

		if err := marshalField(field, value.FieldByIndex(field.Index), w); err != nil {
			return err
		}
	}
	return nil
}

type structRawTag struct {
	Endian binary.ByteOrder
}

func (t *structRawTag) parseStructRawTag(tag string) error {
	ls := strings.Split(tag, ",")
	for _, l := range ls {
		if l == "be" {
			if t.Endian != nil {
				return TagFormatErr
			}
			t.Endian = binary.BigEndian
		} else if l == "le" {
			if t.Endian != nil {
				return TagFormatErr
			}
			t.Endian = binary.LittleEndian
		}
	}

	return nil
}

func putUint(ui uint64, endian binary.ByteOrder, bitSize int, w io.Writer) error {
	if bitSize > 1 && endian == nil {
		return TagFormatErr
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
		return err
	}
	if n != bitSize {
		return WriteDataLenErr
	}
	return nil
}

func marshalField(field reflect.StructField, value reflect.Value, w io.Writer) error {
	stag := field.Tag.Get("struct_raw")
	var tag structRawTag
	err := tag.parseStructRawTag(stag)
	if err != nil {
		return err
	}
	switch field.Type.Kind() {
	case reflect.Uint8:
		if err := putUint(value.Uint(), nil, 1, w); err != nil {
			return err
		}
	case reflect.Uint16:
		if err := putUint(value.Uint(), tag.Endian, 2, w); err != nil {
			return err
		}
	case reflect.Uint32:
		if err := putUint(value.Uint(), tag.Endian, 4, w); err != nil {
			return err
		}
	case reflect.Uint64:
		if err := putUint(value.Uint(), tag.Endian, 8, w); err != nil {
			return err
		}
	case reflect.Array, reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeErr
		}
		var n int
		var err error
		if field.Type.Kind() == reflect.Array {
			n, err = w.Write(valueByteArrayToByteSlice(value))
		} else {
			n, err = w.Write(value.Bytes())
		}
		if err != nil {
			return err
		}
		if n != value.Len() {
			return WriteDataLenErr
		}
	default:
		return InvalidTypeErr
	}

	return nil
}

func Unmarshal(data []byte, s interface{}) error {
	bb := bytes.NewBuffer(data)
	return UnmarshalFromReader(bb, s)
}

func UnmarshalFromReader(r io.Reader, s interface{}) error {
	if reflect.ValueOf(s).Kind() != reflect.Ptr {
		return InvalidTypeErr
	}
	value := reflect.Indirect(reflect.ValueOf(s))
	if value.Kind() != reflect.Struct {
		return InvalidTypeErr
	}
	return unmarshal(r, value)
}

func unmarshal(r io.Reader, value reflect.Value) error {
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		err := unmarshalField(field, value.FieldByIndex(field.Index), r)
		if err != nil {
			return err
		}
	}
	return nil
}

func unmarshalField(field reflect.StructField, value reflect.Value, r io.Reader) error {
	stag := field.Tag.Get("struct_raw")
	var tag structRawTag
	err := tag.parseStructRawTag(stag)
	if err != nil {
		return err
	}
	switch field.Type.Kind() {
	case reflect.Uint8:
		if u, err := getUint(tag.Endian, 1, r); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Uint16:
		if u, err := getUint(tag.Endian, 2, r); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Uint32:
		if u, err := getUint(tag.Endian, 4, r); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Uint64:
		if u, err := getUint(tag.Endian, 8, r); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Array, reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeErr
		}
		b := make([]byte, value.Len())
		if n, err := r.Read(b); err != nil {
			return err
		} else if n != value.Len() {
			return ReadDataErr
		}
		if field.Type.Kind() == reflect.Array {
			copy(valueByteArrayToByteSlice(value), b)
		} else {
			value.SetBytes(b)
		}
	default:
		return InvalidTypeErr
	}
	return nil
}

func getUint(endian binary.ByteOrder, bitSize int, r io.Reader) (uint64, error) {
	if bitSize > 1 && endian == nil {
		return 0, TagFormatErr
	}
	b := [8]byte{}
	n, err := r.Read(b[:bitSize])
	if err != nil {
		return 0, err
	}
	if n != bitSize {
		return 0, ReadDataErr
	}
	switch bitSize {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(endian.Uint16(b[:2])), nil
	case 4:
		return uint64(endian.Uint32(b[:4])), nil
	case 8:
		return endian.Uint64(b[:8]), nil

	default:
		panic("invalid bitSize")
	}
}

func StructLen(s interface{}) (int, error) {
	value := reflect.ValueOf(s)
	if reflect.ValueOf(s).Kind() == reflect.Ptr {
		value = reflect.Indirect(reflect.ValueOf(s))
	}
	if value.Kind() != reflect.Struct {
		return 0, InvalidTypeErr
	}
	return structLen(value)
}

func structLen(value reflect.Value) (int, error) {
	fl := 0
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}

		if l, err := fieldLen(field, value.FieldByIndex(field.Index)); err != nil {
			return 0, err
		} else {
			fl += l
		}
	}
	return fl, nil
}

func fieldLen(field reflect.StructField, value reflect.Value) (int, error) {
	switch field.Type.Kind() {
	case reflect.Uint8:
		return 1, nil
	case reflect.Uint16:
		return 2, nil
	case reflect.Uint32:
		return 4, nil
	case reflect.Uint64:
		return 8, nil
	case reflect.Array, reflect.Slice:
		return value.Len(), nil
	default:
		return 0, InvalidTypeErr
	}
}

type slice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func valueByteArrayToByteSlice(value reflect.Value) []byte {
	return *(*[]byte)(unsafe.Pointer(&slice{
		Data: value.Index(0).Addr().UnsafePointer(),
		Len:  value.Len(),
		Cap:  value.Len(),
	}))
}
