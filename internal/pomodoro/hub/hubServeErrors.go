package hub

import (
	"fmt"
	"net/http"
)

type HubServeError interface {
	error
	ErrorObj() error
	Message() string
	Code() int
}

type HubServeErrorImpl struct {
	errorObj error
	message  string
	code     int
}

func (hubError *HubServeErrorImpl) ErrorObj() error {
	return hubError.errorObj
}

func (hubError *HubServeErrorImpl) Message() string {
	return hubError.message
}

func (hubError *HubServeErrorImpl) Code() int {
	return hubError.code
}

func (hubError *HubServeErrorImpl) Error() string {
	return fmt.Sprintf(`{"error": "%s"}`, hubError.message)
}

func NewUnauthorizedError(originError error, message string) HubServeError {
	return &HubServeErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusUnauthorized,
	}
}

func NewForbiddenError(originError error, message string) HubServeError {
	return &HubServeErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusForbidden,
	}
}

func NewConflictError(originError error, message string) HubServeError {
	return &HubServeErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusConflict,
	}
}

func NewBadRequestError(originError error, message string) HubServeError {
	return &HubServeErrorImpl{
		errorObj: originError,
		message:  message,
		code:     http.StatusBadRequest,
	}
}
