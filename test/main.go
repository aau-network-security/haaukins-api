package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	//origin, _ := url.Parse("http://localhost:46411")
	//
	//director := func(req *http.Request) {
	//	req.Header.Add("X-Forwarded-Host", req.Host)
	//	req.Header.Add("X-Origin-Host", origin.Host)
	//	req.URL.Scheme = "http"
	//	req.URL.Host = origin.Host
	//}
	//
	//proxy := &httputil.ReverseProxy{Director: director}

	m := http.NewServeMux()

	m.HandleFunc("/guacamole/", guacamole())
	m.HandleFunc("/", origin())

	log.Fatal(http.ListenAndServe(":9001", m))
}

//func main() {
//
//	r := mux.NewRouter()
//
//	r.HandleFunc("/guacamole/", guacamole())
//	r.HandleFunc("/", origin())
//
//	log.Fatal(http.ListenAndServe(":9001", r))
//}
//
func guacamole() http.HandlerFunc {
	origin, _ := url.Parse("http://localhost:46411/guacamole/")

	director := func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = "http"
		req.URL.Host = origin.Host
	}

	proxy := &httputil.ReverseProxy{Director: director}

	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)

	}
}

func origin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/guacamole/", http.StatusFound)
	}
}
