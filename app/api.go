package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const (
	requestChallenges = "chals"
	sessionCookie     = "haaukins_session"
)

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/api/{chals}", lm.request(lm.getOrCreateClient(lm.getOrCreateChallengeEnv())))
	return m
}

func (lm *LearningMaterialAPI) request(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		chals, err := lm.GetChallengesFromRequest(mux.Vars(r)["chals"])

		//Bad request (challenge tags don't exist)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(badRequestHTMLTemplate))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestChallenges, chals)))
	}
}

func (lm *LearningMaterialAPI) getOrCreateClient(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie(sessionCookie)

		//Error getting the cookie --> Client is new --> Create Env
		if err != nil {

			client := lm.ClientRequestStore.NewClient(r.Host)
			log.Info().Msgf("Create new Client [%s]", client.ID())

			token, err := client.CreateToken(lm.conf.API.SignKey)
			if err != nil {
				ErrorResponse(w)
				return
			}
			go lm.CreateChallengeEnv(client, mux.Vars(r)["chals"])

			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token})
			WaitingResponse(w)
			return
		}

		//got the cookie --> Client exists --> Get or Create Env
		next.ServeHTTP(w, r)
	}
}

func (lm *LearningMaterialAPI) getOrCreateChallengeEnv() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		chals := mux.Vars(r)["chals"]
		cookie, _ := r.Cookie(sessionCookie)
		clientID, err := GetTokenFromCookie(cookie.Value, lm.conf.API.SignKey)
		if err != nil { //Error getting the client ID from cookie
			ErrorResponse(w)
			return
		}
		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil { //Error getting Client
			ErrorResponse(w)
			return
		}
		cc, err := client.GetChallenge(chals)

		//Create a new challenge
		if err != nil {
			if client.RequestMade() >= lm.conf.API.MaxRequest {
				ErrorResponse(w) //todo create maxrequest error page
				return
			}
			go lm.CreateChallengeEnv(client, mux.Vars(r)["chals"])
			return
		}

		//Check for error while creating the environment
		select {
		case err = <-cc.err:
			ErrorResponse(w)
			return
		default:
		}

		if !cc.isReady {
			log.Info().Msgf("[NOT READY] Environment [%s] for the client [%s]", chals, client.ID())
			WaitingResponse(w)
			return
		}

		log.Info().Msgf("[READY] Environment [%s] for the client [%s]", chals, client.ID())

		authC := http.Cookie{Name: "GUAC_AUTH", Value: cc.guacCookie, Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		host := fmt.Sprintf("http://%s:%d/guacamole", lm.conf.Host, cc.guacPort)
		http.Redirect(w, r, host, http.StatusFound)
	}
}

func (lm *LearningMaterialAPI) CreateChallengeEnv(client Client, chals string) {

	log.Info().Msgf("Creating new Environment [%s] for the client [%s]", chals, client.ID())

	cc := client.NewClientChallenge(chals)

	chalsTag, _ := lm.GetChallengesFromRequest(chals)

	env, err := lm.newEnvironment(chalsTag)
	if err != nil {
		go cc.NewError(err)
		return
	}

	err = env.Assign(client, chals)
	if err != nil {
		go cc.NewError(err)
		return
	}

	client.AddRequest()
}
