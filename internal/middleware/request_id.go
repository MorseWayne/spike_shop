package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	HeaderRequestID = "X-Request-ID"
)

// RequestID ensures each request has an ID in context and response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(HeaderRequestID)
		if strings.TrimSpace(rid) == "" {
			rid = uuid.New().String()
		}
		w.Header().Set(HeaderRequestID, rid)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), rid)))
	})
}
