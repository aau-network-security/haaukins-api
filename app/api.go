package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/gorilla/mux"
)

const (
	requestChallenges = "chals"
	sessionCookie     = "haaukins_session"
)

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := mux.NewRouter()
	final := http.HandlerFunc(final)
	m.HandleFunc("/api/{chals}", lm.request(lm.getOrCreateClient(lm.getOrCreateChallenge(final))))
	return m
}

func (lm *LearningMaterialAPI) request(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		challengesFromLink := mux.Vars(r)["chals"]
		challenges := strings.Split(challengesFromLink, ",")
		chals, err := lm.GetChallengesFromRequest(challenges)

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

		//Cookie dosent exists so the client is new
		if err != nil {

			client := lm.ClientRequestStore.NewClient(r.Host)
			token, err := client.CreateToken(lm.conf.API.SignKey)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(errorHTMLTemplate))
				return
			}
			//AAAAAAAAAAAA
			err = lm.CreateChallengeEnv(client, mux.Vars(r)["chals"])

			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token})
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(waitingHTMLTemplate))
			return
		}

		//Cookie exists so check if the requested challenge is [ new | loading | exists ]
		next.ServeHTTP(w, r)
	}
}

func (lm *LearningMaterialAPI) getOrCreateChallenge(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		chals := mux.Vars(r)["chals"]
		cookie, _ := r.Cookie(sessionCookie)
		clientID, err := GetTokenFromCookie(cookie.Value, lm.conf.API.SignKey)
		if err != nil {
			fmt.Println(err)
			return
		}
		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil {
			//handle error pls
			fmt.Println(err)
			return
		}
		cc, err := client.GetChallenge(chals)

		// create a new challenge for that client
		if err != nil {
			err = lm.CreateChallengeEnv(client, mux.Vars(r)["chals"])
		}

		fmt.Println(cc.isReady)
		_, _ = w.Write([]byte("aadadad"))

	}
}

func (lm *LearningMaterialAPI) CreateChallengeEnv(client *Client, chals string) error {
	client.m.Lock()
	defer client.m.Unlock()

	log.Info().Msgf("Create new Environment [%s] for the client [%s]", chals, client.username)

	cc := ClientChallenge{
		isReady:    false,
		guacCookie: "",
		guacPort:   "",
	}
	client.challenges[chals] = cc

	//waiting page again and put the function in the chain middleware

	return nil
}

func final(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context().Value(requestChallenges)
	//chals := r.Context().Value(requestChallenges).([]store.Tag)

	fmt.Println(ctx)
	w.Write([]byte("OK"))
}
