package app

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

//"net/http"
//	"net/http/httputil"
//"net/url"

//"github.com/aau-network-security/haaukins/store"

//"github.com/aau-network-security/haaukins/svcs"

//"github.com/rs/zerolog/log"

var (
	SessionErr = errors.New("session must exist")
	wsHeaders  = []string{
		"Sec-Websocket-Extensions",
		"Sec-Websocket-Version",
		"Sec-Websocket-Key",
		"Connection",
		"Upgrade",
	}
	upgrader = websocket.Upgrader{}
)

type guacTokenLoginEndpoint struct {
	lm        *LearningMaterialAPI
	loginFunc func(string, string) (string, error)
}

func NewGuacTokenLoginEndpoint(lm *LearningMaterialAPI, loginFunc func(string, string) (string, error)) *guacTokenLoginEndpoint {
	return &guacTokenLoginEndpoint{
		lm:        lm,
		loginFunc: loginFunc,
	}
}

func (*guacTokenLoginEndpoint) ValidRequest(r *http.Request) bool {

	fmt.Println("ValidRequest")

	if r.URL.Path == "/guaclogin" && r.Method == http.MethodGet {
		return true
	}

	return false
}

func (gtl *guacTokenLoginEndpoint) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(

		func(w http.ResponseWriter, r *http.Request) {

			fmt.Println("Interceptttttttttttttttttt")

			c, err := r.Cookie(sessionCookie)
			if err != nil {
				log.Debug().Msgf("Error session is not found in guacTokenLoginEndpoint, error is %s ", err)
				return
			}

			clientID, err := GetTokenFromCookie(c.Value, gtl.lm.conf.API.SignKey)
			if err != nil { //Error getting the client ID from cookie
				log.Error().Msgf("Failed to find Client ID by token")
				return
			}

			client, err := gtl.lm.ClientRequestStore.GetClient(clientID)
			if err != nil { //Error getting Client
				log.Error().Msgf("Failed to find Client by ID")
				return
			}
			token, err := gtl.loginFunc(client.ID(), client.ID())
			if err != nil {
				log.Warn().
					Err(err).
					Str("client-id", client.ID()).
					Msg("Failed to login team to guacamole")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("unable to connect lab"))
				w.Write([]byte(err.Error()))
				return
			}

			authC := http.Cookie{Name: "GUAC_AUTH", Value: token, Path: "/guacamole/"}
			http.SetCookie(w, &authC)

			http.Redirect(w, r, "/guacamole", http.StatusFound)

		})
}

//func ProxyHandler(lm *LearningMaterialAPI) svcs.ProxyConnector {
//	loginFunc := func(u string, p string) (string, error) {
//		//content, err := env.guacamole.RawLogin(u, p)
//		//if err != nil {
//		//	return "", err
//		//}
//
//		return url.QueryEscape(string("content")), nil
//	}
//
//	return func(efff store.Event) http.Handler {
//		origin, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", 1234) + "/guacamole")
//		host := fmt.Sprintf("127.0.0.1:%d", 1234)
//		interceptors := svcs.Interceptors{
//			NewGuacTokenLoginEndpoint(*lm, loginFunc),
//		}
//
//		proxy := &httputil.ReverseProxy{
//			Director: func(req *http.Request) {
//				req.Header.Add("X-Forwarded-Host", req.Host)
//				req.URL.Scheme = "http"
//				req.URL.Host = origin.Host
//
//			},
//		}
//
//		return interceptors.Intercept(http.HandlerFunc(
//			func(w http.ResponseWriter, r *http.Request) {
//				if isWebSocket(r) {
//					websocketProxy(host, *lm).ServeHTTP(w, r)
//					return
//				}
//				proxy.ServeHTTP(w, r)
//			}))
//	}
//}

func websocketProxy(target string, lm LearningMaterialAPI) http.Handler {
	origin := fmt.Sprintf("http://%s", target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL
		url.Host = target
		url.Scheme = "ws"

		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			log.Error().Err(SessionErr)
			return
		}

		clientID, err := GetTokenFromCookie(cookie.Value, lm.conf.API.SignKey)
		if err != nil { //Error getting the client ID from cookie
			log.Error().Msgf("Failed to find Client ID by token")
			return
		}

		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil { //Error getting Client
			log.Error().Msgf("Failed to find Client by ID")
			return
		}

		rHeader := http.Header{}
		copyHeaders(r.Header, rHeader, wsHeaders)
		rHeader.Set("Origin", origin)
		rHeader.Set("X-Forwarded-Host", r.Host)

		backend, resp, err := websocket.DefaultDialer.Dial(url.String(), rHeader)
		if err != nil {
			log.Error().Msgf("Failed to connect target (%s): %s", url.String(), err)
			return
		}
		defer backend.Close()

		upgradeHeader := http.Header{}
		if h := resp.Header.Get("Sec-Websocket-Protocol"); h != "" {
			upgradeHeader.Set("Sec-Websocket-Protocol", h)
		}

		c, err := upgrader.Upgrade(w, r, upgradeHeader)
		if err != nil {
			log.Error().Msgf("Failed to upgrade connection: %s", err)
			return
		}
		defer c.Close()

		errClient := make(chan error, 1)
		errBackend := make(chan error, 1)

		cp := func(src *websocket.Conn, dst *websocket.Conn, errc chan error) {
			var actions []func([]byte)

			for {
				msgType, data, err := src.ReadMessage()
				if err != nil {
					m := getCloseMsg(err)
					dst.WriteMessage(websocket.CloseMessage, m)
					errc <- err
				}

				if err := dst.WriteMessage(msgType, data); err != nil {
					errc <- err
					break
				}

				for _, action := range actions {
					action(data)
				}
			}

		}

		go cp(backend, c, errClient)

		log.Debug().
			Str("id", client.ID()).
			Msg("team connected")

		var msgFormat string
		select {
		case err = <-errClient:
			msgFormat = "Error when copying from client to backend: %s"
		case err = <-errBackend:
			msgFormat = "Error when copying from backend to client: %s"
		}

		e, ok := err.(*websocket.CloseError)
		if ok && e.Code == websocket.CloseNoStatusReceived {
			log.Debug().
				Str("id", client.ID()).
				Msg("team disconnected")
		} else if !ok || e.Code != websocket.CloseNormalClosure {
			log.Error().Msgf(msgFormat, err)
		}
	})
}

func copyHeaders(src, dst http.Header, ignore []string) {
	for k, vv := range src {
		isIgnored := false
		for _, h := range ignore {
			if k == h {
				isIgnored = true
				break
			}
		}
		if isIgnored {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func getCloseMsg(err error) []byte {
	res := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%s", err))
	if e, ok := err.(*websocket.CloseError); ok {
		if e.Code != websocket.CloseNoStatusReceived {
			res = websocket.FormatCloseMessage(e.Code, e.Text)
		}
	}
	return res
}

func isWebSocket(req *http.Request) bool {
	if upgrade := req.Header.Get("Upgrade"); upgrade != "" {
		return upgrade == "websocket" || upgrade == "Websocket"
	}

	return false
}
