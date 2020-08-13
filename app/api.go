package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	requestedChallenges = "challenges"
	sessionCookie       = "haaukins_session"
)

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/api/", lm.request(lm.getOrCreateClient(lm.getOrCreateEnvironment())))
	m.HandleFunc("/admin/envs/", lm.listEnvs())
	m.HandleFunc("/guacamole/", lm.proxyHandler())
	return m
}

//Checks if the requested challenges exists and if the API can handle one more request
func (lm *LearningMaterialAPI) request(next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// No need to sanitize the url requested
		//https://stackoverflow.com/questions/23285364/does-go-sanitize-urls-for-web-requests

		_, err := lm.GetChallengesFromRequest(r.URL.Query().Get(requestedChallenges))

		//Bad request (challenge tags don't exist, or bad request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest) //todo make 404 not found page
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(badRequestHTMLTemplate))
			return
		}

		//Check if the API can handle another request
		if len(lm.ClientRequestStore.GetAllRequests()) > lm.conf.API.TotalMaxRequest {
			TooManyRequests(w) //todo create maxrequest error page
			return
		}

		next.ServeHTTP(w, r)
	}
}

//Check if the client already made a request before, if yes goes to the next function,
//if not create a new client and a new environment
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
			go lm.CreateEnvironment(client, r.URL.Query().Get(requestedChallenges))

			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/"})
			WaitingResponse(w)
			return
		}

		//got the cookie --> Client exists --> Get or Create Env
		next.ServeHTTP(w, r)
	}
}

//Check if the requested challenges already exists in an environment,
//if yes just wait the environment is ready, if not create a new environment checking if the
//client can make another request
func (lm *LearningMaterialAPI) getOrCreateEnvironment() http.HandlerFunc {

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
		cr, err := client.GetClientRequest(chals)

		//Create a new Environment
		if err != nil {
			if client.RequestMade() >= lm.conf.API.ClientMaxRequest {
				ClientTooManyRequests(w) //todo create maxrequest error page
				return
			}
			go lm.CreateEnvironment(client, chals)
			WaitingResponse(w)
			return
		}

		//Check for error while creating the environment
		select {
		case err = <-cr.err:
			ErrorResponse(w)
			return
		default:
		}

		//Check if the environment is ready
		if !cr.isReady {
			WaitingResponse(w)
			return
		}

		log.Info().Msgf("[READY] Client Request [%s] for the client [%s]", chals, client.ID())

		authC := http.Cookie{Name: "GUAC_AUTH", Value: cr.guacCookie, Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		host := fmt.Sprintf("/guacamole/?%s=%s", requestedChallenges, chals)
		time.Sleep(5 * time.Second) //wait a little bit more in order to boot kali linux
		http.Redirect(w, r, host, http.StatusFound)
	}
}

//Create a new client request, create a new environment and assign it to the client
//Start a go routine that triggers when the timer expires
func (lm *LearningMaterialAPI) CreateEnvironment(client Client, chals string) {

	log.Info().Msgf("Creating new Environment with challenges [%s] for [%s]", chals, client.ID())

	cr := client.NewClientRequest(chals)

	chalsTag, _ := lm.GetChallengesFromRequest(chals)

	env, err := lm.NewEnvironment(chalsTag)
	if err != nil {
		go cr.NewError(err)
		return
	}

	err = env.Assign(client, chals)
	if err != nil {
		go cr.NewError(err)
		log.Error().Msg("Error while assigning the environment to the client")
		err := env.Close()
		log.Error().Msgf("Error closing the environment through timer: %s", err.Error())
		return
	}

	//Close the environment from the Timer
	go func() {
		<-env.GetTimer().C
		client.RemoveClientRequest(chals)
		err := env.Close()
		log.Error().Msgf("Error closing the environment through timer: %s", err.Error())
	}()

}

//todo might be changed and done in a better way, perhaps add timer as well in a new column
//List the Environments running, it can be called only through admin priviledges
func (lm *LearningMaterialAPI) listEnvs() http.HandlerFunc {

	envTable := `
<table>
	<thead>
		<tr><th>Client</th><th>Request Made</th><th>Challenges</th></tr>
	</thead>
	<tbody>`

	return func(w http.ResponseWriter, r *http.Request) {

		username, password, authOK := r.BasicAuth()

		if authOK == false {
			http.Error(w, "Not authorized", 401) //todo maybe create this page
			return
		}

		if username != lm.conf.API.Admin.Username || password != lm.conf.API.Admin.Password {
			http.Error(w, "Not authorized", 401)
			return
		}

		clients := lm.ClientRequestStore.GetAllClients()
		var envs []Environment
		for _, c := range clients {
			envTable += fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>`, c.Host(), c.RequestMade())
			for _, r := range c.GetAllClientRequests() {
				envs = append(envs, r.env)
				envTable += r.env.GetChallenges() + `<br>`
			}
			envTable += `</td></tr>`
		}

		envTable += `</tbody></table>`

		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(envTable))
	}
}
