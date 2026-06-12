package logging

import (
	"net/http"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(requestIDHeader, id)

		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}
