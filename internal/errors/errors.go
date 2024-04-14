package errors

import (
	stdErrors "errors"
	"fmt"
)

type ErrorCode string

const (
	ErrDB              ErrorCode = "some error in storage layer"
	ErrNoDataFound     ErrorCode = "no data found"
	ErrAlreadyExists   ErrorCode = "already exists"
	ErrTagNotFound     ErrorCode = "tag not found"
	ErrFeatureNotFound ErrorCode = "feature not found"

	ErrUnauthorized ErrorCode = "Unauthorized"

	ErrForbidden ErrorCode = "access is forbidden"
)

type domainError struct {
	error
	errorCode ErrorCode
}

func (e domainError) Error() string {
	return fmt.Sprintf("%s: %s", e.error.Error(), e.errorCode)
}

func Unwrap(err error) error {
	var dErr domainError
	if stdErrors.As(err, &dErr) {
		return stdErrors.Unwrap(dErr.error)
	}

	return stdErrors.Unwrap(err)
}

func Code(err error) ErrorCode {
	if err == nil {
		return ""
	}

	var dErr domainError
	if stdErrors.As(err, &dErr) {
		return dErr.errorCode
	}

	return ""
}

func NewDomainError(errorCode ErrorCode, format string, args ...interface{}) error {
	return domainError{
		error:     fmt.Errorf(format, args...),
		errorCode: errorCode,
	}
}

func WrapIntoDomainError(err error, errorCode ErrorCode, msg string) error {
	return domainError{
		error:     fmt.Errorf("%s: [%w]", msg, err),
		errorCode: errorCode,
	}
}
