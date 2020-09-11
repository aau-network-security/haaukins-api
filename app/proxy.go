package app

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

//Handle the request made to `/guacamole/`, it forwards the request to guacamole instance
func (lm *LearningMaterialAPI) proxyHandler() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		baseURL := fmt.Sprintf("http://localhost:%d/guacamole/", lm.guacamole.GetPort())
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

//Handle the request made to `/guaclogin/`, it redirects the request to proxyHandler instance
func (lm *LearningMaterialAPI) guacLogin() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		rChallenges := r.URL.Query().Get(requestedChallenges)
		clientCookie, _ := r.Cookie(sessionCookie)

		clientID, err := GetTokenFromCookie(clientCookie.Value, lm.conf.API.SignKey)
		if err != nil { //Error getting the client ID from cookie
			log.Error().Msgf("Error getting session token: %v", err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetToken,
				Toomanyrequests: false,
			})
			return
		}

		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil { //Error getting Client
			log.Error().Msgf("Proxy Error getting client [%ds: %v", clientID, err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetClient,
				Toomanyrequests: false,
			})
			return
		}

		cr, err := client.GetClientRequest(rChallenges)
		if err != nil {
			log.Error().Msgf("Proxy Error getting client request [%s]: %v", rChallenges, err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetCR,
				Toomanyrequests: false,
			})
			return
		}

		content, err := lm.guacamole.RawLogin(cr.ID(), cr.ID())
		if err != nil {
			log.Error().Msgf("Unable to login guacamole [%s]: %v", cr.ID(), err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetCR,
				Toomanyrequests: false,
			})
			return
		}
		guacLoginCookie := url.QueryEscape(string(content))

		authC := http.Cookie{Name: "GUAC_AUTH", Value: guacLoginCookie, Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		time.Sleep(10 * time.Second) //wait a little bit more in order to boot kali linux
		host := fmt.Sprintf("/guacamole/?%s=%s", requestedChallenges, rChallenges)
		http.Redirect(w, r, host, http.StatusFound)
	}
}
