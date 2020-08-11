package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

const (
	requestedChallenges = "challenges"
	sessionCookie       = "haaukins_session"
)

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/api/", lm.request(lm.getOrCreateClient(lm.getOrCreateChallengeEnv())))
	m.HandleFunc("/admin/clients", lm.listClients())
	m.HandleFunc("/admin/envs", lm.listEnvs())
	m.HandleFunc("/guacamole/", lm.proxyHandler())
	return m
}

func (lm *LearningMaterialAPI) request(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// No need to sanitize the url requested
		//https://stackoverflow.com/questions/23285364/does-go-sanitize-urls-for-web-requests

		_, err := lm.GetChallengesFromRequest(r.URL.Query().Get(requestedChallenges))

		//Bad request (challenge tags don't exist, or bad request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(badRequestHTMLTemplate))
			return
		}

		next.ServeHTTP(w, r)
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
			go lm.CreateChallengeEnv(client, r.URL.Query().Get(requestedChallenges))

			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/"})
			WaitingResponse(w)
			return
		}

		//got the cookie --> Client exists --> Get or Create Env
		next.ServeHTTP(w, r)
	}
}

func (lm *LearningMaterialAPI) getOrCreateChallengeEnv() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		chals := r.URL.Query().Get(requestedChallenges)
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
			go lm.CreateChallengeEnv(client, chals)
			WaitingResponse(w)
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
		host := fmt.Sprintf("/guacamole/?%s=%s", requestedChallenges, chals)
		http.Redirect(w, r, host, http.StatusFound)
	}
}

func (lm *LearningMaterialAPI) CreateChallengeEnv(client Client, chals string) {

	envID := GetEnvID(client.ID(), chals)
	log.Info().Msgf("Creating new Environment [%s]", envID)

	cc := client.NewClientChallenge(chals)

	chalsTag, _ := lm.GetChallengesFromRequest(chals)

	env, err := lm.newEnvironment(chalsTag, envID)
	if err != nil {
		go cc.NewError(err)
		return
	}

	err = env.Assign(client, chals)
	if err != nil {
		go cc.NewError(err)
		return
	}

	lm.rcpool.AddRequestChallenge(env)
	client.AddRequest()
}

func (lm *LearningMaterialAPI) listClients() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("list of clients"))
	}
}

func (lm *LearningMaterialAPI) listEnvs() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		envs := lm.rcpool.GetALLRequestsChallenge()
		envsID := make([]string, len(envs))
		var i int
		for _, e := range envs {
			envsID[i] = e.ID()
			i++
		}

		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(strings.Join(envsID, "\n")))
	}
}
