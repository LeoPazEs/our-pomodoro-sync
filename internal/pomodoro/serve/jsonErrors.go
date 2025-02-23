package serve

import (
	"fmt"
	"net/http"
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

func (hubError *JsonErrorImpl) ErrorObj() error {
	return hubError.errorObj
}

func (hubError *JsonErrorImpl) Message() string {
	return hubError.message
}

func (hubError *JsonErrorImpl) Code() int {
	return hubError.code
}

func (hubError *JsonErrorImpl) Error() string {
	return fmt.Sprintf(`{"code": %d,"message": "%s"}`, hubError.code, hubError.message)
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
