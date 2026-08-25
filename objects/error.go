package objects

type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    any    `json:"code"`
}

type ErrorWithStatusCode struct {
	Error
	StatusCode int `json:"status_code"`
}

func ErrorWrapper(err error, code string, statusCode int) *ErrorWithStatusCode {
	Error := Error{
		Message: err.Error(),
		Type:    "one_api_error",
		Code:    code,
	}
	return &ErrorWithStatusCode{
		Error:      Error,
		StatusCode: statusCode,
	}
}
