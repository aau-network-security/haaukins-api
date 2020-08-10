package app

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ProxyConnector func(Environment) http.HandlerFunc

func (lm *LearningMaterialAPI) ProxyHandler() ProxyConnector {

	return func(e Environment) http.HandlerFunc {

		//loginFunc := func(u, p string) (string, error) {
		//	content, err := e.GetGuacamole().RawLogin(u, p)
		//	if err != nil {
		//		return "", err
		//	}
		//
		//	return url.QueryEscape(string(content)), nil
		//}
		//
		//interceptors := Interceptors{
		//	NewGuacTokenLoginEndpoint(lm, loginFunc),
		//}

		baseURL := fmt.Sprintf("http://localhost:%d", e.GetGuacPort())
		origin, _ := url.Parse(baseURL + "/guacamole/")

		fmt.Println(baseURL)
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

		//return interceptors.Intercept(http.HandlerFunc(
		//	func(w http.ResponseWriter, r *http.Request) {
		//		if isWebSocket(r) {
		//			websocketProxy(host, *lm).ServeHTTP(w, r)
		//			return
		//		}
		//		fmt.Println(r.Host + r.URL.String())
		//		proxy.ServeHTTP(w, r)
		//	}))
	}
	//return func(w http.ResponseWriter, r *http.Request) {
	//
	//	cookie, err := r.Cookie(sessionCookie)
	//	if err != nil {
	//		log.Error().Err(SessionErr)
	//		return
	//	}
	//
	//	clientID, err := GetTokenFromCookie(cookie.Value, lm.conf.API.SignKey)
	//	if err != nil { //Error getting the client ID from cookie
	//		log.Error().Msgf("Failed to find Client ID by token")
	//		return
	//	}
	//
	//	client, err := lm.ClientRequestStore.GetClient(clientID)
	//	if err != nil { //Error getting Client
	//		log.Error().Msgf("Failed to find Client by ID")
	//		return
	//	}
	//	//todo to change
	//	cc, _ := client.GetChallenge("ftp")
	//
	//	baseURL := fmt.Sprintf("http://127.0.0.1:%d", cc.guacPort)
	//	origin, _ := url.Parse(baseURL + "/guacamole")
	//	host := fmt.Sprintf("127.0.0.1:%d", cc.guacPort)
	//
	//	proxy := &httputil.ReverseProxy{
	//		Director: func(req *http.Request) {
	//			req.Header.Add("X-Forwarded-Host", req.Host)
	//			req.URL.Scheme = "http"
	//			req.URL.Host = origin.Host
	//
	//		},
	//	}
	//
	//	fmt.Println("before if")
	//	if isWebSocket(r) {
	//		fmt.Println("inside if")
	//		websocketProxy(host, *lm).ServeHTTP(w, r)
	//		return
	//	}
	//	proxy.ServeHTTP(w, r)
	//
	//}
}
