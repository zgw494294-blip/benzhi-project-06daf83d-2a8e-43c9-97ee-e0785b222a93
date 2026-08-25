package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
)

const maxRequestBody = 1 << 20

type errorEnvelope struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"requestId"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	body := errorBody{Code: "INTERNAL_ERROR", Message: "服务器处理失败"}
	if app, ok := err.(*application.Error); ok {
		status = app.Status
		body = errorBody{Code: app.Code, Message: app.Message, Details: app.Details}
	}
	respond(w, status, errorEnvelope{Error: body, RequestID: requestID(r)})
}
func badJSON(w http.ResponseWriter, r *http.Request, err error) {
	respond(w, 400, errorEnvelope{Error: errorBody{Code: "INVALID_JSON", Message: "请求 JSON 无效：" + err.Error()}, RequestID: requestID(r)})
}
func limit(value, name string, max int) error {
	if len([]rune(strings.TrimSpace(value))) > max {
		return &application.Error{Code: "FIELD_TOO_LONG", Message: name + " 超过长度限制", Status: 400}
	}
	return nil
}
