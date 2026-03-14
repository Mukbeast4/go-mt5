package protocol

import "encoding/json"

const (
	TypeResponse = "response"
	TypeEvent    = "event"
)

type Request struct {
	ID     string      `json:"id"`
	Action string      `json:"action"`
	Params interface{} `json:"params,omitempty"`
}

type Response struct {
	ID      string           `json:"id,omitempty"`
	Type    string           `json:"type,omitempty"`
	Action  string           `json:"action,omitempty"`
	Success bool             `json:"success"`
	Data    json.RawMessage  `json:"data,omitempty"`
	Error   *ResponseError   `json:"error,omitempty"`
	Event   string           `json:"event,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
