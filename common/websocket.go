package common

import "encoding/base64"

type MessageType int

const (
	Request MessageType = iota
	ResponseData
	ResponseError
	Unknown
)

type ErrorCode int

const (
	Unavailable ErrorCode = iota
	Timeout
	InternalError
)

type WSMessage struct {
	Type string `json:"type"`
}

func (w *WSMessage) MessageType() MessageType {
	switch w.Type {
	case "request":
		return Request
	case "response_data":
		return ResponseData
	case "response_error":
		return ResponseError
	default:
		return Unknown
	}
}

type RequestMessage struct {
	WSMessage
	Data string `json:"data"`
}

func NewRequestMessage(data string) *RequestMessage {
	return &RequestMessage{
		WSMessage: WSMessage{"request"},
		Data:      data,
	}
}

type ResponseDataMessage struct {
	WSMessage
	Response string `json:"response"`
}

func NewResponseDataMessage(data string) *ResponseDataMessage {
	return &ResponseDataMessage{
		WSMessage: WSMessage{"response_data"},
		Response:  data,
	}
}

type ResponseErrorMessage struct {
	WSMessage
	Code ErrorCode `json:"code"`
}

func NewResponseErrorMessage(code ErrorCode) *ResponseErrorMessage {
	return &ResponseErrorMessage{
		WSMessage: WSMessage{"response_error"},
		Code:      code,
	}
}

func (rm *ResponseDataMessage) DecodeResponse() ([]byte, error) {
	return base64.StdEncoding.DecodeString(rm.Response)
}
