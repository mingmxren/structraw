package structraw

import "errors"

var (
	ErrTagFormat    = errors.New("tag format error")
	ErrReadData     = errors.New("read data error")
	ErrInvalidType  = errors.New("invalid type error")
	ErrWriteDataLen = errors.New("write data len error")
)
