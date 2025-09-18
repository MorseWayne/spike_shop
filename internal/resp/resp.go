package resp

import (
	"encoding/json"
	"net/http"
	"time"
)

// Code is business error code. 0 means success.
type Code int

const (
	CodeOK            Code = 0
	CodeInternalError Code = 10000
	CodeInvalidParam  Code = 10001
	CodeTimeout       Code = 10002
)

type Response[T any] struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	Data      *T     `json:"data"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

func WriteJSON[T any](w http.ResponseWriter, status int, code Code, message string, data *T, requestID, traceID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response[T]{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: requestID,
		TraceID:   traceID,
		Timestamp: time.Now().Unix(),
	})
}

func OK[T any](w http.ResponseWriter, data *T, requestID, traceID string) {
	WriteJSON(w, http.StatusOK, CodeOK, "OK", data, requestID, traceID)
}

func Error(w http.ResponseWriter, status int, code Code, message, requestID, traceID string) {
	WriteJSON[any](w, status, code, message, nil, requestID, traceID)
}

// Map business code to HTTP status for common cases
func HTTPStatusFromCode(code Code) int {
	switch code {
	case CodeOK:
		return http.StatusOK
	case CodeInvalidParam:
		return http.StatusBadRequest
	case CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
