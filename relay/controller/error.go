package controller

import (
	"encoding/json"
	"fmt"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/objects"
	"io"
	"net/http"
	"strconv"
)

type GeneralErrorResponse struct {
	Error    objects.Error `json:"error"`
	Message  string        `json:"message"`
	Msg      string        `json:"msg"`
	Err      string        `json:"err"`
	ErrorMsg string        `json:"error_msg"`
	Header   struct {
		Message string `json:"message"`
	} `json:"header"`
	Response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

func (e GeneralErrorResponse) ToMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Msg != "" {
		return e.Msg
	}
	if e.Err != "" {
		return e.Err
	}
	if e.ErrorMsg != "" {
		return e.ErrorMsg
	}
	if e.Header.Message != "" {
		return e.Header.Message
	}
	if e.Response.Error.Message != "" {
		return e.Response.Error.Message
	}
	return ""
}

func RelayErrorHandler(resp *http.Response) (ErrorWithStatusCode *objects.ErrorWithStatusCode) {
	if resp == nil {
		return &objects.ErrorWithStatusCode{
			StatusCode: 500,
			Error: objects.Error{
				Message: "resp is nil",
				Type:    "upstream_error",
				Code:    "bad_response",
			},
		}
	}
	ErrorWithStatusCode = &objects.ErrorWithStatusCode{
		StatusCode: resp.StatusCode,
		Error: objects.Error{
			Message: "",
			Type:    "upstream_error",
			Code:    "bad_response_status_code",
			Param:   strconv.Itoa(resp.StatusCode),
		},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if config.DebugEnabled {
		logger.SysLog(fmt.Sprintf("error happened, status code: %d, response: \n%s", resp.StatusCode, string(responseBody)))
	}
	err = resp.Body.Close()
	if err != nil {
		return
	}
	var errResponse GeneralErrorResponse
	err = json.Unmarshal(responseBody, &errResponse)
	if err != nil {
		return
	}
	if errResponse.Error.Message != "" {
		// OpenAI format error, so we override the default one
		ErrorWithStatusCode.Error = errResponse.Error
	} else {
		ErrorWithStatusCode.Error.Message = errResponse.ToMessage()
	}
	if ErrorWithStatusCode.Error.Message == "" {
		ErrorWithStatusCode.Error.Message = fmt.Sprintf("bad response status code %d", resp.StatusCode)
	}
	return
}
