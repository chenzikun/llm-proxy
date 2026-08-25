package utils

import (
	"github.com/songquanpeng/one-api/objects"
	"net/http"
)

func WrapErr(err error) *objects.ErrorWithStatusCode {
	return &objects.ErrorWithStatusCode{
		StatusCode: http.StatusInternalServerError,
		Error: objects.Error{
			Message: err.Error(),
		},
	}
}
