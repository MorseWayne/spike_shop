// Package resp 定义统一的 HTTP 响应模型与业务错误码，并提供便捷写入函数。
package resp

import (
	"encoding/json"
	"net/http"
	"time"
)

// Code 为业务错误码；约定 0 表示成功。
type Code int

const (
	CodeOK            Code = 0
	CodeInternalError Code = 10000
	CodeInvalidParam  Code = 10001
	CodeTimeout       Code = 10002
)

// Response 为统一响应结构，包含业务码、信息、数据载荷与可选链路标识。
type Response[T any] struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	Data      *T     `json:"data"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// WriteJSON 将 Response 写入到 http.ResponseWriter，按入参设置 HTTP 状态码与响应体。
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

// OK 写入一个 code=0 的成功响应，HTTP 状态为 200。
func OK[T any](w http.ResponseWriter, data *T, requestID, traceID string) {
	WriteJSON(w, http.StatusOK, CodeOK, "OK", data, requestID, traceID)
}

// Error 写入一个失败响应，HTTP 状态由调用方决定。
func Error(w http.ResponseWriter, status int, code Code, message, requestID, traceID string) {
	WriteJSON[any](w, status, code, message, nil, requestID, traceID)
}

// HTTPStatusFromCode 提供常见业务码到 HTTP 状态码的映射。
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
