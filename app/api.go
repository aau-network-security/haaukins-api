package app

import (
	"context"
	"net/http"
	"strings"

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
	m.HandleFunc("/admin/clients", lm.listClients())
	m.HandleFunc("/admin/envs", lm.listEnvs())
	lm.m = m
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

		chals := mux.Vars(r)["chals"]
		cookie, _ := r.Cookie(sessionCookie)
		//todo merge the function GetTokenFromCookie lm.ClientRequestStore.GetClient(clientID)
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

		//authC := http.Cookie{Name: "GUAC_AUTH", Value: cc.guacCookie, Path: "/guacamole/"}
		//http.SetCookie(w, &authC)
		//host := fmt.Sprintf("http://%s:%d/guacamole", lm.conf.Host, cc.guacPort)

		env, err := lm.rcpool.GetRequestChallenge(GetEnvID(client.ID(), chals))
		if err != nil {
			//todo manage it
			log.Info().Msg("AAAAAAAAAAAAAAAAAA ERROR getting env")
			WaitingResponse(w)
			return
		}
		lm.m.Handle("/guaclogin", lm.ProxyHandler()(env))
		lm.m.Handle("/guacamole", lm.ProxyHandler()(env))
		lm.m.Handle("/guacamole/", lm.ProxyHandler()(env))

		http.Redirect(w, r, "/guaclogin", http.StatusFound)
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
