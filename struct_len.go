package structraw

import (
	"fmt"
	"reflect"
)

func StructLen(s interface{}) (int, error) {
	value := reflect.ValueOf(s)
	if reflect.ValueOf(s).Kind() == reflect.Ptr {
		value = reflect.Indirect(reflect.ValueOf(s))
	}
	if value.Kind() != reflect.Struct {
		return 0, ErrInvalidType
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
		return 0, fmt.Errorf("%w: field:%s type kind:%s not support", ErrInvalidType, field.Name, field.Type.Kind())
	}
}
