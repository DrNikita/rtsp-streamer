package middleware

import (
	"fmt"
	"net/http"
)

func VerifyCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//TODO: request to auth service to verify header credentials
		fmt.Println("retdirecting to localhost:8080/video-list")
		// http.Redirect(w, r, "file:///E:/Diploma/ui-auth/login.html", http.StatusMovedPermanently)
		next.ServeHTTP(w, r)
	})
}
