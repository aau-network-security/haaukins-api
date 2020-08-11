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
			go lm.CreateEnvironment(client, r.URL.Query().Get(requestedChallenges))

			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/"})
			WaitingResponse(w)
			return
		}

		//got the cookie --> Client exists --> Get or Create Env
		next.ServeHTTP(w, r)
	}
}

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
			if client.RequestMade() >= lm.conf.API.MaxRequest {
				ErrorResponse(w) //todo create maxrequest error page
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

		if !cr.isReady {
			//log.Info().Msgf("[NOT READY] Environment [%s] for the client [%s]", chals, client.ID())
			WaitingResponse(w)
			return
		}

		log.Info().Msgf("[READY] Client Request [%s] for the client [%s]", chals, client.ID())

		authC := http.Cookie{Name: "GUAC_AUTH", Value: cr.guacCookie, Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		host := fmt.Sprintf("/guacamole/?%s=%s", requestedChallenges, chals)
		time.Sleep(5 * time.Second)
		http.Redirect(w, r, host, http.StatusFound)
	}
}

func (lm *LearningMaterialAPI) CreateEnvironment(client Client, chals string) {

	log.Info().Msgf("Creating new Environment with challenges [%s] for [%s]", chals, client.ID())

	cr := client.NewClientRequest(chals)

	chalsTag, _ := lm.GetChallengesFromRequest(chals)

	env, err := lm.newEnvironment(chalsTag)
	if err != nil {
		go cr.NewError(err)
		return
	}

	err = env.Assign(client, chals)
	if err != nil {
		go cr.NewError(err)
		log.Error().Msg("Error while assigning the environment to the client")
		env.Close() //todo might cause an error
		return
	}

	//Close the environment
	go func() {
		<-env.GetTimer().C
		env.Close()
		client.RemoveClientRequest(chals)
	}()

}

func (lm *LearningMaterialAPI) listEnvs() http.HandlerFunc {

	envTable :=
		`<table>
			<thead>
				<tr><th>Client</th><th>Request Made</th><th>Challenges</th></tr>
			</thead>
			<tbody>
        `

	return func(w http.ResponseWriter, r *http.Request) {

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
