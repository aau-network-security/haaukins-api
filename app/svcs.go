package app

import (
	"fmt"
	"net/http"
)

type ProxyConnector func(Environment) http.Handler

type Interception interface {
	ValidRequest(r *http.Request) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (inter Interceptors) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		for _, i := range inter {

			if i.ValidRequest(r) {
				i.Intercept(next).ServeHTTP(w, r)
				return
			}
		}
		fmt.Println("FOrrrrrrrrrrrrrr loooop intercept")
		fmt.Println(r.Host + r.URL.String())
		next.ServeHTTP(w, r)
	})
}
