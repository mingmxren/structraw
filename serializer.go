package structraw

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
)

var (
	TagErr             = errors.New("struct raw tag format error")
	UnmarshalDataError = errors.New("struct raw unmarshal data error")
)

type InvalidTypeError struct {
	Type reflect.Type
}

func (e InvalidTypeError) Error() string {
	if e.Type == nil {
		return "struct_raw: Marshal/Unmarshal(nil)"
	}

	if e.Type.Kind() != reflect.Ptr {
		return "struct_raw: Marshal/Unmarshal(non-pointer " + e.Type.String() + ")"
	}

	return "struct_raw: Marshal/Unmarshal( " + e.Type.String() + ")"
}

// Marshal with struct_raw tag
func Marshal(s interface{}) ([]byte, error) {
	value := reflect.ValueOf(s)
	if reflect.ValueOf(s).Kind() == reflect.Ptr {
		value = reflect.Indirect(reflect.ValueOf(s))
	}
	if value.Kind() != reflect.Struct {
		return nil, &InvalidTypeError{reflect.TypeOf(s)}
	}
	return marshal(value)
}

func marshal(value reflect.Value) ([]byte, error) {
	bb := &bytes.Buffer{}
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}

		if err := marshalField(field, value.FieldByIndex(field.Index), bb); err != nil {
			return nil, err
		}
	}
	return bb.Bytes(), nil
}

type structRawTag struct {
	Endian binary.ByteOrder
}

func parseStructRawTag(tag string) (*structRawTag, error) {
	t := &structRawTag{}
	ls := strings.Split(tag, ",")
	for _, l := range ls {
		if l == "be" {
			if t.Endian != nil {
				return nil, TagErr
			}
			t.Endian = binary.BigEndian
		} else if l == "le" {
			if t.Endian != nil {
				return nil, TagErr
			}
			t.Endian = binary.LittleEndian
		}
	}

	return t, nil
}

func marshalField(field reflect.StructField, value reflect.Value, bb *bytes.Buffer) error {
	stag := field.Tag.Get("struct_raw")
	tag, err := parseStructRawTag(stag)
	if err != nil {
		return err
	}
	switch field.Type.Kind() {
	case reflect.Uint8:
		bb.WriteByte(byte(value.Uint()))
	case reflect.Uint16:
		if tag.Endian == nil {
			return TagErr
		}
		b := make([]byte, 2)
		tag.Endian.PutUint16(b, uint16(value.Uint()))
		bb.Write(b)
	case reflect.Uint32:
		if tag.Endian == nil {
			return TagErr
		}
		b := make([]byte, 4)
		tag.Endian.PutUint32(b, uint32(value.Uint()))
		bb.Write(b)
	case reflect.Uint64:
		if tag.Endian == nil {
			return TagErr
		}
		b := make([]byte, 8)
		tag.Endian.PutUint64(b, value.Uint())
		bb.Write(b)
	case reflect.Array:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeError{Type: field.Type}
		}
		for i := 0; i < value.Len(); i++ {
			bb.WriteByte(byte(value.Index(i).Uint()))
		}
	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeError{Type: field.Type}
		}
		bb.Write(value.Bytes())
	default:
		return InvalidTypeError{Type: field.Type}
	}

	return nil
}

func Unmarshal(data []byte, s interface{}) error {
	if reflect.ValueOf(s).Kind() != reflect.Ptr {
		return InvalidTypeError{reflect.TypeOf(s)}
	}
	value := reflect.Indirect(reflect.ValueOf(s))
	if value.Kind() != reflect.Struct {
		return InvalidTypeError{reflect.TypeOf(s)}
	}
	return unmarshal(data, value)
}

func unmarshal(data []byte, value reflect.Value) error {
	bb := bytes.NewBuffer(data)
	for i := 0; i < value.Type().NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		err := unmarshalField(field, value.FieldByIndex(field.Index), bb)
		if err != nil {
			return err
		}
	}
	if bb.Len() != 0 {
		return UnmarshalDataError
	}
	return nil
}

func unmarshalField(field reflect.StructField, value reflect.Value, bb *bytes.Buffer) error {
	stag := field.Tag.Get("struct_raw")
	tag, err := parseStructRawTag(stag)
	if err != nil {
		return err
	}
	switch field.Type.Kind() {
	case reflect.Uint8:
		b, err := bb.ReadByte()
		if err != nil {
			return err
		}
		value.SetUint(uint64(b))
	case reflect.Uint16:
		if tag.Endian == nil {
			return TagErr
		}
		if u, err := getUint(tag.Endian, 2, bb); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Uint32:
		if tag.Endian == nil {
			return TagErr
		}
		if u, err := getUint(tag.Endian, 4, bb); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Uint64:
		if tag.Endian == nil {
			return TagErr
		}
		if u, err := getUint(tag.Endian, 8, bb); err != nil {
			return err
		} else {
			value.SetUint(u)
		}
	case reflect.Array:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeError{Type: field.Type}
		}
		b := make([]byte, value.Len())
		if n, err := bb.Read(b); err != nil {
			return err
		} else if n != value.Len() {
			return UnmarshalDataError
		}
		for i := 0; i < value.Len(); i++ {
			value.Index(i).SetUint(uint64(b[i]))
		}
	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.Uint8 {
			return InvalidTypeError{Type: field.Type}
		}
		b := make([]byte, value.Len())
		if n, err := bb.Read(b); err != nil {
			return err
		} else if n != value.Len() {
			return UnmarshalDataError
		}
		value.SetBytes(b)
	default:
		return InvalidTypeError{Type: field.Type}
	}
	return nil
}

func getUint(endian binary.ByteOrder, bitSize int, bb *bytes.Buffer) (uint64, error) {
	if endian == nil {
		return 0, TagErr
	}
	b := make([]byte, bitSize)
	n, err := bb.Read(b)
	if err != nil {
		return 0, err
	}
	if n != bitSize {
		return 0, UnmarshalDataError
	}
	if bitSize == 2 {
		return uint64(endian.Uint16(b)), nil
	} else if bitSize == 4 {
		return uint64(endian.Uint32(b)), nil
	} else if bitSize == 8 {
		return endian.Uint64(b), nil
	} else {
		panic("invalid bitSize")
	}
}
