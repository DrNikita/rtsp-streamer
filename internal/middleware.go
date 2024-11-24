package internal

import (
	"net/http"
)

func verifyCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//TODO: request to auth service to verify header credentials
		next.ServeHTTP(w, r)
	})
}
