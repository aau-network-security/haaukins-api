package app

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	requestedChallenges = "challenges"
	sessionCookie       = "haaukins_session"
	sessionChal         = "chals"
	timeFormat          = "2006-01-02 15:04:05"

	errorChallengesTag = "Challenges Tag not found"
	errorCreateToken   = "Error creating session token"
	errorGetToken      = "Error getting session token"
	errorGetClient     = "Error getting client"
	errorCreateEnv     = "Error creating the environment"
	errorGetCR         = "Error getting the environment"

	errorAPIRequests    = "API reached the maximum number of requests it can handles"
	errorClientRequests = "You reached the maximum number of requests you can make"

	REALM = "Enter password to use secret challenge"
)

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", lm.handleIndex())
	m.HandleFunc("/api/", lm.handleRequest(lm.getOrCreateClient(lm.getOrCreateEnvironment()), lm.conf.SecretChallengeAuth.Username, lm.conf.SecretChallengeAuth.Password))
	m.HandleFunc("/admin/envs/", lm.listEnvs())
	m.HandleFunc("/guaclogin/", lm.guacLogin())
	m.HandleFunc("/guacamole/", lm.proxyHandler())
	m.HandleFunc("/challengesFrontend", lm.handleFrontendChallengesRequest())

	m.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("resources/public"))))

	return m
}

func (lm *LearningMaterialAPI) handleIndex() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		"resources/private/base.tmpl.html",
		"resources/private/index.tmpl.html",
	)
	if err != nil {
		log.Error().Msgf("error index tmpl: %s", err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			notFoundPage(w, r)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.NotFound(w, r)
			log.Error().Msgf("template err index:: %s", err.Error())
		}
	}
}

//Checks if the requested challenges exists and if the API can handle one more request
func (lm *LearningMaterialAPI) handleRequest(next http.Handler, username, password string) http.HandlerFunc {
	var enableBasicAuth bool
	return func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/api/" {
			notFoundPage(w, r)
			return
		}

		// No need to sanitize the url requested
		//https://stackoverflow.com/questions/23285364/does-go-sanitize-urls-for-web-requests

		challenges, err := lm.GetChallengesFromRequest(r.URL.Query().Get(requestedChallenges))

		exercises, err := lm.exStore.GetExercisesByTags(challenges...)
		if err != nil {
			errorPage(w, r, http.StatusBadRequest, returnError{
				Content:         err.Error(),
				Toomanyrequests: false,
			})
			return
		}
		for _, e := range exercises {
			if e.Secret {
				enableBasicAuth = e.Secret
				continue
			}
		}
		//Bad request (challenge tags don't exist, or bad request)
		if err != nil {
			errorPage(w, r, http.StatusBadRequest, returnError{
				Content:         errorChallengesTag,
				Toomanyrequests: false,
			})
			return
		}

		//Check if the API can handle another request
		if len(lm.ClientRequestStore.GetAllRequests()) > lm.conf.API.TotalMaxRequest {
			log.Info().Msg("API reached the maximum number of requests it can handles")
			errorPage(w, r, http.StatusServiceUnavailable, returnError{
				Content:         errorAPIRequests,
				Toomanyrequests: true,
			})
			return
		}
		if enableBasicAuth {
			user, pass, ok := r.BasicAuth()

			if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+REALM+`"`)
				w.WriteHeader(401)
				w.Write([]byte("Unauthorised.\n"))

				return
			}
		}
		if lm.conf.API.Captcha.Enabled {
			_, err = r.Cookie(sessionChal)
			if err != nil {
				isValid := lm.captcha.Verify(r.FormValue("g-recaptcha-response"))
				if !isValid {

					// check if the challenges are secret if so,
					// request a password to be used for the challenge.

					formActionURL := fmt.Sprintf("/api/?%s=%s", requestedChallenges, r.URL.Query().Get(requestedChallenges))

					w.WriteHeader(http.StatusBadRequest)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write([]byte(getCaptchaPage(formActionURL, lm.conf.API.Captcha.SiteKey)))

					return
				}
				authC := http.Cookie{Name: sessionChal, Value: r.FormValue("g-recaptcha-response"), Path: "/", MaxAge: 200}
				http.SetCookie(w, &authC)
			}
		}

		next.ServeHTTP(w, r)
	}
}

func (lm *LearningMaterialAPI) BasicAuth(handler http.HandlerFunc, username, password, realm string) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		user, pass, ok := r.BasicAuth()

		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			w.WriteHeader(401)
			w.Write([]byte("Unauthorised.\n"))

			return
		}

		handler(w, r)
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
			log.Info().Str("client", client.ID()).Msg("Create new Client")

			token, err := client.CreateToken(lm.conf.API.SignKey)
			if err != nil {
				log.Error().Msgf("Error creating session token: %v", err)
				errorPage(w, r, http.StatusInternalServerError, returnError{
					Content:         errorCreateToken,
					Toomanyrequests: false,
				})
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
			log.Error().Msgf("Error getting session token: %v", err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetToken,
				Toomanyrequests: false,
			})
			return
		}
		client, err := lm.ClientRequestStore.GetClient(clientID)
		if err != nil { //Error getting Client
			log.Error().Msgf("Error getting client [%s]: %v", clientID, err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorGetClient,
				Toomanyrequests: false,
			})
			return
		}
		cr, err := client.GetClientRequest(chals)

		//Create a new Environment
		if err != nil {
			if client.RequestMade() >= lm.conf.API.ClientMaxRequest {
				log.Debug().Msgf("Client [%s] has reached max number of requests", clientID)
				errorPage(w, r, http.StatusTooManyRequests, returnError{
					Content:         errorClientRequests,
					Toomanyrequests: true,
				})
				return
			}
			go lm.CreateEnvironment(client, chals)
			WaitingResponse(w)
			return
		}

		//Check for error while creating the environment
		select {
		case err = <-cr.err:
			log.Error().Msgf("Error while creating the environment: %v", err)
			errorPage(w, r, http.StatusInternalServerError, returnError{
				Content:         errorCreateEnv,
				Toomanyrequests: false,
			})
			fw := csv.NewWriter(lm.storeFile)
			writeToCSVFile(fw, []string{time.Now().Format(timeFormat), clientID, client.Host(), chals, err.Error()})
			return
		default:
		}

		//Check if the environment is ready
		if !cr.isReady {
			WaitingResponse(w)
			return
		}

		log.Info().Msgf("[READY] Client Request [%s] for the client [%s]", chals, client.ID())
		fw := csv.NewWriter(lm.storeFile)
		writeToCSVFile(fw, []string{time.Now().Format(timeFormat), clientID, client.Host(), chals, ""})

		host := fmt.Sprintf("/guaclogin/?%s=%s", requestedChallenges, chals)
		http.Redirect(w, r, host, http.StatusFound)
	}
}

//Create a new client request, create a new environment and assign it to the client
//Start a go routine that triggers when the timer expires
func (lm *LearningMaterialAPI) CreateEnvironment(client Client, chals string) {

	log.Info().Str("chals", chals).Str("client", client.ID()).Msg("Creating new Environment")

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
		if err != nil {
			log.Error().Msgf("Error closing the environment through timer, assign function: %s", err.Error())
		}
		return
	}

	//Close the environment from the Timer
	go func() {
		<-env.GetTimer().C
		client.RemoveClientRequest(chals)
		err := env.Close()
		if err != nil {
			log.Error().Msgf("Error closing the environment through timer: %s", err.Error())
		}
	}()

}

//List the Environments running, it can be called only through admin priviledges
func (lm *LearningMaterialAPI) listEnvs() http.HandlerFunc {

	type ListEnvs struct {
		Client      string
		Host        string
		Environment []string
	}

	return func(w http.ResponseWriter, r *http.Request) {

		username, password, authOK := r.BasicAuth()

		if authOK == false {
			http.Error(w, "Not authorized", 401)
			return
		}

		if username != lm.conf.API.Admin.Username || password != lm.conf.API.Admin.Password {
			http.Error(w, "Not authorized", 401)
			return
		}

		clients := lm.ClientRequestStore.GetAllClients()
		listEnvs := make([]ListEnvs, len(clients))

		for _, c := range clients {
			le := ListEnvs{
				Client:      c.ID(),
				Host:        c.Host(),
				Environment: []string{},
			}
			for _, r := range c.GetAllClientRequests() {
				le.Environment = append(le.Environment, r.env.GetChallenges())
			}
			listEnvs = append(listEnvs, le)
		}

		jsonLE, err := json.Marshal(listEnvs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonLE)
	}
}
