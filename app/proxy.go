package app

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

//Handle the request made to `/guacamole/`, it forwards the request to guacamole instance
func (lm *LearningMaterialAPI) proxyHandler() http.HandlerFunc {

	challengesTag := ""

	return func(w http.ResponseWriter, r *http.Request) {

		rChallenges := r.URL.Query().Get(requestedChallenges)
		// Get the requested challenges from the URL, the value change everytime a new ENV is requested
		if rChallenges != "" {
			challengesTag = rChallenges
		}

		cookie, _ := r.Cookie(sessionCookie)

		clientID, err := GetTokenFromCookie(cookie.Value, lm.conf.API.SignKey)
		if err != nil { //Error getting the client ID from cookie
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         "Internal Error. Please contact Haaukins maintainers",
				Toomanyrequests: false,
			})
			return
		}
		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil { //Error getting Client
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         "Internal Error. Please contact Haaukins maintainers",
				Toomanyrequests: false,
			})
			return
		}

		cc, err := client.GetClientRequest(challengesTag)
		if err != nil {
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         "Internal Error. Please contact Haaukins maintainers",
				Toomanyrequests: false,
			})
			return
		}

		baseURL := fmt.Sprintf("http://localhost:%d/guacamole/", cc.guacPort)
		origin, _ := url.Parse(baseURL)

		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", origin.Host)
			req.URL.Scheme = "http"
			req.URL.Host = origin.Host
		}

		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}
}
