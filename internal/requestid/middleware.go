package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const Header = "X-Request-ID"

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		requestID := r.Header.Get(Header)

		if requestID == "" {
			requestID = generate()
		}

		// Keep the ID available to downstream handlers.
		r.Header.Set(Header, requestID)

		// Return the same ID to the client.
		w.Header().Set(Header, requestID)

		next.ServeHTTP(w, r)
	})
}

func generate() string {
	buffer := make([]byte, 16)

	if _, err := rand.Read(buffer); err != nil {
		return "unknown"
	}

	return hex.EncodeToString(buffer)
}
