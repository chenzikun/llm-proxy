package utils

import (
	"github.com/zicorn/llm-proxy/internal/objects"
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
