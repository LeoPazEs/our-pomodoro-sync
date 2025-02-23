package serve

import (
	"fmt"
	"net/http"
	"strconv"
)

type JsonError interface {
	error
	ErrorObj() error
	Message() string
	Code() int
}

type JsonErrorImpl struct {
	errorObj error
	message  string
	code     int
}

func (jse *JsonErrorImpl) ErrorObj() error {
	return jse.errorObj
}

func (jse *JsonErrorImpl) Message() string {
	return strconv.Quote(jse.message)
}

func (jse *JsonErrorImpl) Code() int {
	return jse.code
}

func (jse *JsonErrorImpl) Error() string {
	return fmt.Sprintf(`{"code": %d,"message": %s}`, jse.code, jse.Message())
}

func NewUnauthorizedError(originError error, message string) JsonError {
	return &JsonErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusUnauthorized,
	}
}

func NewForbiddenError(originError error, message string) JsonError {
	return &JsonErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusForbidden,
	}
}

func NewConflictError(originError error, message string) JsonError {
	return &JsonErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusConflict,
	}
}

func NewBadRequestError(originError error, message string) JsonError {
	return &JsonErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusBadRequest,
	}
}
