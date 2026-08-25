package entity

type Tool struct {
	Id           string        `json:"id,omitempty"`
	Index        *int          `json:"index,omitempty"` // required in OpenAI streaming tool_calls deltas to group chunks of the same tool call
	Type         string        `json:"type,omitempty"`  // when splicing claude tools stream messages, it is empty
	Function     Function      `json:"function"`
	CacheControl *CacheControl `json:"cache_control,omitempty"` // Anthropic-compatible: cache the tool definition (incl. schema) at prompt cache level
}

type Function struct {
	Description string `json:"description,omitempty"`
	Name        string `json:"name,omitempty"`       // when splicing claude tools stream messages, it is empty
	Parameters  any    `json:"parameters,omitempty"` // request
	Arguments   any    `json:"arguments,omitempty"`  // response
}
