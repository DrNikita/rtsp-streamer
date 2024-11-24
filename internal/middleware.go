package internal

import (
	"fmt"
	"net/http"
)

var i int

func Init() {
	i = 0
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(i)
		i++
	})
}
